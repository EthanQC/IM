package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/schollz/progressbar/v3"
)

// Config 压测配置
type Config struct {
	Mode         string        // connect-only, messaging
	Target       string        // WebSocket URL
	Conns        int           // 总连接数
	Duration     time.Duration // 压测持续时间
	Ramp         time.Duration // 爬坡时间
	PingInterval time.Duration // 心跳间隔
	MsgRate      int           // 每秒消息数（messaging 模式）
	PayloadSize  int           // 消息体大小
	AuthMode     string        // none, token, user-file
	TokenFile    string        // Token 文件路径
	UserFile     string        // 用户文件路径
	Output       string        // 输出格式：text, json
	Verbose      bool          // 详细输出

	// 高并发优化参数
	HandshakeTimeout  time.Duration // 握手超时
	ReadBufferSize    int           // 读缓冲区大小
	WriteBufferSize   int           // 写缓冲区大小
	MaxConnsPerSecond int           // 每秒最大连接数（限速）
	RetryAttempts     int           // 连接重试次数
	RetryDelay        time.Duration // 重试延迟
	ReadTimeout       time.Duration // 读超时
	WriteTimeout      time.Duration // 写超时
}

// Stats 统计数据
type Stats struct {
	mu sync.RWMutex

	// 连接统计
	TotalAttempts   int64 `json:"total_attempts"`
	SuccessConns    int64 `json:"success_conns"`
	FailedConns     int64 `json:"failed_conns"`
	CurrentConns    int64 `json:"current_conns"`
	Disconnects     int64 `json:"disconnects"`
	ReconnectFailed int64 `json:"reconnect_failed"`

	// 延迟统计（纳秒）
	ConnLatencies []int64 `json:"-"`
	MsgLatencies  []int64 `json:"-"`

	// 消息统计
	MessagesSent     int64 `json:"messages_sent"`
	MessagesReceived int64 `json:"messages_received"`
	MessagesFailed   int64 `json:"messages_failed"`

	// Ping/Pong 统计
	PingsSent     int64 `json:"pings_sent"`
	PongsReceived int64 `json:"pongs_received"`

	// 错误统计
	Errors map[string]int64 `json:"errors"`

	// 时间
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Result 压测结果
type Result struct {
	Config Config `json:"config"`

	// 连接指标
	TotalAttempts int64   `json:"total_attempts"`
	SuccessConns  int64   `json:"success_conns"`
	FailedConns   int64   `json:"failed_conns"`
	SuccessRate   float64 `json:"success_rate_percent"`
	Disconnects   int64   `json:"disconnects"`
	FinalConns    int64   `json:"final_conns"`

	// 连接延迟
	ConnLatency LatencyStats `json:"conn_latency_ms"`

	// 消息延迟（messaging 模式）
	MsgLatency LatencyStats `json:"msg_latency_ms,omitempty"`

	// 消息统计
	MessagesSent     int64 `json:"messages_sent"`
	MessagesReceived int64 `json:"messages_received"`

	// 心跳统计
	PingsSent     int64   `json:"pings_sent"`
	PongsReceived int64   `json:"pongs_received"`
	PongRate      float64 `json:"pong_rate_percent"`

	// 错误
	Errors map[string]int64 `json:"errors"`

	// 时间
	Duration   time.Duration `json:"duration_seconds"`
	ActualTime float64       `json:"actual_time_seconds"`
}

// LatencyStats 延迟统计
type LatencyStats struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	P50    float64 `json:"p50"`
	P90    float64 `json:"p90"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	StdDev float64 `json:"std_dev"`
}

// Conn WebSocket 连接包装
type Conn struct {
	id        int
	conn      *websocket.Conn
	userID    uint64
	connected bool
	mu        sync.Mutex
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== wsbench - WebSocket 压测工具 ===")
	fmt.Printf("模式: %s\n", cfg.Mode)
	fmt.Printf("目标: %s\n", cfg.Target)
	fmt.Printf("连接数: %d\n", cfg.Conns)
	fmt.Printf("持续时间: %s\n", cfg.Duration)
	fmt.Printf("爬坡时间: %s\n", cfg.Ramp)
	fmt.Printf("心跳间隔: %s\n", cfg.PingInterval)
	fmt.Printf("握手超时: %s\n", cfg.HandshakeTimeout)
	fmt.Printf("最大连接速率: %d/秒\n", cfg.MaxConnsPerSecond)
	fmt.Printf("重试次数: %d\n", cfg.RetryAttempts)
	fmt.Println()

	stats := &Stats{
		Errors:    make(map[string]int64),
		StartTime: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n收到中断信号，正在关闭...")
		cancel()
	}()

	// 运行压测
	runBench(ctx, cfg, stats)

	stats.EndTime = time.Now()

	// 生成结果
	result := generateResult(cfg, stats)

	// 输出结果
	switch cfg.Output {
	case "json":
		outputJSON(result)
	case "csv":
		outputCSV(result)
	default:
		outputText(result)
	}
}

func parseFlags() Config {
	cfg := Config{}

	flag.StringVar(&cfg.Mode, "mode", "connect-only", "压测模式: connect-only, messaging")
	flag.StringVar(&cfg.Target, "target", "ws://localhost:8084/ws", "WebSocket URL")
	flag.IntVar(&cfg.Conns, "conns", 1000, "总连接数")
	flag.DurationVar(&cfg.Duration, "duration", 5*time.Minute, "压测持续时间")
	flag.DurationVar(&cfg.Ramp, "ramp", 1*time.Minute, "爬坡时间")
	flag.DurationVar(&cfg.PingInterval, "ping-interval", 30*time.Second, "心跳间隔")
	flag.IntVar(&cfg.MsgRate, "msg-rate", 10, "每连接每分钟消息数（messaging 模式）")
	flag.IntVar(&cfg.PayloadSize, "payload-size", 128, "消息体大小（字节）")
	flag.StringVar(&cfg.AuthMode, "auth-mode", "none", "认证模式: none, token, user-file")
	flag.StringVar(&cfg.TokenFile, "token-file", "", "Token 文件路径")
	flag.StringVar(&cfg.UserFile, "user-file", "", "用户文件路径")
	flag.StringVar(&cfg.Output, "output", "text", "输出格式: text, json, csv")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "详细输出")

	// 高并发优化参数
	flag.DurationVar(&cfg.HandshakeTimeout, "handshake-timeout", 30*time.Second, "WebSocket握手超时")
	flag.IntVar(&cfg.ReadBufferSize, "read-buffer", 8192, "读缓冲区大小")
	flag.IntVar(&cfg.WriteBufferSize, "write-buffer", 8192, "写缓冲区大小")
	flag.IntVar(&cfg.MaxConnsPerSecond, "max-cps", 500, "每秒最大连接数（限速）")
	flag.IntVar(&cfg.RetryAttempts, "retry", 3, "连接重试次数")
	flag.DurationVar(&cfg.RetryDelay, "retry-delay", 1*time.Second, "重试延迟")
	flag.DurationVar(&cfg.ReadTimeout, "read-timeout", 120*time.Second, "读超时")
	flag.DurationVar(&cfg.WriteTimeout, "write-timeout", 10*time.Second, "写超时")

	flag.Parse()

	return cfg
}

func runBench(ctx context.Context, cfg Config, stats *Stats) {
	var wg sync.WaitGroup
	connCh := make(chan *Conn, cfg.Conns)

	// 使用批量连接方式，控制每秒连接数
	connsPerSecond := float64(cfg.Conns) / cfg.Ramp.Seconds()
	if connsPerSecond < 1 {
		connsPerSecond = 1
	}
	// 限制最大连接速率
	if connsPerSecond > float64(cfg.MaxConnsPerSecond) {
		connsPerSecond = float64(cfg.MaxConnsPerSecond)
	}

	fmt.Printf("爬坡速率: %.1f 连接/秒 (限制: %d/秒)\n\n", connsPerSecond, cfg.MaxConnsPerSecond)

	// 进度条
	bar := progressbar.NewOptions(cfg.Conns,
		progressbar.OptionSetDescription("建立连接"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("conn"),
	)

	// 使用信号量控制并发连接数 - 增大容量避免阻塞
	// 信号量容量设为 MaxConnsPerSecond * 30，因为连接可能需要最多 30 秒完成
	semCapacity := cfg.MaxConnsPerSecond * 30
	if semCapacity < 5000 {
		semCapacity = 5000
	}
	sem := make(chan struct{}, semCapacity)

	// 爬坡建立连接 - 使用批量方式
	batchSize := int(connsPerSecond / 10) // 每100ms一批
	if batchSize < 1 {
		batchSize = 1
	}
	batchInterval := time.Duration(float64(time.Second) / (connsPerSecond / float64(batchSize)))

	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	connID := 0
	rampDone := false

	for !rampDone {
		select {
		case <-ctx.Done():
			rampDone = true
		case <-ticker.C:
			// 每次批量创建连接
			for i := 0; i < batchSize && connID < cfg.Conns; i++ {
				id := connID
				connID++

				// 使用非阻塞方式获取信号量，如果满了就直接创建（允许突发）
				select {
				case sem <- struct{}{}:
				default:
					// 信号量满了，直接继续（允许超额并发）
				}

				wg.Add(1)
				go func(id int) {
					defer func() {
						wg.Done()
						// 非阻塞释放信号量
						select {
						case <-sem:
						default:
						}
					}()
					conn := createConnectionWithRetry(ctx, id, cfg, stats)
					if conn != nil {
						select {
						case connCh <- conn:
						case <-ctx.Done():
							conn.conn.Close()
						}
					}
					bar.Add(1)
				}(id)
			}

			if connID >= cfg.Conns {
				rampDone = true
			}
		}
	}

	bar.Finish()
	fmt.Println()

	// 等待所有连接 goroutine 完成
	wg.Wait()

	// 收集已建立的连接
	close(connCh)
	var conns []*Conn
	for c := range connCh {
		conns = append(conns, c)
	}

	fmt.Printf("成功建立 %d 个连接\n", len(conns))

	if len(conns) == 0 {
		fmt.Println("没有成功建立的连接，退出")
		return
	}

	// 等待爬坡完成后的剩余时间
	elapsed := time.Since(stats.StartTime)
	remainingDuration := cfg.Duration - elapsed
	if remainingDuration <= 0 {
		remainingDuration = time.Minute
	}

	fmt.Printf("维持连接 %s...\n\n", remainingDuration)

	// 启动心跳和消息发送
	var connWg sync.WaitGroup
	for _, c := range conns {
		connWg.Add(1)
		go func(c *Conn) {
			defer connWg.Done()
			runConnection(ctx, c, cfg, stats, remainingDuration)
		}(c)
	}

	// 状态报告
	reportTicker := time.NewTicker(10 * time.Second)
	defer reportTicker.Stop()

	done := make(chan struct{})
	go func() {
		connWg.Wait()
		close(done)
	}()

	timeout := time.After(remainingDuration)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("收到中断信号，等待连接关闭...")
			connWg.Wait()
			return
		case <-timeout:
			fmt.Println("压测时间到，关闭连接...")
			for _, c := range conns {
				c.mu.Lock()
				if c.conn != nil {
					c.conn.Close()
				}
				c.mu.Unlock()
			}
			connWg.Wait()
			return
		case <-done:
			return
		case <-reportTicker.C:
			printProgress(stats)
		}
	}
}

func createConnectionWithRetry(ctx context.Context, id int, cfg Config, stats *Stats) *Conn {
	var lastErr error
	for attempt := 0; attempt <= cfg.RetryAttempts; attempt++ {
		if attempt > 0 {
			// 指数退避重试
			delay := cfg.RetryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
		}

		conn := createConnection(ctx, id, cfg, stats, attempt > 0)
		if conn != nil {
			return conn
		}
		lastErr = fmt.Errorf("connection attempt %d failed", attempt+1)
	}

	if cfg.Verbose && lastErr != nil {
		fmt.Printf("连接 %d 最终失败，共尝试 %d 次\n", id, cfg.RetryAttempts+1)
	}
	return nil
}

func createConnection(ctx context.Context, id int, cfg Config, stats *Stats, isRetry bool) *Conn {
	if !isRetry {
		atomic.AddInt64(&stats.TotalAttempts, 1)
	}

	start := time.Now()

	// 构建 URL
	url := fmt.Sprintf("%s?user_id=%d&device_id=bench_%d&platform=bench", cfg.Target, 100000+id, id)

	// 创建 dialer - 优化缓冲区和超时
	dialer := websocket.Dialer{
		HandshakeTimeout:  cfg.HandshakeTimeout,
		ReadBufferSize:    cfg.ReadBufferSize,
		WriteBufferSize:   cfg.WriteBufferSize,
		EnableCompression: false, // 禁用压缩以提高性能
	}

	// 连接
	header := http.Header{}
	ws, resp, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		if !isRetry {
			atomic.AddInt64(&stats.FailedConns, 1)
		}
		stats.mu.Lock()
		errStr := categorizeError(err, resp)
		stats.Errors[errStr]++
		stats.mu.Unlock()

		if cfg.Verbose {
			fmt.Printf("连接 %d 失败: %v\n", id, err)
		}
		return nil
	}

	latency := time.Since(start).Nanoseconds()
	stats.mu.Lock()
	stats.ConnLatencies = append(stats.ConnLatencies, latency)
	stats.mu.Unlock()

	atomic.AddInt64(&stats.SuccessConns, 1)
	atomic.AddInt64(&stats.CurrentConns, 1)

	return &Conn{
		id:        id,
		conn:      ws,
		userID:    uint64(100000 + id),
		connected: true,
	}
}

// categorizeError 分类错误，便于分析
func categorizeError(err error, resp *http.Response) string {
	errStr := err.Error()

	// HTTP 响应错误
	if resp != nil {
		return fmt.Sprintf("http_%d", resp.StatusCode)
	}

	// 常见错误分类
	switch {
	case contains(errStr, "connection refused"):
		return "conn_refused"
	case contains(errStr, "connection reset"):
		return "conn_reset"
	case contains(errStr, "timeout"):
		return "timeout"
	case contains(errStr, "too many open files"):
		return "fd_exhausted"
	case contains(errStr, "no such host"):
		return "dns_error"
	case contains(errStr, "network is unreachable"):
		return "network_unreachable"
	case contains(errStr, "i/o timeout"):
		return "io_timeout"
	case contains(errStr, "EOF"):
		return "eof"
	default:
		if len(errStr) > 30 {
			return errStr[:30]
		}
		return errStr
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func runConnection(ctx context.Context, c *Conn, cfg Config, stats *Stats, duration time.Duration) {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.connected = false
		c.mu.Unlock()
		atomic.AddInt64(&stats.CurrentConns, -1)
	}()

	// 设置 Pong 处理器 - 记录服务端回复的 Pong (WebSocket协议层)
	c.conn.SetPongHandler(func(appData string) error {
		atomic.AddInt64(&stats.PongsReceived, 1)
		// 收到Pong后重置读超时
		c.conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
		return nil
	})

	// 设置读超时，防止连接假死
	c.conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))

	// 读取消息的 goroutine
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()

			if conn == nil {
				return
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					atomic.AddInt64(&stats.Disconnects, 1)
					stats.mu.Lock()
					errStr := "read_" + categorizeError(err, nil)
					stats.Errors[errStr]++
					stats.mu.Unlock()
				}
				return
			}

			// 收到消息后重置读超时
			c.conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))

			atomic.AddInt64(&stats.MessagesReceived, 1)

			// 解析消息类型
			var wsMsg struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(msg, &wsMsg) == nil && wsMsg.Type == "pong" {
				atomic.AddInt64(&stats.PongsReceived, 1)
			}
		}
	}()

	// 心跳 ticker - 使用抖动避免所有连接同时发心跳
	jitter := time.Duration(c.id%1000) * time.Millisecond
	time.Sleep(jitter) // 初始抖动

	pingTicker := time.NewTicker(cfg.PingInterval)
	defer pingTicker.Stop()

	// 消息发送 ticker（messaging 模式）
	var msgTickerCh <-chan time.Time
	var msgTicker *time.Ticker
	if cfg.Mode == "messaging" && cfg.MsgRate > 0 {
		interval := time.Minute / time.Duration(cfg.MsgRate)
		msgTicker = time.NewTicker(interval)
		msgTickerCh = msgTicker.C
		defer msgTicker.Stop()
	}

	timeout := time.After(duration)

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			return
		case <-readDone:
			return
		case <-pingTicker.C:
			sendPing(c, cfg, stats)
		case <-msgTickerCh:
			sendMessage(c, cfg, stats)
		}
	}
}

func sendPing(c *Conn, cfg Config, stats *Stats) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || !c.connected {
		return
	}

	msg := map[string]interface{}{
		"type": "ping",
		"ts":   time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(msg)

	c.conn.SetWriteDeadline(time.Now().Add(cfg.WriteTimeout))
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		stats.mu.Lock()
		stats.Errors["ping_failed"]++
		stats.mu.Unlock()
		return
	}

	atomic.AddInt64(&stats.PingsSent, 1)
}

func sendMessage(c *Conn, cfg Config, stats *Stats) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || !c.connected {
		return
	}

	// 构造测试消息
	payload := make([]byte, cfg.PayloadSize)
	for i := range payload {
		payload[i] = 'x'
	}

	msg := map[string]interface{}{
		"type": "message",
		"id":   fmt.Sprintf("%d-%d", c.id, time.Now().UnixNano()),
		"data": map[string]interface{}{
			"content": string(payload),
		},
		"ts": time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(msg)

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		atomic.AddInt64(&stats.MessagesFailed, 1)
		return
	}

	atomic.AddInt64(&stats.MessagesSent, 1)
}

func printProgress(stats *Stats) {
	current := atomic.LoadInt64(&stats.CurrentConns)
	success := atomic.LoadInt64(&stats.SuccessConns)
	failed := atomic.LoadInt64(&stats.FailedConns)
	disconnects := atomic.LoadInt64(&stats.Disconnects)
	pings := atomic.LoadInt64(&stats.PingsSent)
	pongs := atomic.LoadInt64(&stats.PongsReceived)

	elapsed := time.Since(stats.StartTime)
	fmt.Printf("[%s] 当前连接: %d | 成功: %d | 失败: %d | 断开: %d | Ping/Pong: %d/%d\n",
		elapsed.Round(time.Second), current, success, failed, disconnects, pings, pongs)
}

func generateResult(cfg Config, stats *Stats) Result {
	result := Result{
		Config:           cfg,
		TotalAttempts:    stats.TotalAttempts,
		SuccessConns:     stats.SuccessConns,
		FailedConns:      stats.FailedConns,
		Disconnects:      stats.Disconnects,
		FinalConns:       stats.CurrentConns,
		MessagesSent:     stats.MessagesSent,
		MessagesReceived: stats.MessagesReceived,
		PingsSent:        stats.PingsSent,
		PongsReceived:    stats.PongsReceived,
		Errors:           stats.Errors,
		Duration:         cfg.Duration,
		ActualTime:       stats.EndTime.Sub(stats.StartTime).Seconds(),
	}

	if stats.TotalAttempts > 0 {
		result.SuccessRate = float64(stats.SuccessConns) / float64(stats.TotalAttempts) * 100
	}
	if stats.PingsSent > 0 {
		result.PongRate = float64(stats.PongsReceived) / float64(stats.PingsSent) * 100
	}

	// 计算连接延迟
	result.ConnLatency = calculateLatencyStats(stats.ConnLatencies)

	// 计算消息延迟
	if len(stats.MsgLatencies) > 0 {
		result.MsgLatency = calculateLatencyStats(stats.MsgLatencies)
	}

	return result
}

func calculateLatencyStats(latencies []int64) LatencyStats {
	if len(latencies) == 0 {
		return LatencyStats{}
	}

	// 排序
	sorted := make([]int64, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// 转换为毫秒
	toMs := func(ns int64) float64 { return float64(ns) / 1e6 }

	// 计算统计值
	var sum int64
	for _, v := range sorted {
		sum += v
	}
	avg := float64(sum) / float64(len(sorted))

	// 标准差
	var variance float64
	for _, v := range sorted {
		diff := float64(v) - avg
		variance += diff * diff
	}
	variance /= float64(len(sorted))
	stdDev := math.Sqrt(variance)

	return LatencyStats{
		Min:    toMs(sorted[0]),
		Max:    toMs(sorted[len(sorted)-1]),
		Avg:    toMs(int64(avg)),
		P50:    toMs(sorted[len(sorted)*50/100]),
		P90:    toMs(sorted[len(sorted)*90/100]),
		P95:    toMs(sorted[len(sorted)*95/100]),
		P99:    toMs(sorted[len(sorted)*99/100]),
		StdDev: toMs(int64(stdDev)),
	}
}

func outputJSON(result Result) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON 编码错误: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func outputText(result Result) {
	fmt.Println()
	fmt.Println("==================== 压测结果 ====================")
	fmt.Println()
	fmt.Println("--- 连接统计 ---")
	fmt.Printf("尝试连接数:     %d\n", result.TotalAttempts)
	fmt.Printf("成功连接数:     %d\n", result.SuccessConns)
	fmt.Printf("失败连接数:     %d\n", result.FailedConns)
	fmt.Printf("连接成功率:     %.2f%%\n", result.SuccessRate)
	fmt.Printf("断开连接数:     %d\n", result.Disconnects)
	fmt.Printf("最终连接数:     %d\n", result.FinalConns)
	fmt.Println()

	fmt.Println("--- 连接延迟 (ms) ---")
	fmt.Printf("Min:    %.2f\n", result.ConnLatency.Min)
	fmt.Printf("Max:    %.2f\n", result.ConnLatency.Max)
	fmt.Printf("Avg:    %.2f\n", result.ConnLatency.Avg)
	fmt.Printf("P50:    %.2f\n", result.ConnLatency.P50)
	fmt.Printf("P90:    %.2f\n", result.ConnLatency.P90)
	fmt.Printf("P95:    %.2f\n", result.ConnLatency.P95)
	fmt.Printf("P99:    %.2f\n", result.ConnLatency.P99)
	fmt.Printf("StdDev: %.2f\n", result.ConnLatency.StdDev)
	fmt.Println()

	if result.Config.Mode == "messaging" {
		fmt.Println("--- 消息统计 ---")
		fmt.Printf("发送消息数:     %d\n", result.MessagesSent)
		fmt.Printf("接收消息数:     %d\n", result.MessagesReceived)
		fmt.Println()
	}

	fmt.Println("--- 心跳统计 ---")
	fmt.Printf("发送 Ping 数:   %d\n", result.PingsSent)
	fmt.Printf("接收 Pong 数:   %d\n", result.PongsReceived)
	fmt.Printf("Pong 响应率:    %.2f%%\n", result.PongRate)
	fmt.Println()

	if len(result.Errors) > 0 {
		fmt.Println("--- 错误统计 ---")
		for err, count := range result.Errors {
			fmt.Printf("%s: %d\n", err, count)
		}
		fmt.Println()
	}

	fmt.Printf("--- 运行时间: %.2f 秒 ---\n", result.ActualTime)
	fmt.Println()
	fmt.Println("=================================================")
}

func outputCSV(result Result) {
	// CSV Header
	fmt.Println("metric,value")

	// 基础信息
	fmt.Printf("mode,%s\n", result.Config.Mode)
	fmt.Printf("target,%s\n", result.Config.Target)
	fmt.Printf("target_conns,%d\n", result.Config.Conns)
	fmt.Printf("duration_seconds,%.2f\n", result.ActualTime)

	// 连接统计
	fmt.Printf("total_attempts,%d\n", result.TotalAttempts)
	fmt.Printf("success_conns,%d\n", result.SuccessConns)
	fmt.Printf("failed_conns,%d\n", result.FailedConns)
	fmt.Printf("success_rate_percent,%.2f\n", result.SuccessRate)
	fmt.Printf("disconnects,%d\n", result.Disconnects)
	fmt.Printf("final_conns,%d\n", result.FinalConns)

	// 连接延迟
	fmt.Printf("conn_latency_min_ms,%.2f\n", result.ConnLatency.Min)
	fmt.Printf("conn_latency_max_ms,%.2f\n", result.ConnLatency.Max)
	fmt.Printf("conn_latency_avg_ms,%.2f\n", result.ConnLatency.Avg)
	fmt.Printf("conn_latency_p50_ms,%.2f\n", result.ConnLatency.P50)
	fmt.Printf("conn_latency_p90_ms,%.2f\n", result.ConnLatency.P90)
	fmt.Printf("conn_latency_p95_ms,%.2f\n", result.ConnLatency.P95)
	fmt.Printf("conn_latency_p99_ms,%.2f\n", result.ConnLatency.P99)
	fmt.Printf("conn_latency_stddev_ms,%.2f\n", result.ConnLatency.StdDev)

	// 消息统计
	if result.Config.Mode == "messaging" {
		fmt.Printf("messages_sent,%d\n", result.MessagesSent)
		fmt.Printf("messages_received,%d\n", result.MessagesReceived)
		fmt.Printf("msg_latency_p50_ms,%.2f\n", result.MsgLatency.P50)
		fmt.Printf("msg_latency_p95_ms,%.2f\n", result.MsgLatency.P95)
		fmt.Printf("msg_latency_p99_ms,%.2f\n", result.MsgLatency.P99)
	}

	// 心跳统计
	fmt.Printf("pings_sent,%d\n", result.PingsSent)
	fmt.Printf("pongs_received,%d\n", result.PongsReceived)
	fmt.Printf("pong_rate_percent,%.2f\n", result.PongRate)
}
