#!/bin/bash
# 云服务器压测客户端一键部署脚本
# 在 2C2G 云服务器上运行此脚本

set -e

echo "=== IM 压测客户端部署脚本 ==="
echo ""

# 1. 调整系统参数
echo "[1/4] 调整系统内核参数..."
sudo tee /etc/sysctl.d/99-bench.conf > /dev/null << 'EOF'
# 最大文件描述符
fs.file-max = 2000000
fs.nr_open = 2000000

# 网络优化
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 10
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_max_tw_buckets = 2000000
net.ipv4.tcp_syncookies = 1

# 内存优化
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
EOF

sudo sysctl -p /etc/sysctl.d/99-bench.conf

# 2. 调整文件描述符限制
echo "[2/4] 调整文件描述符限制..."
sudo tee -a /etc/security/limits.conf > /dev/null << 'EOF'
* soft nofile 1000000
* hard nofile 1000000
* soft nproc 1000000
* hard nproc 1000000
root soft nofile 1000000
root hard nofile 1000000
EOF

# 当前会话生效
ulimit -n 1000000 2>/dev/null || true

# 3. 安装 Go（如果没有）
echo "[3/4] 检查 Go 环境..."
if ! command -v go &> /dev/null; then
    echo "安装 Go 1.21..."
    wget -q https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
    rm go1.21.6.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
fi
go version

# 4. 下载并编译 wsbench
echo "[4/4] 编译 wsbench..."
mkdir -p ~/wsbench
cd ~/wsbench

# 创建 go.mod
cat > go.mod << 'EOF'
module wsbench

go 1.21

require (
	github.com/gorilla/websocket v1.5.3
	github.com/schollz/progressbar/v3 v3.14.1
)
EOF

# 创建 main.go（精简版，专注高并发）
cat > main.go << 'GOEOF'
package main

