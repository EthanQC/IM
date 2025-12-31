package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
)

// ContactModel GORM模型
type ContactModel struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"column:user_id;not null;index"`
	FriendID  uint64    `gorm:"column:friend_id;not null;index"`
	Remark    *string   `gorm:"column:remark;type:varchar(64)"`
	Status    int8      `gorm:"column:status;type:tinyint;not null;default:1"`
	Type      int8      `gorm:"column:type;type:tinyint;not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (ContactModel) TableName() string {
	return "contacts"
}

func (m *ContactModel) toEntity() *entity.Contact {
	return &entity.Contact{
		ID:        m.ID,
		UserID:    m.UserID,
		FriendID:  m.FriendID,
		Remark:    m.Remark,
		Status:    entity.ContactStatus(m.Status),
		Type:      entity.ContactType(m.Type),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func contactModelFromEntity(e *entity.Contact) *ContactModel {
	return &ContactModel{
		ID:        e.ID,
		UserID:    e.UserID,
		FriendID:  e.FriendID,
		Remark:    e.Remark,
		Status:    int8(e.Status),
		Type:      int8(e.Type),
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

// ContactRepositoryMySQL MySQL联系人仓储实现
type ContactRepositoryMySQL struct {
	db *gorm.DB
}

func NewContactRepositoryMySQL(db *gorm.DB) out.ContactRepository {
	return &ContactRepositoryMySQL{db: db}
}

func (r *ContactRepositoryMySQL) Create(ctx context.Context, contact *entity.Contact) error {
	model := contactModelFromEntity(contact)
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	contact.ID = model.ID
	contact.CreatedAt = model.CreatedAt
	contact.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *ContactRepositoryMySQL) GetByID(ctx context.Context, id uint64) (*entity.Contact, error) {
	var model ContactModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *ContactRepositoryMySQL) GetContact(ctx context.Context, userID, friendID uint64) (*entity.Contact, error) {
	var model ContactModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND friend_id = ? AND status != ?", userID, friendID, entity.ContactStatusDeleted).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *ContactRepositoryMySQL) List(ctx context.Context, userID uint64, status entity.ContactStatus, page, pageSize int) ([]*entity.Contact, int, error) {
	var models []ContactModel
	var total int64

	query := r.db.WithContext(ctx).Model(&ContactModel{}).Where("user_id = ? AND status = ?", userID, status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&models).Error; err != nil {
		return nil, 0, err
	}

	contacts := make([]*entity.Contact, len(models))
	for i, m := range models {
		contacts[i] = m.toEntity()
	}

	return contacts, int(total), nil
}

func (r *ContactRepositoryMySQL) Update(ctx context.Context, contact *entity.Contact) error {
	model := contactModelFromEntity(contact)
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *ContactRepositoryMySQL) Delete(ctx context.Context, userID, friendID uint64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND friend_id = ?", userID, friendID).
		Delete(&ContactModel{}).Error
}

func (r *ContactRepositoryMySQL) IsContact(ctx context.Context, userID, friendID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ContactModel{}).
		Where("user_id = ? AND friend_id = ? AND status = ?", userID, friendID, entity.ContactStatusNormal).
		Count(&count).Error
	return count > 0, err
}
