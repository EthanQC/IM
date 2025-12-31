package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
)

// ContactApplyModel GORM模型
type ContactApplyModel struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	FromUserID uint64    `gorm:"column:from_user_id;not null;index"`
	ToUserID   uint64    `gorm:"column:to_user_id;not null;index"`
	Message    *string   `gorm:"column:message;type:varchar(255)"`
	Status     int8      `gorm:"column:status;type:tinyint;not null;default:0"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (ContactApplyModel) TableName() string {
	return "contact_applies"
}

func (m *ContactApplyModel) toEntity() *entity.ContactApply {
	return &entity.ContactApply{
		ID:         m.ID,
		FromUserID: m.FromUserID,
		ToUserID:   m.ToUserID,
		Message:    m.Message,
		Status:     entity.ApplyStatus(m.Status),
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

func contactApplyModelFromEntity(e *entity.ContactApply) *ContactApplyModel {
	return &ContactApplyModel{
		ID:         e.ID,
		FromUserID: e.FromUserID,
		ToUserID:   e.ToUserID,
		Message:    e.Message,
		Status:     int8(e.Status),
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}

// ContactApplyRepositoryMySQL MySQL好友申请仓储实现
type ContactApplyRepositoryMySQL struct {
	db *gorm.DB
}

func NewContactApplyRepositoryMySQL(db *gorm.DB) out.ContactApplyRepository {
	return &ContactApplyRepositoryMySQL{db: db}
}

func (r *ContactApplyRepositoryMySQL) Create(ctx context.Context, apply *entity.ContactApply) error {
	model := contactApplyModelFromEntity(apply)
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	apply.ID = model.ID
	apply.CreatedAt = model.CreatedAt
	apply.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *ContactApplyRepositoryMySQL) GetByID(ctx context.Context, id uint64) (*entity.ContactApply, error) {
	var model ContactApplyModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *ContactApplyRepositoryMySQL) GetPendingApply(ctx context.Context, fromUserID, toUserID uint64) (*entity.ContactApply, error) {
	var model ContactApplyModel
	err := r.db.WithContext(ctx).
		Where("from_user_id = ? AND to_user_id = ? AND status = ?", fromUserID, toUserID, entity.ApplyStatusPending).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *ContactApplyRepositoryMySQL) ListReceived(ctx context.Context, userID uint64, status entity.ApplyStatus, page, pageSize int) ([]*entity.ContactApply, int, error) {
	var models []ContactApplyModel
	var total int64

	query := r.db.WithContext(ctx).Model(&ContactApplyModel{}).Where("to_user_id = ?", userID)
	if status >= 0 {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, 0, err
	}

	applies := make([]*entity.ContactApply, len(models))
	for i, m := range models {
		applies[i] = m.toEntity()
	}

	return applies, int(total), nil
}

func (r *ContactApplyRepositoryMySQL) ListSent(ctx context.Context, userID uint64, status entity.ApplyStatus, page, pageSize int) ([]*entity.ContactApply, int, error) {
	var models []ContactApplyModel
	var total int64

	query := r.db.WithContext(ctx).Model(&ContactApplyModel{}).Where("from_user_id = ?", userID)
	if status >= 0 {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, 0, err
	}

	applies := make([]*entity.ContactApply, len(models))
	for i, m := range models {
		applies[i] = m.toEntity()
	}

	return applies, int(total), nil
}

func (r *ContactApplyRepositoryMySQL) Update(ctx context.Context, apply *entity.ContactApply) error {
	model := contactApplyModelFromEntity(apply)
	return r.db.WithContext(ctx).Save(model).Error
}
