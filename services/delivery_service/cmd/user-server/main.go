package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EthanQC/IM/services/delivery_service/internal/adapters/in/ws"
	"github.com/EthanQC/IM/services/delivery_service/internal/adapters/out/db"
	"github.com/EthanQC/IM/services/delivery_service/internal/adapters/out/mq"
	redisRepo "github.com/EthanQC/IM/services/delivery_service/internal/adapters/out/redis"
	"github.com/EthanQC/IM/services/delivery_service/internal/application"
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

	// 初始化Redis
	redisClient, err := initRedis()
	if err != nil {
		log.Fatalf("Failed to init redis: %v", err)
	}

	// 初始化仓储
	onlineUserRepo := redisRepo.NewOnlineUserRepositoryRedis(redisClient)
	pendingMsgRepo := db.NewPendingMessageRepositoryMySQL(database)

	// 初始化连接管理器
	connManager := ws.NewConnectionManager().(*ws.ConnectionManagerImpl)

	// 初始化用例层
	deliveryUseCase := application.NewDeliveryUseCase(
		onlineUserRepo,
		pendingMsgRepo,
		connManager,
		nil, // PushService 暂时不实现
	)
	connUseCase := application.NewConnectionUseCase(onlineUserRepo, deliveryUseCase)

	// 设置连接用例
	connManager.SetConnectionUseCase(connUseCase)

	// 初始化Kafka消费者
	kafkaBrokers := viper.GetStringSlice("kafka.brokers")
	groupID := viper.GetString("kafka.group_id")
	consumer, err := mq.NewKafkaMessageConsumer(kafkaBrokers, groupID, deliveryUseCase)
	if err != nil {
		log.Fatalf("Failed to init kafka consumer: %v", err)
	}

	// 启动Kafka消费者
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := consumer.Start(ctx); err != nil {
		log.Fatalf("Failed to start kafka consumer: %v", err)
	}

	// 初始化WebSocket服务器
	wsServer := ws.NewWSServer(connManager, connUseCase)

	// 初始化HTTP服务器
	router := gin.Default()
	
	// WebSocket端点
	router.GET("/ws", func(c *gin.Context) {
		// 从JWT或Query中获取用户信息
		userID := c.GetUint64("user_id")
		if userID == 0 {
			// 尝试从query获取（用于测试）
			userID = uint64(c.GetInt("user_id"))
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

	// 启动HTTP服务器
	httpPort := viper.GetInt("server.http_port")
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,
	}

	go func() {
		log.Printf("Delivery server starting on port %d", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	if err := consumer.Stop(); err != nil {
		log.Printf("Kafka consumer stop error: %v", err)
	}

	log.Println("Server exited properly")
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
		Logger: logger.Default.LogMode(logger.Info),
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
