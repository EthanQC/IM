package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-redis/redis/v8"
	kafka "github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/EthanQC/IM/pkg/zlog"
	grpcAdapter "github.com/EthanQC/IM/services/identity_service/internal/adapters/in/gRPC"
	httpAdapter "github.com/EthanQC/IM/services/identity_service/internal/adapters/in/http"
	aliyunSms "github.com/EthanQC/IM/services/identity_service/internal/adapters/out/aliyun"
	mysqlRepo "github.com/EthanQC/IM/services/identity_service/internal/adapters/out/mysql"
	redisRepo "github.com/EthanQC/IM/services/identity_service/internal/adapters/out/redis"
	authApp "github.com/EthanQC/IM/services/identity_service/internal/application/auth"
	contactApp "github.com/EthanQC/IM/services/identity_service/internal/application/contact"
	smsApp "github.com/EthanQC/IM/services/identity_service/internal/application/sms"
	statusApp "github.com/EthanQC/IM/services/identity_service/internal/application/status"
	userApp "github.com/EthanQC/IM/services/identity_service/internal/application/user"
	"github.com/EthanQC/IM/services/identity_service/pkg/jwt"
)

// Config 定义从 YAML 加载的所有配置项
type Config struct {
	Server struct {
		HTTPPort int    `mapstructure:"http_port"`
		GrpcPort int    `mapstructure:"grpc_port"`
		Mode     string `mapstructure:"mode"`
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
	// 加载配置
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("../configs")
	viper.SetDefault("server.grpc_port", 9080)
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "读取配置文件失败: %v\n", err)
		os.Exit(1)
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "解析配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
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
	logCfg.Service = "identity-service"
	zlog.MustInitGlobal(*logCfg)
	defer zap.L().Sync()

	logger := zap.L()
	logger.Info("identity_service starting", zap.String("env", env))

	// 构造监听地址
	httpAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	grpcAddr := fmt.Sprintf(":%d", cfg.Server.GrpcPort)

	// 初始化 MySQL
	db, err := gorm.Open(mysqlDriver.Open(cfg.Mysql.DSN), &gorm.Config{})
	if err != nil {
		logger.Fatal("连接 MySQL 失败", zap.Error(err))
	}
	if cfg.Server.Mode != "release" {
		if err := db.AutoMigrate(&mysqlRepo.RefreshTokenModel{}); err != nil {
			logger.Fatal("初始化 refresh_tokens 表失败", zap.Error(err))
		}
	}
	logger.Info("MySQL 连接成功")

	// 初始化 Redis
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("连接 Redis 失败", zap.Error(err))
	}
	logger.Info("Redis 连接成功")

	// JWT 管理器
	jwtMgr := jwt.NewManager(cfg.JWT.Secret)

	// 仓库
	authCodeRepo := redisRepo.NewAuthCodeRepoRedis(rdb, cfg.Code.TTL)
	accessTokenRepo := redisRepo.NewAccessTokenRepoRedis(rdb, cfg.JWT.AccessTTL)
	refreshTokenRepo := mysqlRepo.NewRefreshTokenRepoMysql(db)
	userStatusRepo := mysqlRepo.NewUserStatusRepoMysql(db)
	userRepo := mysqlRepo.NewUserRepositoryMySQL(db)
	contactRepo := mysqlRepo.NewContactRepositoryMySQL(db)
	contactApplyRepo := mysqlRepo.NewContactApplyRepositoryMySQL(db)
	blacklistRepo := mysqlRepo.NewBlacklistRepositoryMySQL(db)

	// 用户用例
	userUC := userApp.NewUserUseCaseImpl(userRepo, jwtMgr, nil)
	contactUC := contactApp.NewContactUseCaseImpl(contactRepo, contactApplyRepo, blacklistRepo, userRepo)

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
		userRepo,
	)

	// 启动 HTTP 服务
	mux := http.NewServeMux()
	httpAdapter.NewAuthHandler(authUC).RegisterRoutes(mux)

	httpAdapter.NewSMSHandler(smsSendUC).RegisterRoutes(mux)

	go func() {
		logger.Info("HTTP 服务启动", zap.String("addr", httpAddr))
		if err := http.ListenAndServe(httpAddr, mux); err != nil {
			logger.Fatal("HTTP 服务失败", zap.Error(err))
		}
	}()

	// 启动 gRPC 服务
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatal("监听 gRPC 失败", zap.Error(err))
	}
	grpcServer := grpc.NewServer()
	grpcAdapter.NewAuthServer(
		authUC,
		userUC,
		contactUC,
		smsSendUC,
	).RegisterServer(grpcServer)
	logger.Info("gRPC 服务启动", zap.String("addr", grpcAddr))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("gRPC 服务失败", zap.Error(err))
	}
}
