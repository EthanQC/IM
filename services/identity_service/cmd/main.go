package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	kafka "github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"

	grpcAdapter "github.com/EthanQC/IM/services/identity_service/internal/adapters/in/gRPC"
	httpAdapter "github.com/EthanQC/IM/services/identity_service/internal/adapters/in/http"
	aliyunSms "github.com/EthanQC/IM/services/identity_service/internal/adapters/out/aliyun"
	mysqlRepo "github.com/EthanQC/IM/services/identity_service/internal/adapters/out/mysql"
	redisRepo "github.com/EthanQC/IM/services/identity_service/internal/adapters/out/redis"
	authApp "github.com/EthanQC/IM/services/identity_service/internal/application/auth"
	smsApp "github.com/EthanQC/IM/services/identity_service/internal/application/sms"
	statusApp "github.com/EthanQC/IM/services/identity_service/internal/application/status"
	userApp "github.com/EthanQC/IM/services/identity_service/internal/application/user"
	"github.com/EthanQC/IM/services/identity_service/pkg/jwt"
)

// Config 定义从 YAML 加载的所有配置项
type Config struct {
	Server struct {
		HTTPPort int `mapstructure:"port"`
		GrpcPort int `mapstructure:"grpc_port"`
	} `mapstructure:"server"`
	JWT struct {
		Secret     string        `mapstructure:"secret"`
		AccessTTL  time.Duration `mapstructure:"access_ttl"`
		RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
	} `mapstructure:"jwt"`
	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`
	Mysql struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"mysql"`
	Kafka struct {
		Brokers []string `mapstructure:"brokers"`
		Topic   string   `mapstructure:"topic"`
	} `mapstructure:"kafka"`
	Code struct {
		TTL         time.Duration `mapstructure:"ttl"`
		MaxAttempts int           `mapstructure:"max_attempts"`
	} `mapstructure:"code"`
	SMS struct {
		Region          string `mapstructure:"region"`
		AccessKeyID     string `mapstructure:"access_key_id"`
		AccessKeySecret string `mapstructure:"access_key_secret"`
		SignName        string `mapstructure:"sign_name"`
		TemplateCode    string `mapstructure:"template_code"`
	} `mapstructure:"sms"`
}

func main() {
	// 仅定义一个配置文件路径参数
	cfgPath := flag.String("config", "configs/dev/identity_service.yaml", "配置文件路径（YAML）")
	flag.Parse()

	// 加载配置
	viper.SetConfigFile(*cfgPath)
	viper.SetConfigType("yaml")
	// 默认值：如果配置中未包含 grpc_port，则使用 9090
	viper.SetDefault("server.grpc_port", 9090)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("解析配置失败: %v", err)
	}

	// 构造监听地址
	httpAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	grpcAddr := fmt.Sprintf(":%d", cfg.Server.GrpcPort)

	// 初始化 MySQL
	db, err := gorm.Open(mysqlDriver.Open(cfg.Mysql.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接 MySQL 失败: %v", err)
	}

	// 初始化 Redis
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("连接 Redis 失败: %v", err)
	}

	// JWT 管理器
	jwtMgr := jwt.NewManager(cfg.JWT.Secret)

	// 仓库
	authCodeRepo := redisRepo.NewAuthCodeRepoRedis(rdb, cfg.Code.TTL)
	accessTokenRepo := redisRepo.NewAccessTokenRepoRedis(rdb, cfg.JWT.AccessTTL)
	refreshTokenRepo := mysqlRepo.NewRefreshTokenRepoMysql(db)
	userStatusRepo := mysqlRepo.NewUserStatusRepoMysql(db)
	userRepo := mysqlRepo.NewUserRepositoryMySQL(db)

	// 用户用例
	userUC := userApp.NewUserUseCaseImpl(userRepo, jwtMgr, nil)

	// 短信服务用例
	smsClient, _ := aliyunSms.NewAliyunSMSClient(
		cfg.SMS.Region,
		cfg.SMS.AccessKeyID,
		cfg.SMS.AccessKeySecret,
		cfg.SMS.SignName,
		cfg.SMS.TemplateCode,
	)
	smsVerifyUC := smsApp.NewVerifyCodeUseCase(authCodeRepo, cfg.Code.MaxAttempts)
	smsSendUC := smsApp.NewSendCodeUseCase(authCodeRepo, smsClient, cfg.Code.TTL)

	// Kafka 发布者（预留事件用）
	writer := kafka.NewWriter(kafka.WriterConfig{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.Topic})
	defer writer.Close()
	_ = writer // 仅示意，事件发布后续接入

	// 认证用例
	genUC := authApp.NewGenerateTokenUseCase(
		refreshTokenRepo,
		userStatusRepo,
		jwtMgr,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
	)
	refreshUC := authApp.NewRefreshTokenUseCase(
		refreshTokenRepo,
		userStatusRepo,
		jwtMgr,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
	)
	revokeUC := authApp.NewRevokeTokenUseCase(accessTokenRepo, refreshTokenRepo)
	statusUC := statusApp.NewCheckUserStatusUseCase(userStatusRepo)
	authUC := authApp.NewDefaultAuthUseCase(
		genUC,
		refreshUC,
		revokeUC,
		statusUC,
		smsVerifyUC,
	)

	// 启动 HTTP 服务
	mux := http.NewServeMux()
	httpAdapter.NewAuthHandler(authUC).RegisterRoutes(mux)

	httpAdapter.NewSMSHandler(smsSendUC).RegisterRoutes(mux)

	go func() {
		log.Printf("HTTP 服务启动: %s", httpAddr)
		if err := http.ListenAndServe(httpAddr, mux); err != nil {
			log.Fatalf("HTTP 服务失败: %v", err)
		}
	}()

	// 启动 gRPC 服务
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("监听 gRPC 失败: %v", err)
	}
	grpcServer := grpc.NewServer()
	grpcAdapter.NewAuthServer(
		authUC,
		userUC,
		smsSendUC,
	).RegisterServer(grpcServer)
	log.Printf("gRPC 服务启动: %s", grpcAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC 服务失败: %v", err)
	}
}
