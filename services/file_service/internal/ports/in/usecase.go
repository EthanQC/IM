package in

import (
	"context"

	"github.com/EthanQC/IM/services/file_service/internal/domain/entity"
)

// FileUseCase 文件用例接口
type FileUseCase interface {
	// CreateUpload 创建上传
	CreateUpload(ctx context.Context, input *CreateUploadInput) (*entity.UploadToken, error)
	// CompleteUpload 完成上传
	CompleteUpload(ctx context.Context, input *CompleteUploadInput) (*entity.FileUpload, error)
	// GetFile 获取文件
	GetFile(ctx context.Context, id uint64) (*entity.FileUpload, error)
	// GetDownloadURL 获取下载URL
	GetDownloadURL(ctx context.Context, id uint64) (string, error)
	// DeleteFile 删除文件
	DeleteFile(ctx context.Context, userID, fileID uint64) error
}

// CreateUploadInput 创建上传输入
type CreateUploadInput struct {
	UserID      uint64
	Filename    string
	ContentType string
	SizeBytes   int64
	Kind        entity.FileKind
}

// CompleteUploadInput 完成上传输入
type CompleteUploadInput struct {
	UserID         uint64
	ObjectKey      string
	ConversationID uint64
	ClientMsgID    string
	Width          int32
	Height         int32
	Duration       int32
	Thumbnail      string
}
