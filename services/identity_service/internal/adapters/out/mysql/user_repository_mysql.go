package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
)

// UserModel GORM模型
type UserModel struct {
	ID           uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	Username     string     `gorm:"column:username;type:varchar(32);not null;uniqueIndex"`
	PasswordHash string     `gorm:"column:password_hash;type:varchar(255);not null"`
	Phone        *string    `gorm:"column:phone;type:varchar(20);uniqueIndex"`
	Email        *string    `gorm:"column:email;type:varchar(100);uniqueIndex"`
	DisplayName  string     `gorm:"column:display_name;type:varchar(64);not null"`
	AvatarURL    *string    `gorm:"column:avatar_url;type:varchar(512)"`
	Status       int8       `gorm:"column:status;type:tinyint;not null;default:1"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	DeletedAt    *time.Time `gorm:"column:deleted_at;index"`
}

func (UserModel) TableName() string {
	return "users"
}

// toEntity 转换为领域实体
func (m *UserModel) toEntity() *entity.User {
	return &entity.User{
		ID:           m.ID,
		Username:     m.Username,
		PasswordHash: m.PasswordHash,
		Phone:        m.Phone,
		Email:        m.Email,
		DisplayName:  m.DisplayName,
		AvatarURL:    m.AvatarURL,
		Status:       entity.UserStatus(m.Status),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		DeletedAt:    m.DeletedAt,
	}
}

// fromEntity 从领域实体转换
func userModelFromEntity(e *entity.User) *UserModel {
	return &UserModel{
		ID:           e.ID,
		Username:     e.Username,
		PasswordHash: e.PasswordHash,
		Phone:        e.Phone,
		Email:        e.Email,
		DisplayName:  e.DisplayName,
		AvatarURL:    e.AvatarURL,
		Status:       int8(e.Status),
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
		DeletedAt:    e.DeletedAt,
	}
}

// UserRepositoryMySQL MySQL用户仓储实现
type UserRepositoryMySQL struct {
	db *gorm.DB
}

// NewUserRepositoryMySQL 创建MySQL用户仓储
func NewUserRepositoryMySQL(db *gorm.DB) out.UserRepository {
	return &UserRepositoryMySQL{db: db}
}

func (r *UserRepositoryMySQL) Create(ctx context.Context, user *entity.User) error {
	model := userModelFromEntity(user)
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	user.ID = model.ID
	user.CreatedAt = model.CreatedAt
	user.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *UserRepositoryMySQL) GetByID(ctx context.Context, id uint64) (*entity.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *UserRepositoryMySQL) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("username = ? AND deleted_at IS NULL", username).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *UserRepositoryMySQL) GetByPhone(ctx context.Context, phone string) (*entity.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("phone = ? AND deleted_at IS NULL", phone).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *UserRepositoryMySQL) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	var model UserModel
	err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *UserRepositoryMySQL) Update(ctx context.Context, user *entity.User) error {
	model := userModelFromEntity(user)
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *UserRepositoryMySQL) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&UserModel{}).
		Where("username = ? AND deleted_at IS NULL", username).
		Count(&count).Error
	return count > 0, err
}

func (r *UserRepositoryMySQL) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&UserModel{}).
		Where("phone = ? AND deleted_at IS NULL", phone).
		Count(&count).Error
	return count > 0, err
}

func (r *UserRepositoryMySQL) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&UserModel{}).
		Where("email = ? AND deleted_at IS NULL", email).
		Count(&count).Error
	return count > 0, err
}
