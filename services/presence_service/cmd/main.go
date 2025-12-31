package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	grpcServer "github.com/EthanQC/IM/services/presence_service/internal/adapters/in/grpc"
	redisRepo "github.com/EthanQC/IM/services/presence_service/internal/adapters/out/redis"
	"github.com/EthanQC/IM/services/presence_service/internal/application"
)

func main() {
	// 加载配置
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化Redis
	redisClient, err := initRedis()
	if err != nil {
		log.Fatalf("Failed to init redis: %v", err)
	}
	defer redisClient.Close()

	// 初始化仓储
	presenceRepo := redisRepo.NewPresenceRepositoryRedis(redisClient)

	// 初始化用例
	presenceUseCase := application.NewPresenceUseCase(presenceRepo, nil)

	// 启动gRPC服务器
	grpcPort := viper.GetInt("server.grpc_port")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	presenceServer := grpcServer.NewPresenceServer(presenceUseCase)
	grpcServer.RegisterPresenceServiceServer(server, presenceServer)

	go func() {
		log.Printf("Presence service starting on port %d", grpcPort)
		if err := server.Serve(listener); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	server.GracefulStop()
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
