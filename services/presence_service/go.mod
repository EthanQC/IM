module github.com/EthanQC/IM/services/presence_service

go 1.24.2

require (
	github.com/EthanQC/IM/api v0.0.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/spf13/viper v1.18.2
	google.golang.org/grpc v1.62.1
	google.golang.org/protobuf v1.33.0
)

replace github.com/EthanQC/IM/api => ../../api
