package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

// PendingMessageModel 待投递消息数据库模型
type PendingMessageModel struct {
	ID             uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID         uint64     `gorm:"column:user_id;not null;index"`
	MessageID      uint64     `gorm:"column:message_id;not null"`
	ConversationID uint64     `gorm:"column:conversation_id;not null"`
	Payload        string     `gorm:"column:payload;type:text;not null"`
	Status         int8       `gorm:"column:status;default:0"`
	RetryCount     int        `gorm:"column:retry_count;default:0"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	DeliveredAt    *time.Time `gorm:"column:delivered_at"`
}

func (PendingMessageModel) TableName() string {
	return "pending_messages"
}

func (m *PendingMessageModel) toEntity() *entity.PendingMessage {
	return &entity.PendingMessage{
		ID:             m.ID,
		UserID:         m.UserID,
		MessageID:      m.MessageID,
		ConversationID: m.ConversationID,
		Payload:        m.Payload,
		Status:         entity.DeliveryStatus(m.Status),
		RetryCount:     m.RetryCount,
		CreatedAt:      m.CreatedAt,
		DeliveredAt:    m.DeliveredAt,
	}
}

// PendingMessageRepositoryMySQL MySQL实现
type PendingMessageRepositoryMySQL struct {
	db *gorm.DB
}

func NewPendingMessageRepositoryMySQL(db *gorm.DB) out.PendingMessageRepository {
	return &PendingMessageRepositoryMySQL{db: db}
}

func (r *PendingMessageRepositoryMySQL) Save(ctx context.Context, msg *entity.PendingMessage) error {
	model := &PendingMessageModel{
		UserID:         msg.UserID,
		MessageID:      msg.MessageID,
		ConversationID: msg.ConversationID,
		Payload:        msg.Payload,
		Status:         int8(msg.Status),
		RetryCount:     msg.RetryCount,
		CreatedAt:      msg.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	msg.ID = model.ID
	return nil
}

func (r *PendingMessageRepositoryMySQL) GetPending(ctx context.Context, userID uint64, limit int) ([]*entity.PendingMessage, error) {
	var models []PendingMessageModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, entity.DeliveryStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	result := make([]*entity.PendingMessage, len(models))
	for i, m := range models {
		result[i] = m.toEntity()
	}
	return result, nil
}

func (r *PendingMessageRepositoryMySQL) MarkDelivered(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&PendingMessageModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       entity.DeliveryStatusDelivered,
			"delivered_at": now,
		}).Error
}

func (r *PendingMessageRepositoryMySQL) MarkFailed(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).
		Model(&PendingMessageModel{}).
		Where("id = ?", id).
		Update("status", entity.DeliveryStatusFailed).Error
}

func (r *PendingMessageRepositoryMySQL) IncrRetryCount(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).
		Model(&PendingMessageModel{}).
		Where("id = ?", id).
		Update("retry_count", gorm.Expr("retry_count + 1")).Error
}

func (r *PendingMessageRepositoryMySQL) DeleteExpired(ctx context.Context, beforeTimestamp int64) (int64, error) {
	before := time.Unix(beforeTimestamp, 0)
	result := r.db.WithContext(ctx).
		Where("created_at < ? AND status IN ?", before, []int8{
			int8(entity.DeliveryStatusDelivered),
			int8(entity.DeliveryStatusFailed),
			int8(entity.DeliveryStatusExpired),
		}).
		Delete(&PendingMessageModel{})
	return result.RowsAffected, result.Error
}
