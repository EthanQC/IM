package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/pkg/zlog"
	grpcServer "github.com/EthanQC/IM/services/file_service/internal/adapters/in/grpc"
	minioAdapter "github.com/EthanQC/IM/services/file_service/internal/adapters/out/minio"
	mysqlRepo "github.com/EthanQC/IM/services/file_service/internal/adapters/out/mysql"
	"github.com/EthanQC/IM/services/file_service/internal/application"
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
	logCfg.Service = "file-service"
	zlog.MustInitGlobal(*logCfg)
	defer zap.L().Sync()

	logger := zap.L()
	logger.Info("file_service starting", zap.String("env", env))

	// 初始化数据库
	database, err := initDB()
	if err != nil {
		logger.Fatal("Failed to init database", zap.Error(err))
	}

	// 初始化MinIO
	minioStorage, err := minioAdapter.NewMinIOStorage(
		viper.GetString("minio.endpoint"),
		viper.GetString("minio.access_key"),
		viper.GetString("minio.secret_key"),
		viper.GetBool("minio.use_ssl"),
	)
	if err != nil {
		logger.Fatal("Failed to init minio", zap.Error(err))
	}

	// 确保 bucket存在
	if storage, ok := minioStorage.(*minioAdapter.MinIOStorage); ok {
		bucket := viper.GetString("minio.bucket")
		region := viper.GetString("minio.region")
		if err := storage.EnsureBucket(context.Background(), bucket, region); err != nil {
			logger.Warn("Failed to ensure bucket", zap.Error(err))
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

	// 初始化消息服务客户端
	msgAddr := viper.GetString("grpc.message_addr")
	if msgAddr == "" {
		logger.Fatal("message service address is required")
	}
	msgConn, err := grpc.Dial(msgAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal("Failed to connect message service", zap.Error(err))
	}
	defer msgConn.Close()
	messageClient := imv1.NewMessageServiceClient(msgConn)

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
		logger.Info("HTTP server starting", zap.Int("port", httpPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// 启动gRPC服务器
	grpcPort := viper.GetInt("server.grpc_port")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	server := grpc.NewServer()
	fileServer := grpcServer.NewFileServer(fileUseCase, messageClient)
	grpcServer.RegisterFileServiceServer(server, fileServer)

	go func() {
		logger.Info("gRPC server starting", zap.Int("port", grpcPort))
		if err := server.Serve(listener); err != nil {
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

	httpServer.Shutdown(ctx)
	server.GracefulStop()

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
	if viper.GetString("server.mode") != "release" {
		if err := database.AutoMigrate(&mysqlRepo.FileModel{}); err != nil {
			return nil, err
		}
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
