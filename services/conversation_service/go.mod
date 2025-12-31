module github.com/EthanQC/IM/services/conversation_service

go 1.24.2

require (
	github.com/EthanQC/IM v0.0.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/segmentio/kafka-go v0.4.47
	github.com/spf13/viper v1.20.1
	google.golang.org/grpc v1.69.4
	google.golang.org/protobuf v1.36.3
	gorm.io/driver/mysql v1.5.7
	gorm.io/gorm v1.25.12
)

replace github.com/EthanQC/IM => ../..
