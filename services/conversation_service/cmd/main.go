package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/EthanQC/IM/pkg/zlog"
	grpcAdapter "github.com/EthanQC/IM/services/conversation_service/internal/adapters/in/grpc"
	mysqlRepo "github.com/EthanQC/IM/services/conversation_service/internal/adapters/out/mysql"
	"github.com/EthanQC/IM/services/conversation_service/internal/application/conversation"
)

type Config struct {
	Server struct {
		GRPCPort int `mapstructure:"grpc_port"`
	} `mapstructure:"server"`
	MySQL struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"mysql"`
	Kafka struct {
		Brokers []string `mapstructure:"brokers"`
	} `mapstructure:"kafka"`
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
	viper.SetDefault("server.grpc_port", 9081)
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
	logCfg.Service = "conversation-service"
	zlog.MustInitGlobal(*logCfg)
	defer zap.L().Sync()

	logger := zap.L()
	logger.Info("conversation_service starting", zap.String("env", env))

	// 初始化MySQL
	db, err := gorm.Open(mysqlDriver.Open(cfg.MySQL.DSN), &gorm.Config{})
	if err != nil {
		logger.Fatal("连接MySQL失败", zap.Error(err))
	}
	logger.Info("MySQL 连接成功")

	// 初始化仓储
	convRepo := mysqlRepo.NewConversationRepositoryMySQL(db)
	participantRepo := mysqlRepo.NewParticipantRepositoryMySQL(db)

	// 初始化用例
	convUC := conversation.NewConversationUseCaseImpl(convRepo, participantRepo, nil)

	// 初始化gRPC服务器
	grpcServer := grpc.NewServer()
	convServer := grpcAdapter.NewConversationServer(convUC)
	convServer.RegisterServer(grpcServer)

	// 启动gRPC服务
	grpcAddr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatal("监听失败", zap.Error(err))
	}

	logger.Info("Conversation Service gRPC listening", zap.String("addr", grpcAddr))

	// 优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx

	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("gRPC服务启动失败", zap.Error(err))
	}
}
