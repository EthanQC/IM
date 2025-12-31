package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/file_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/file_service/internal/ports/out"
)

// FileModel 文件数据库模型 - 匹配 attachments 表
type FileModel struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	ObjectKey    string    `gorm:"column:object_key;type:varchar(255);not null;uniqueIndex"`
	OriginalName string    `gorm:"column:original_name;type:varchar(255);not null"`
	UploaderID   uint64    `gorm:"column:uploader_id;not null;index"`
	SizeBytes    int64     `gorm:"column:size_bytes;not null"`
	ContentType  string    `gorm:"column:content_type;type:varchar(128);not null"`
	MD5          string    `gorm:"column:md5;type:char(32)"`
	Width        int32     `gorm:"column:width"`
	Height       int32     `gorm:"column:height"`
	Duration     int32     `gorm:"column:duration"`
	Metadata     *string   `gorm:"column:metadata;type:json"`
	Status       int8      `gorm:"column:status;default:1"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (FileModel) TableName() string {
	return "attachments"
}

func (m *FileModel) toEntity() *entity.FileUpload {
	return &entity.FileUpload{
		ID:          m.ID,
		UserID:      m.UploaderID,
		ObjectKey:   m.ObjectKey,
		FileName:    m.OriginalName,
		ContentType: m.ContentType,
		SizeBytes:   m.SizeBytes,
		Status:      entity.FileStatus(m.Status),
		Width:       m.Width,
		Height:      m.Height,
		Duration:    m.Duration,
		CreatedAt:   m.CreatedAt,
	}
}

// FileRepositoryMySQL MySQL文件仓储实现
type FileRepositoryMySQL struct {
	db *gorm.DB
}

func NewFileRepositoryMySQL(db *gorm.DB) out.FileRepository {
	return &FileRepositoryMySQL{db: db}
}

func (r *FileRepositoryMySQL) Create(ctx context.Context, file *entity.FileUpload) error {
	model := &FileModel{
		ObjectKey:    file.ObjectKey,
		OriginalName: file.FileName,
		UploaderID:   file.UserID,
		SizeBytes:    file.SizeBytes,
		ContentType:  file.ContentType,
		Status:       int8(file.Status),
		Width:        file.Width,
		Height:       file.Height,
		Duration:     file.Duration,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}

	file.ID = model.ID
	file.CreatedAt = model.CreatedAt
	return nil
}

func (r *FileRepositoryMySQL) GetByID(ctx context.Context, id uint64) (*entity.FileUpload, error) {
	var model FileModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *FileRepositoryMySQL) GetByObjectKey(ctx context.Context, objectKey string) (*entity.FileUpload, error) {
	var model FileModel
	err := r.db.WithContext(ctx).Where("object_key = ?", objectKey).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *FileRepositoryMySQL) UpdateStatus(ctx context.Context, id uint64, status entity.FileStatus) error {
	return r.db.WithContext(ctx).
		Model(&FileModel{}).
		Where("id = ?", id).
		Update("status", int8(status)).Error
}

func (r *FileRepositoryMySQL) Update(ctx context.Context, file *entity.FileUpload) error {
	return r.db.WithContext(ctx).
		Model(&FileModel{}).
		Where("id = ?", file.ID).
		Updates(map[string]interface{}{
			"status":    int8(file.Status),
			"url":       file.URL,
			"thumbnail": file.Thumbnail,
			"width":     file.Width,
			"height":    file.Height,
			"duration":  file.Duration,
		}).Error
}