import (
	"context"
	"flag"
	"fmt"
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

type Config struct {
	Target           string
	Conns            int
	Duration         time.Duration
	Ramp             time.Duration
	PingInterval     time.Duration
	MaxConnsPerSec   int
	HandshakeTimeout time.Duration
	ReadTimeout      time.Duration
}

type Stats struct {
	TotalAttempts  int64
	SuccessConns   int64
	FailedConns    int64
	CurrentConns   int64
	Disconnects    int64
	PingsSent      int64
	PongsReceived  int64
	ConnLatencies  []int64
	Errors         map[string]int64
	mu             sync.Mutex
	StartTime      time.Time
}

type Conn struct {
	id        int
	conn      *websocket.Conn
	connected bool
	mu        sync.Mutex
}

func main() {
	cfg := parseFlags()

	fmt.Println("=== wsbench 云端压测客户端 ===")
	fmt.Printf("目标: %s\n", cfg.Target)
	fmt.Printf("连接数: %d\n", cfg.Conns)
	fmt.Printf("爬坡时间: %s\n", cfg.Ramp)
	fmt.Printf("持续时间: %s\n", cfg.Duration)
	fmt.Printf("最大连接速率: %d/秒\n", cfg.MaxConnsPerSec)
	fmt.Println()

	stats := &Stats{
		Errors:    make(map[string]int64),
		StartTime: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n收到中断信号，正在关闭...")
		cancel()
	}()

	runBench(ctx, cfg, stats)
	printResult(cfg, stats)
}

func parseFlags() Config {
	cfg := Config{}
	flag.StringVar(&cfg.Target, "target", "ws://localhost:8084/ws", "WebSocket URL")
	flag.IntVar(&cfg.Conns, "conns", 10000, "总连接数")
	flag.DurationVar(&cfg.Duration, "duration", 5*time.Minute, "压测持续时间")
	flag.DurationVar(&cfg.Ramp, "ramp", 2*time.Minute, "爬坡时间")
	flag.DurationVar(&cfg.PingInterval, "ping-interval", 30*time.Second, "心跳间隔")
	flag.IntVar(&cfg.MaxConnsPerSec, "max-cps", 500, "每秒最大连接数")
	flag.DurationVar(&cfg.HandshakeTimeout, "handshake-timeout", 30*time.Second, "握手超时")
	flag.DurationVar(&cfg.ReadTimeout, "read-timeout", 120*time.Second, "读超时")
	flag.Parse()
	return cfg
}

func runBench(ctx context.Context, cfg Config, stats *Stats) {
	var wg sync.WaitGroup
	connCh := make(chan *Conn, cfg.Conns)

	connsPerSecond := float64(cfg.Conns) / cfg.Ramp.Seconds()
	if connsPerSecond > float64(cfg.MaxConnsPerSec) {
		connsPerSecond = float64(cfg.MaxConnsPerSec)
	}

	fmt.Printf("实际爬坡速率: %.1f 连接/秒\n\n", connsPerSecond)

	bar := progressbar.NewOptions(cfg.Conns,
		progressbar.OptionSetDescription("建立连接"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("conn"),
	)

	// 高并发信号量
	semCapacity := cfg.MaxConnsPerSec * 60
	if semCapacity < 10000 {
		semCapacity = 10000
	}
	sem := make(chan struct{}, semCapacity)

	batchSize := int(connsPerSecond / 10)
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
			for i := 0; i < batchSize && connID < cfg.Conns; i++ {
				id := connID
				connID++

				select {
				case sem <- struct{}{}:
				default:
				}

				wg.Add(1)
				go func(id int) {
					defer func() {
						wg.Done()
						select {
						case <-sem:
						default:
						}
					}()
					conn := createConnection(ctx, id, cfg, stats)
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
	wg.Wait()

	close(connCh)
	var conns []*Conn
	for c := range connCh {
		conns = append(conns, c)
	}

	fmt.Printf("成功建立 %d 个连接\n", len(conns))

	if len(conns) == 0 {
		return
	}

	elapsed := time.Since(stats.StartTime)
	remainingDuration := cfg.Duration - elapsed
	if remainingDuration <= 0 {
		remainingDuration = time.Minute
	}

	fmt.Printf("维持连接 %s...\n\n", remainingDuration)

	var connWg sync.WaitGroup
	for _, c := range conns {
		connWg.Add(1)
		go func(c *Conn) {
			defer connWg.Done()
			runConnection(ctx, c, cfg, stats, remainingDuration)
		}(c)
	}

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
			for _, c := range conns {
				c.mu.Lock()
				if c.conn != nil {
					c.conn.Close()
				}
				c.mu.Unlock()
			}
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
			fmt.Printf("[%s] 当前连接: %d | 成功: %d | 失败: %d | 断开: %d | Ping/Pong: %d/%d\n",
				time.Since(stats.StartTime).Round(time.Second),
				atomic.LoadInt64(&stats.CurrentConns),
				atomic.LoadInt64(&stats.SuccessConns),
				atomic.LoadInt64(&stats.FailedConns),
				atomic.LoadInt64(&stats.Disconnects),
				atomic.LoadInt64(&stats.PingsSent),
				atomic.LoadInt64(&stats.PongsReceived))
		}
	}
}

func createConnection(ctx context.Context, id int, cfg Config, stats *Stats) *Conn {
	atomic.AddInt64(&stats.TotalAttempts, 1)
	start := time.Now()

	url := fmt.Sprintf("%s?user_id=%d&device_id=cloud_%d&platform=bench", cfg.Target, 100000+id, id)

	dialer := websocket.Dialer{
		HandshakeTimeout:  cfg.HandshakeTimeout,
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		EnableCompression: false,
	}

	ws, resp, err := dialer.DialContext(ctx, url, http.Header{})
	if err != nil {
		atomic.AddInt64(&stats.FailedConns, 1)
		stats.mu.Lock()
		errStr := categorizeError(err, resp)
		stats.Errors[errStr]++
		stats.mu.Unlock()
		return nil
	}

	latency := time.Since(start).Nanoseconds()
	stats.mu.Lock()
	stats.ConnLatencies = append(stats.ConnLatencies, latency)
	stats.mu.Unlock()

	atomic.AddInt64(&stats.SuccessConns, 1)
	atomic.AddInt64(&stats.CurrentConns, 1)

	return &Conn{id: id, conn: ws, connected: true}
}

func categorizeError(err error, resp *http.Response) string {
	if resp != nil {
		return fmt.Sprintf("http_%d", resp.StatusCode)
	}
	errStr := err.Error()
	switch {
	case contains(errStr, "connection refused"):
		return "conn_refused"
	case contains(errStr, "connection reset"):
		return "conn_reset"
	case contains(errStr, "timeout"):
		return "timeout"
	case contains(errStr, "too many open files"):
		return "fd_exhausted"
	default:
		if len(errStr) > 30 {
			return errStr[:30]
		}
		return errStr
	}
}

func contains(s, substr string) bool {
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

	c.conn.SetPongHandler(func(appData string) error {
		atomic.AddInt64(&stats.PongsReceived, 1)
		c.conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
		return nil
	})

	c.conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))

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
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					atomic.AddInt64(&stats.Disconnects, 1)
				}
				return
			}
		}
	}()

	pingTicker := time.NewTicker(cfg.PingInterval)
	defer pingTicker.Stop()

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
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()
			if conn == nil {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			atomic.AddInt64(&stats.PingsSent, 1)
		}
	}
}

