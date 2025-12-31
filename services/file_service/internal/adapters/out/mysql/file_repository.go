package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/file_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/file_service/internal/ports/out"
)

// FileModel 文件数据库模型
type FileModel struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID      uint64    `gorm:"column:user_id;not null;index"`
	ObjectKey   string    `gorm:"column:object_key;type:varchar(512);not null;uniqueIndex"`
	FileName    string    `gorm:"column:file_name;type:varchar(255);not null"`
	ContentType string    `gorm:"column:content_type;type:varchar(128)"`
	SizeBytes   int64     `gorm:"column:size_bytes;not null"`
	Kind        string    `gorm:"column:kind;type:varchar(32);not null"`
	Status      int8      `gorm:"column:status;default:0"`
	Bucket      string    `gorm:"column:bucket;type:varchar(64);not null"`
	URL         string    `gorm:"column:url;type:varchar(1024)"`
	Thumbnail   string    `gorm:"column:thumbnail;type:varchar(1024)"`
	Width       int32     `gorm:"column:width"`
	Height      int32     `gorm:"column:height"`
	Duration    int32     `gorm:"column:duration"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (FileModel) TableName() string {
	return "attachments"
}

func (m *FileModel) toEntity() *entity.FileUpload {
	return &entity.FileUpload{
		ID:          m.ID,
		UserID:      m.UserID,
		ObjectKey:   m.ObjectKey,
		FileName:    m.FileName,
		ContentType: m.ContentType,
		SizeBytes:   m.SizeBytes,
		Kind:        entity.FileKind(m.Kind),
		Status:      entity.FileStatus(m.Status),
		Bucket:      m.Bucket,
		URL:         m.URL,
		Thumbnail:   m.Thumbnail,
		Width:       m.Width,
		Height:      m.Height,
		Duration:    m.Duration,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
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
		UserID:      file.UserID,
		ObjectKey:   file.ObjectKey,
		FileName:    file.FileName,
		ContentType: file.ContentType,
		SizeBytes:   file.SizeBytes,
		Kind:        string(file.Kind),
		Status:      int8(file.Status),
		Bucket:      file.Bucket,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}

	file.ID = model.ID
	file.CreatedAt = model.CreatedAt
	file.UpdatedAt = model.UpdatedAt
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
