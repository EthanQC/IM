package out

import (
	"context"
	"io"
	"time"

	"github.com/EthanQC/IM/services/file_service/internal/domain/entity"
)

// FileRepository 文件仓储接口
type FileRepository interface {
	// Create 创建文件记录
	Create(ctx context.Context, file *entity.FileUpload) error
	// GetByID 根据ID获取文件
	GetByID(ctx context.Context, id uint64) (*entity.FileUpload, error)
	// GetByObjectKey 根据ObjectKey获取文件
	GetByObjectKey(ctx context.Context, objectKey string) (*entity.FileUpload, error)
	// UpdateStatus 更新状态
	UpdateStatus(ctx context.Context, id uint64, status entity.FileStatus) error
	// Update 更新文件信息
	Update(ctx context.Context, file *entity.FileUpload) error
}

// ObjectStorage 对象存储接口
type ObjectStorage interface {
	// GeneratePresignedPutURL 生成预签名上传URL
	GeneratePresignedPutURL(ctx context.Context, bucket, objectKey, contentType string, expiry time.Duration) (string, error)
	// GeneratePresignedGetURL 生成预签名下载URL
	GeneratePresignedGetURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	// Upload 上传文件
	Upload(ctx context.Context, bucket, objectKey string, reader io.Reader, size int64, contentType string) error
	// Delete 删除文件
	Delete(ctx context.Context, bucket, objectKey string) error
	// Exists 检查文件是否存在
	Exists(ctx context.Context, bucket, objectKey string) (bool, error)
	// GetObjectInfo 获取对象信息
	GetObjectInfo(ctx context.Context, bucket, objectKey string) (*ObjectInfo, error)
}

// ObjectInfo 对象信息
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified time.Time
	ETag         string
}
