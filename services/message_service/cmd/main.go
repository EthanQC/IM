package main

import (
	"context"
	"fmt"
	"net"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/pkg/zlog"
	"github.com/EthanQC/IM/services/message_service/internal/adapters/in/grpc/server"
	httpAdapter "github.com/EthanQC/IM/services/message_service/internal/adapters/in/http"
	"github.com/EthanQC/IM/services/message_service/internal/adapters/in/ws"
	"github.com/EthanQC/IM/services/message_service/internal/adapters/out/db"
	grpcOut "github.com/EthanQC/IM/services/message_service/internal/adapters/out/grpc"
	"github.com/EthanQC/IM/services/message_service/internal/adapters/out/mq"
	redisRepo "github.com/EthanQC/IM/services/message_service/internal/adapters/out/redis"
	"github.com/EthanQC/IM/services/message_service/internal/application"
	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
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
	logCfg.Service = "message-service"
	zlog.MustInitGlobal(*logCfg)
	defer zap.L().Sync()

	logger := zap.L()
	logger.Info("message_service starting", zap.String("env", env))

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

	// 初始化Kafka发布器
	kafkaBrokers := viper.GetStringSlice("kafka.brokers")
	eventPublisher, err := mq.NewKafkaEventPublisher(kafkaBrokers)
	if err != nil {
		logger.Fatal("Failed to init kafka publisher", zap.Error(err))
	}

	// 初始化仓储层
	messageRepo := db.NewMessageRepositoryMySQL(database)

	// 使用Redis Lua脚本实现的原子序号生成器
	sequenceRepo := redisRepo.NewSequenceRepositoryRedis(redisClient)

	// 使用Redis实现的收件箱仓储（热数据缓存）
	inboxRepo := redisRepo.NewInboxRepositoryRedis(redisClient)

	// Timeline 仓储（热消息缓存）
	timelineRepo := redisRepo.NewTimelineRepositoryRedis(redisClient)

	// 初始化会话成员仓储（gRPC）
	var memberRepo out.ConversationMemberRepository
	convAddr := viper.GetString("grpc.conversation_addr")
	if convAddr == "" {
		logger.Fatal("conversation service address is required")
	}
	convTimeout := viper.GetDuration("grpc.timeout")
	if convTimeout == 0 {
		convTimeout = 3 * time.Second
	}
	convConn, err := grpc.Dial(convAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal("Failed to connect conversation service", zap.Error(err))
	}
	defer convConn.Close()
	memberRepo = grpcOut.NewConversationClient(imv1.NewConversationServiceClient(convConn), convTimeout)

	// 初始化应用层（增强版）
	messageUseCase := application.NewEnhancedMessageUseCase(
		messageRepo,
		sequenceRepo,
		inboxRepo,
		timelineRepo,
		memberRepo,
		eventPublisher,
	)

	// 初始化WebSocket Hub
	hub := ws.NewHub(messageUseCase)
	go hub.Run()

	// 初始化HTTP服务器
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		if userIDStr := c.GetHeader("X-User-ID"); userIDStr != "" {
			if userID, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
				c.Set("user_id", userID)
			}
		}
		c.Next()
	})
	chatController := httpAdapter.NewChatController(messageUseCase)
	apiGroup := router.Group("/api/v1")
	chatController.RegisterRoutes(apiGroup)

	// WebSocket路由
	router.GET("/ws", func(c *gin.Context) {
		// 从Token中获取用户ID和设备ID
		userID := c.GetUint64("user_id")
		deviceID := c.Query("device_id")
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if deviceID == "" {
			deviceID = "default"
		}
		hub.ServeWs(c.Writer, c.Request, userID, deviceID)
	})

	// 启动HTTP服务器
	httpPort := viper.GetInt("server.http_port")
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,
	}
	go func() {
		logger.Info("HTTP server starting", zap.Int("port", httpPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// 启动gRPC服务器
	grpcPort := viper.GetInt("server.grpc_port")
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		logger.Fatal("Failed to listen on gRPC port", zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	messageGrpcServer := server.NewMessageServer(messageUseCase)
	server.RegisterMessageServiceServer(grpcServer, messageGrpcServer)

	go func() {
		logger.Info("gRPC server starting", zap.Int("port", grpcPort))
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Warn("HTTP server shutdown error", zap.Error(err))
	}
	grpcServer.GracefulStop()

	logger.Info("Servers exited properly")
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

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	return nil
}

func initDB() (*gorm.DB, error) {
	dsn := viper.GetString("mysql.dsn")

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(viper.GetInt("mysql.max_idle_conns"))
	sqlDB.SetMaxOpenConns(viper.GetInt("mysql.max_open_conns"))
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
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
		return nil, fmt.Errorf("failed to connect redis: %w", err)
	}

	return client, nil
}
