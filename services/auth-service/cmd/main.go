package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/EthanQC/IM/pkg/logger/zlog"
	"github.com/EthanQC/IM/services/auth-service/internal/adapters/in/http"
	"github.com/EthanQC/IM/services/auth-service/internal/adapters/out/aliyun"
	"github.com/EthanQC/IM/services/auth-service/internal/adapters/out/eventbus"
	"github.com/EthanQC/IM/services/auth-service/internal/adapters/out/mysql"
	"github.com/EthanQC/IM/services/auth-service/internal/adapters/out/redis"
	authApp "github.com/EthanQC/IM/services/auth-service/internal/application/auth"
	smsApp "github.com/EthanQC/IM/services/auth-service/internal/application/sms"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 初始化全局日志
	zlog.Init(os.Getenv("LOG_LEVEL"))

	// Kafka EventBus
	kafkaBrokers := []string{os.Getenv("KAFKA_BROKER")}
	evBus, err := eventbus.NewKafkaEventBus(kafkaBrokers)
	if err != nil {
		log.Fatalf("初始化 Kafka 失败: %v", err)
	}

	// MySQL 连接
	dsn := os.Getenv("MYSQL_DSN")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接 MySQL 失败: %v", err)
	}

	// Redis 连接
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASS"),
		DB:       0,
	})

	// 阿里云 SMS 客户端
	aliClient, err := dysmsapi.NewClientWithAccessKey(
		os.Getenv("ALIYUN_REGION"),
		os.Getenv("ALIYUN_ACCESS_KEY_ID"),
		os.Getenv("ALIYUN_ACCESS_KEY_SECRET"),
	)
	if err != nil {
		log.Fatalf("初始化阿里云 SMS 客户端失败: %v", err)
	}
	smsClient := aliyun.NewAliyunSMSClient(aliClient, os.Getenv("ALIYUN_SIGN_NAME"), os.Getenv("ALIYUN_TEMPLATE_CODE"))

	// 构造 Out Adapters
	authCodeRepo := redis.NewAuthCodeRepoRedis(rdb, 5*time.Minute)
	accessRepo := redis.NewAccessTokenRepoRedis(rdb, 24*time.Hour)
	refreshRepo := mysql.NewRefreshTokenRepoMysql(db)
	userStatusRepo := mysql.NewUserStatusRepoMysql(db)

	// 构造 UseCase
	authUC := authApp.NewAuthUseCase(refreshRepo, accessRepo, authCodeRepo, evBus)
	smsUC := smsApp.NewSendCodeUseCase(authCodeRepo, smsClient, evBus)

	// 构造 HTTP Handlers
	authHandler := http.NewAuthHandler(authUC)
	smsHandler := http.NewSMSHandler(smsUC)

	mux := http.NewServeMux()
	authHandler.RegisterRoutes(mux)
	smsHandler.RegisterRoutes(mux)

	// 启动 HTTP 服务
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	zlog.FromContext(context.Background()).Info("Auth-service listening", zlog.String("port", port))
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("HTTP 服务启动失败: %v", err)
	}
}
