package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EthanQC/IM/pkg/zlog"
	"github.com/EthanQC/IM/services/delivery_service/internal/adapters/in/ws"
	"github.com/EthanQC/IM/services/delivery_service/internal/adapters/out/db"
	"github.com/EthanQC/IM/services/delivery_service/internal/adapters/out/mq"
	redisRepo "github.com/EthanQC/IM/services/delivery_service/internal/adapters/out/redis"
	"github.com/EthanQC/IM/services/delivery_service/internal/application"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
)

func main() {
	// 加载配置
	if err := loadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	os.Setenv("APP_ENV", env)
	logCfgPath := filepath.Join(".", "configs", fmt.Sprintf("config.%s.yaml", env))
	if _, err := os.Stat(logCfgPath); os.IsNotExist(err) {
		logCfgPath = filepath.Join("..", "configs", fmt.Sprintf("config.%s.yaml", env))
	}

	logCfg, err := zlog.LoadConfig(logCfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载日志配置失败: %v\n", err)
		os.Exit(1)
	}
	logCfg.Service = "delivery-service"
	zlog.MustInitGlobal(*logCfg)
	defer zap.L().Sync()

	logger := zap.L()
	logger.Info("delivery_service starting", zap.String("env", env))

	// 初始化数据库
	database, err := initDB()
	if err != nil {
		logger.Fatal("Failed to init database", zap.Error(err))
	}

	// 初始化Redis
	redisClient, err := initRedis()
	if err != nil {
		logger.Fatal("Failed to init redis", zap.Error(err))
	}

	// 获取服务器地址（用于路由）
	serverAddr := fmt.Sprintf("%s:%d", getHostname(), viper.GetInt("server.http_port"))

	// 初始化仓储
	onlineUserRepo := redisRepo.NewEnhancedOnlineUserRepositoryRedis(redisClient, serverAddr)
	pendingMsgRepo := db.NewPendingMessageRepositoryMySQL(database)
	syncStateRepo := redisRepo.NewSyncStateRepositoryRedis(redisClient)
	pendingAckRepo := redisRepo.NewPendingAckRepositoryRedis(redisClient)

	// 初始化增强版连接管理器
	connManager := ws.NewEnhancedConnectionManager()

	// 初始化用例层
	deliveryUseCase := application.NewDeliveryUseCase(
		onlineUserRepo,
		pendingMsgRepo,
		connManager,
		nil, // PushService 暂时不实现
	)

	// 设置待确认仓储
	if duc, ok := deliveryUseCase.(*application.DeliveryUseCaseImpl); ok {
		duc.SetPendingAckRepo(pendingAckRepo)
	}

	connUseCase := application.NewConnectionUseCase(onlineUserRepo, deliveryUseCase)

	// 初始化同步用例（需要消息查询仓储，这里暂时设为nil）
	var syncUseCase in.SyncUseCase // 需要 message_service 的仓储，跨服务调用

	// 初始化ACK用例
	ackUseCase := application.NewAckUseCase(pendingAckRepo, syncStateRepo, connManager)

	// 初始化WebRTC信令用例
	signalingConfig := application.SignalingConfig{
		STUNServers: viper.GetStringSlice("webrtc.stun_servers"),
		TURNServers: []in.TURNServer{}, // 可从配置读取
	}
	if len(signalingConfig.STUNServers) == 0 {
		signalingConfig.STUNServers = []string{"stun:stun.l.google.com:19302"}
	}
	signalingUseCase := application.NewSignalingUseCase(signalingConfig, connManager)

	// 初始化Kafka消费者（使用可靠消费者）
	kafkaBrokers := viper.GetStringSlice("kafka.brokers")
	groupID := viper.GetString("kafka.group_id")
	consumer, err := mq.NewReliableKafkaConsumer(kafkaBrokers, groupID, deliveryUseCase)
	if err != nil {
		logger.Fatal("Failed to init kafka consumer", zap.Error(err))
	}

	// 启动Kafka消费者
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := consumer.Start(ctx); err != nil {
		logger.Fatal("Failed to start kafka consumer", zap.Error(err))
	}

	// 初始化增强版WebSocket服务器
	wsServer := ws.NewEnhancedWSServer(
		connManager,
		connUseCase,
		syncUseCase,
		ackUseCase,
		signalingUseCase,
	)

	// 初始化HTTP服务器
	router := gin.Default()

	// WebSocket端点
	router.GET("/ws", func(c *gin.Context) {
		// 从JWT或Query中获取用户信息
		userID := c.GetUint64("user_id")
		if userID == 0 {
			// 尝试从query获取（用于测试或内网）
			if userIDStr := c.Query("user_id"); userIDStr != "" {
				if parsed, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
					userID = parsed
				}
			}
		}
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		deviceID := c.Query("device_id")
		if deviceID == "" {
			deviceID = "default"
		}
		platform := c.Query("platform")
		if platform == "" {
			platform = "web"
		}

		wsServer.HandleConnection(c.Writer, c.Request, userID, deviceID, platform)
	})

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 统计信息
	router.GET("/stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, wsServer.GetStats())
	})

	// 启动HTTP服务器
	httpPort := viper.GetInt("server.http_port")
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,
	}

	go func() {
		logger.Info("Delivery server starting", zap.Int("port", httpPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("HTTP server shutdown error", zap.Error(err))
	}

	if err := consumer.Stop(); err != nil {
		logger.Warn("Kafka consumer stop error", zap.Error(err))
	}

	// 停止信令服务
	if su, ok := signalingUseCase.(*application.SignalingUseCaseImpl); ok {
		su.Stop()
	}

	logger.Info("Server exited properly")
}

func loadConfig() error {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("../configs")
	viper.AddConfigPath("../../configs")

	return viper.ReadInConfig()
}

func initDB() (*gorm.DB, error) {
	dsn := viper.GetString("mysql.dsn")

	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(viper.GetInt("mysql.max_idle_conns"))
	sqlDB.SetMaxOpenConns(viper.GetInt("mysql.max_open_conns"))
	sqlDB.SetConnMaxLifetime(time.Hour)

	return database, nil
}

func initRedis() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     viper.GetString("redis.addr"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
		PoolSize: viper.GetInt("redis.pool_size"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

// getHostname 获取当前服务器的主机名或IP
func getHostname() string {
	// 优先使用配置的地址
	if addr := viper.GetString("server.advertise_addr"); addr != "" {
		return addr
	}

	// 尝试获取主机名
	hostname, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return hostname
}
