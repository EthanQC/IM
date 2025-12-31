package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"

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
	cfgPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	viper.SetConfigFile(*cfgPath)
	viper.SetConfigType("yaml")
	viper.SetDefault("server.grpc_port", 9091)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("解析配置失败: %v", err)
	}

	// 初始化MySQL
	db, err := gorm.Open(mysqlDriver.Open(cfg.MySQL.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接MySQL失败: %v", err)
	}

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
		log.Fatalf("监听失败: %v", err)
	}

	log.Printf("Conversation Service gRPC listening on %s", grpcAddr)

	// 优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC服务启动失败: %v", err)
	}
}
