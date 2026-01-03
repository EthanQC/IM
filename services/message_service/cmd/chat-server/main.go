package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/message_service/internal/adapters/in/grpc/server"
	httpAdapter "github.com/EthanQC/IM/services/message_service/internal/adapters/in/http"
	"github.com/EthanQC/IM/services/message_service/internal/adapters/in/ws"
	"github.com/EthanQC/IM/services/message_service/internal/adapters/out/db"
	grpcOut "github.com/EthanQC/IM/services/message_service/internal/adapters/out/grpc"
	"github.com/EthanQC/IM/services/message_service/internal/adapters/out/mq"
	"github.com/EthanQC/IM/services/message_service/internal/application"
	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

func main() {
	// 加载配置
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库
	database, err := initDB()
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// 初始化Kafka发布器
	kafkaBrokers := viper.GetStringSlice("kafka.brokers")
	eventPublisher, err := mq.NewKafkaEventPublisher(kafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to init kafka publisher: %v", err)
	}

	// 初始化仓储层
	messageRepo := db.NewMessageRepositoryMySQL(database)
	sequenceRepo := db.NewSequenceRepositoryMySQL(database)
	inboxRepo := db.NewInboxRepositoryMySQL(database)

	// 初始化会话成员仓储（gRPC）
	var memberRepo out.ConversationMemberRepository
	convAddr := viper.GetString("grpc.conversation_addr")
	if convAddr == "" {
		log.Fatalf("conversation service address is required")
	}
	convTimeout := viper.GetDuration("grpc.timeout")
	if convTimeout == 0 {
		convTimeout = 3 * time.Second
	}
	convConn, err := grpc.Dial(convAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect conversation service: %v", err)
	}
	defer convConn.Close()
	memberRepo = grpcOut.NewConversationClient(imv1.NewConversationServiceClient(convConn), convTimeout)

	// 初始化应用层
	messageUseCase := application.NewMessageUseCaseImpl(
		messageRepo,
		sequenceRepo,
		inboxRepo,
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
		log.Printf("HTTP server starting on port %d", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// 启动gRPC服务器
	grpcPort := viper.GetInt("server.grpc_port")
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port: %v", err)
	}

	grpcServer := grpc.NewServer()
	messageGrpcServer := server.NewMessageServer(messageUseCase)
	server.RegisterMessageServiceServer(grpcServer, messageGrpcServer)

	go func() {
		log.Printf("gRPC server starting on port %d", grpcPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	grpcServer.GracefulStop()

	log.Println("Servers exited properly")
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
		Logger: logger.Default.LogMode(logger.Info),
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
