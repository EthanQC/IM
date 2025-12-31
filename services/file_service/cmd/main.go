package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	grpcServer "github.com/EthanQC/IM/services/file_service/internal/adapters/in/grpc"
	minioAdapter "github.com/EthanQC/IM/services/file_service/internal/adapters/out/minio"
	mysqlRepo "github.com/EthanQC/IM/services/file_service/internal/adapters/out/mysql"
	"github.com/EthanQC/IM/services/file_service/internal/application"
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

	// 初始化MinIO
	minioStorage, err := minioAdapter.NewMinIOStorage(
		viper.GetString("minio.endpoint"),
		viper.GetString("minio.access_key"),
		viper.GetString("minio.secret_key"),
		viper.GetBool("minio.use_ssl"),
	)
	if err != nil {
		log.Fatalf("Failed to init minio: %v", err)
	}

	// 确保bucket存在
	if storage, ok := minioStorage.(*minioAdapter.MinIOStorage); ok {
		bucket := viper.GetString("minio.bucket")
		region := viper.GetString("minio.region")
		if err := storage.EnsureBucket(context.Background(), bucket, region); err != nil {
			log.Printf("Warning: Failed to ensure bucket: %v", err)
		}
	}

	// 初始化仓储
	fileRepo := mysqlRepo.NewFileRepositoryMySQL(database)

	// 初始化用例
	fileUseCase := application.NewFileUseCase(
		fileRepo,
		minioStorage,
		viper.GetString("minio.bucket"),
		viper.GetString("server.callback_url"),
	)

	// 启动HTTP服务器（用于文件上传回调）
	router := gin.Default()
	router.POST("/callback/upload", func(c *gin.Context) {
		// 处理MinIO上传回调
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

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
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	fileServer := grpcServer.NewFileServer(fileUseCase)
	grpcServer.RegisterFileServiceServer(server, fileServer)

	go func() {
		log.Printf("gRPC server starting on port %d", grpcPort)
		if err := server.Serve(listener); err != nil {
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

	httpServer.Shutdown(ctx)
	server.GracefulStop()

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
