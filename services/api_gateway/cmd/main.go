package main

import (
	"context"
	"fmt"
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

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/pkg/zlog"
)

type Config struct {
	Server struct {
		HTTPPort             int           `mapstructure:"http_port"`
		GrpcAddrIdentity     string        `mapstructure:"grpc_addr_identity"`
		GrpcAddrConversation string        `mapstructure:"grpc_addr_conversation"`
		GrpcAddrMessage      string        `mapstructure:"grpc_addr_message"`
		HttpAddrMessage      string        `mapstructure:"http_addr_message"`
		GrpcAddrPresence     string        `mapstructure:"grpc_addr_presence"`
		GrpcAddrFile         string        `mapstructure:"grpc_addr_file"`
		GrpcTimeout          time.Duration `mapstructure:"grpc_timeout"`
		ReadTimeout          time.Duration `mapstructure:"read_timeout"`
		WriteTimeout         time.Duration `mapstructure:"write_timeout"`
	} `mapstructure:"server"`
	JWT struct {
		Secret string `mapstructure:"secret"`
	} `mapstructure:"jwt"`
}

type Gateway struct {
	cfg                Config
	router             *gin.Engine
	identityClient     imv1.IdentityServiceClient
	conversationClient imv1.ConversationServiceClient
	messageClient      imv1.MessageServiceClient
	presenceClient     imv1.PresenceServiceClient
	fileClient         imv1.FileServiceClient
	timeout            time.Duration
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
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
	logCfg.Service = "api-gateway"
	zlog.MustInitGlobal(*logCfg)
	defer zap.L().Sync()

	logger := zap.L()
	logger.Info("api_gateway starting", zap.String("env", env))

	gw := &Gateway{
		cfg:     cfg,
		router:  gin.Default(),
		timeout: cfg.Server.GrpcTimeout,
	}

	// 连接各个gRPC服务
	if cfg.Server.GrpcAddrIdentity != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrIdentity, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Warn("failed to connect identity service", zap.Error(err))
		} else {
			gw.identityClient = imv1.NewIdentityServiceClient(conn)
		}
	}

	if cfg.Server.GrpcAddrConversation != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrConversation, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Warn("failed to connect conversation service", zap.Error(err))
		} else {
			gw.conversationClient = imv1.NewConversationServiceClient(conn)
		}
	}

	if cfg.Server.GrpcAddrMessage != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrMessage, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Warn("failed to connect message service", zap.Error(err))
		} else {
			gw.messageClient = imv1.NewMessageServiceClient(conn)
		}
	}

	if cfg.Server.GrpcAddrPresence != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrPresence, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Warn("failed to connect presence service", zap.Error(err))
		} else {
			gw.presenceClient = imv1.NewPresenceServiceClient(conn)
		}
	}

	if cfg.Server.GrpcAddrFile != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrFile, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Warn("failed to connect file service", zap.Error(err))
		} else {
			gw.fileClient = imv1.NewFileServiceClient(conn)
		}
	}

	gw.registerRoutes()

	addr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      gw.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("API Gateway listening", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("gateway failed", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("API Gateway shutdown")
}

func loadConfig() (Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("../configs")
	viper.SetDefault("server.grpc_timeout", "3s")
	viper.SetDefault("server.read_timeout", "5s")
	viper.SetDefault("server.write_timeout", "5s")

	var cfg Config
	if err := viper.ReadInConfig(); err != nil {
		return cfg, err
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
