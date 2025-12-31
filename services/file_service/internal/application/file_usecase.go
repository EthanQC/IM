package application

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/EthanQC/IM/services/file_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/file_service/internal/ports/in"
	"github.com/EthanQC/IM/services/file_service/internal/ports/out"
)

var (
	ErrFileNotFound    = errors.New("file not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrFileTooLarge    = errors.New("file too large")
	ErrInvalidFileType = errors.New("invalid file type")
)

// 文件大小限制
const (
	MaxImageSize = 10 * 1024 * 1024   // 10MB
	MaxFileSize  = 100 * 1024 * 1024  // 100MB
	MaxAudioSize = 50 * 1024 * 1024   // 50MB
	MaxVideoSize = 500 * 1024 * 1024  // 500MB
)

// FileUseCaseImpl 文件用例实现
type FileUseCaseImpl struct {
	fileRepo      out.FileRepository
	objectStorage out.ObjectStorage
	bucket        string
	callbackURL   string
	presignExpiry time.Duration
}

// NewFileUseCase 创建文件用例
func NewFileUseCase(
	fileRepo out.FileRepository,
	objectStorage out.ObjectStorage,
	bucket string,
	callbackURL string,
) in.FileUseCase {
	return &FileUseCaseImpl{
		fileRepo:      fileRepo,
		objectStorage: objectStorage,
		bucket:        bucket,
		callbackURL:   callbackURL,
		presignExpiry: 30 * time.Minute,
	}
}

// CreateUpload 创建上传
func (uc *FileUseCaseImpl) CreateUpload(ctx context.Context, input *in.CreateUploadInput) (*entity.UploadToken, error) {
	// 验证文件大小
	if err := uc.validateFileSize(input.Kind, input.SizeBytes); err != nil {
		return nil, err
	}

	// 生成ObjectKey
	objectKey := uc.generateObjectKey(input.UserID, input.Kind, input.Filename)

	// 创建文件记录
	file := &entity.FileUpload{
		UserID:      input.UserID,
		ObjectKey:   objectKey,
		FileName:    input.Filename,
		ContentType: input.ContentType,
		SizeBytes:   input.SizeBytes,
		Kind:        input.Kind,
		Status:      entity.FileStatusPending,
		Bucket:      uc.bucket,
	}

	if err := uc.fileRepo.Create(ctx, file); err != nil {
		return nil, err
	}

	// 生成预签名URL
	uploadURL, err := uc.objectStorage.GeneratePresignedPutURL(
		ctx, uc.bucket, objectKey, input.ContentType, uc.presignExpiry,
	)
	if err != nil {
		return nil, err
	}

	return &entity.UploadToken{
		ObjectKey:   objectKey,
		UploadURL:   uploadURL,
		CallbackURL: uc.callbackURL,
		ExpiredAt:   time.Now().Add(uc.presignExpiry),
	}, nil
}

// CompleteUpload 完成上传
func (uc *FileUseCaseImpl) CompleteUpload(ctx context.Context, input *in.CompleteUploadInput) (*entity.FileUpload, error) {
	// 获取文件记录
	file, err := uc.fileRepo.GetByObjectKey(ctx, input.ObjectKey)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, ErrFileNotFound
	}

	// 验证用户权限
	if file.UserID != input.UserID {
		return nil, ErrUnauthorized
	}

	// 验证文件是否已上传到对象存储
	exists, err := uc.objectStorage.Exists(ctx, file.Bucket, file.ObjectKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrFileNotFound
	}

	// 更新文件信息
	file.Status = entity.FileStatusConfirmed
	file.Width = input.Width
	file.Height = input.Height
	file.Duration = input.Duration
	file.Thumbnail = input.Thumbnail

	// 生成下载URL
	downloadURL, err := uc.objectStorage.GeneratePresignedGetURL(
		ctx, file.Bucket, file.ObjectKey, 24*time.Hour,
	)
	if err != nil {
		return nil, err
	}
	file.URL = downloadURL

	if err := uc.fileRepo.Update(ctx, file); err != nil {
		return nil, err
	}

	return file, nil
}

// GetFile 获取文件
func (uc *FileUseCaseImpl) GetFile(ctx context.Context, id uint64) (*entity.FileUpload, error) {
	return uc.fileRepo.GetByID(ctx, id)
}

// GetDownloadURL 获取下载URL
func (uc *FileUseCaseImpl) GetDownloadURL(ctx context.Context, id uint64) (string, error) {
	file, err := uc.fileRepo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if file == nil {
		return "", ErrFileNotFound
	}

	return uc.objectStorage.GeneratePresignedGetURL(
		ctx, file.Bucket, file.ObjectKey, time.Hour,
	)
}

// DeleteFile 删除文件
func (uc *FileUseCaseImpl) DeleteFile(ctx context.Context, userID, fileID uint64) error {
	file, err := uc.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	if file == nil {
		return ErrFileNotFound
	}

	// 验证权限
	if file.UserID != userID {
		return ErrUnauthorized
	}

	// 从对象存储删除
	if err := uc.objectStorage.Delete(ctx, file.Bucket, file.ObjectKey); err != nil {
		return err
	}

	// 更新状态
	return uc.fileRepo.UpdateStatus(ctx, fileID, entity.FileStatusDeleted)
}

// validateFileSize 验证文件大小
func (uc *FileUseCaseImpl) validateFileSize(kind entity.FileKind, size int64) error {
	var maxSize int64
	switch kind {
	case entity.FileKindImage:
		maxSize = MaxImageSize
	case entity.FileKindFile:
		maxSize = MaxFileSize
	case entity.FileKindAudio:
		maxSize = MaxAudioSize
	case entity.FileKindVideo:
		maxSize = MaxVideoSize
	default:
		maxSize = MaxFileSize
	}

	if size > maxSize {
		return ErrFileTooLarge
	}
	return nil
}

// generateObjectKey 生成ObjectKey
func (uc *FileUseCaseImpl) generateObjectKey(userID uint64, kind entity.FileKind, filename string) string {
	ext := filepath.Ext(filename)
	dateStr := time.Now().Format("2006/01/02")
	uniqueID := uuid.New().String()
	return fmt.Sprintf("%s/%d/%s%s", dateStr, userID, uniqueID, ext)
}
