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
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
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
		log.Fatalf("load config: %v", err)
	}

	gw := &Gateway{
		cfg:     cfg,
		router:  gin.Default(),
		timeout: cfg.Server.GrpcTimeout,
	}

	// 连接各个gRPC服务
	if cfg.Server.GrpcAddrIdentity != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrIdentity, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: failed to connect identity service: %v", err)
		} else {
			gw.identityClient = imv1.NewIdentityServiceClient(conn)
		}
	}

	if cfg.Server.GrpcAddrConversation != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrConversation, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: failed to connect conversation service: %v", err)
		} else {
			gw.conversationClient = imv1.NewConversationServiceClient(conn)
		}
	}

	if cfg.Server.GrpcAddrMessage != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrMessage, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: failed to connect message service: %v", err)
		} else {
			gw.messageClient = imv1.NewMessageServiceClient(conn)
		}
	}

	if cfg.Server.GrpcAddrPresence != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrPresence, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: failed to connect presence service: %v", err)
		} else {
			gw.presenceClient = imv1.NewPresenceServiceClient(conn)
		}
	}

	if cfg.Server.GrpcAddrFile != "" {
		conn, err := grpc.Dial(cfg.Server.GrpcAddrFile, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: failed to connect file service: %v", err)
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
		log.Printf("API Gateway listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway failed: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("API Gateway shutdown")
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
