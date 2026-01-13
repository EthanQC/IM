package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/EthanQC/IM/pkg/zlog"
	grpcServer "github.com/EthanQC/IM/services/presence_service/internal/adapters/in/grpc"
	redisRepo "github.com/EthanQC/IM/services/presence_service/internal/adapters/out/redis"
	"github.com/EthanQC/IM/services/presence_service/internal/application"
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
	logCfg.Service = "presence-service"
	zlog.MustInitGlobal(*logCfg)
	defer zap.L().Sync()

	logger := zap.L()
	logger.Info("presence_service starting", zap.String("env", env))

	// 初始化Redis
	redisClient, err := initRedis()
	if err != nil {
		logger.Fatal("Failed to init redis", zap.Error(err))
	}
	defer redisClient.Close()
	logger.Info("Redis 连接成功")

	// 初始化仓储
	presenceRepo := redisRepo.NewPresenceRepositoryRedis(redisClient)

	// 初始化用例
	presenceUseCase := application.NewPresenceUseCase(presenceRepo, nil)

	// 启动gRPC服务器
	grpcPort := viper.GetInt("server.grpc_port")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	server := grpc.NewServer()
	presenceServer := grpcServer.NewPresenceServer(presenceUseCase)
	grpcServer.RegisterPresenceServiceServer(server, presenceServer)

	go func() {
		logger.Info("Presence service starting", zap.Int("port", grpcPort))
		if err := server.Serve(listener); err != nil {
			logger.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	server.GracefulStop()
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
