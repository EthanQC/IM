module github.com/EthanQC/IM/services/file_service

go 1.24.2

require (
	github.com/EthanQC/IM/api v0.0.0
	github.com/gin-gonic/gin v1.9.1
	github.com/google/uuid v1.6.0
	github.com/minio/minio-go/v7 v7.0.69
	github.com/spf13/viper v1.18.2
	google.golang.org/grpc v1.62.1
	google.golang.org/protobuf v1.33.0
	gorm.io/driver/mysql v1.5.6
	gorm.io/gorm v1.25.8
)

replace github.com/EthanQC/IM/api => ../../api