func printResult(cfg Config, stats *Stats) {
	fmt.Println()
	fmt.Println("==================== 压测结果 ====================")
	fmt.Println()
	fmt.Println("--- 连接统计 ---")
	fmt.Printf("尝试连接数:     %d\n", stats.TotalAttempts)
	fmt.Printf("成功连接数:     %d\n", stats.SuccessConns)
	fmt.Printf("失败连接数:     %d\n", stats.FailedConns)
	if stats.TotalAttempts > 0 {
		fmt.Printf("连接成功率:     %.2f%%\n", float64(stats.SuccessConns)/float64(stats.TotalAttempts)*100)
	}
	fmt.Printf("断开连接数:     %d\n", stats.Disconnects)
	fmt.Printf("最终连接数:     %d\n", stats.CurrentConns)
	fmt.Println()

	if len(stats.ConnLatencies) > 0 {
		fmt.Println("--- 连接延迟 (ms) ---")
		sort.Slice(stats.ConnLatencies, func(i, j int) bool {
			return stats.ConnLatencies[i] < stats.ConnLatencies[j]
		})
		n := len(stats.ConnLatencies)
		var sum int64
		for _, v := range stats.ConnLatencies {
			sum += v
		}
		fmt.Printf("Min:    %.2f\n", float64(stats.ConnLatencies[0])/1e6)
		fmt.Printf("Max:    %.2f\n", float64(stats.ConnLatencies[n-1])/1e6)
		fmt.Printf("Avg:    %.2f\n", float64(sum)/float64(n)/1e6)
		fmt.Printf("P50:    %.2f\n", float64(stats.ConnLatencies[n*50/100])/1e6)
		fmt.Printf("P90:    %.2f\n", float64(stats.ConnLatencies[n*90/100])/1e6)
		fmt.Printf("P95:    %.2f\n", float64(stats.ConnLatencies[n*95/100])/1e6)
		fmt.Printf("P99:    %.2f\n", float64(stats.ConnLatencies[n*99/100])/1e6)
		fmt.Println()
	}

	fmt.Println("--- 心跳统计 ---")
	fmt.Printf("发送 Ping 数:   %d\n", stats.PingsSent)
	fmt.Printf("接收 Pong 数:   %d\n", stats.PongsReceived)
	if stats.PingsSent > 0 {
		fmt.Printf("Pong 响应率:    %.2f%%\n", float64(stats.PongsReceived)/float64(stats.PingsSent)*100)
	}

	if len(stats.Errors) > 0 {
		fmt.Println()
		fmt.Println("--- 错误统计 ---")
		for k, v := range stats.Errors {
			fmt.Printf("%s: %d\n", k, v)
		}
	}

	fmt.Println()
	fmt.Printf("--- 运行时间: %.2f 秒 ---\n", time.Since(stats.StartTime).Seconds())
	fmt.Println()
	fmt.Println("=================================================")
}
GOEOF

go mod tidy
go build -o wsbench .

echo ""
echo "=== 部署完成！==="
echo ""
echo "运行压测命令示例："
echo "./wsbench -target 'ws://你的服务器IP:8084/ws' -conns 30000 -ramp 3m -duration 5m -max-cps 500"
echo ""
