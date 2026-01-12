package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

// OutboxModel 发件箱GORM模型
type OutboxModel struct {
	ID             uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	EventType      string         `gorm:"column:event_type;type:varchar(64);not null"`
	AggregateID    string         `gorm:"column:aggregate_id;type:varchar(64);not null"`
	ConversationID uint64         `gorm:"column:conversation_id;not null;index"`
	MessageID      sql.NullInt64  `gorm:"column:message_id"`
	Payload        string         `gorm:"column:payload;type:json;not null"`
	Status         int8           `gorm:"column:status;default:0;index"`
	RetryCount     int            `gorm:"column:retry_count;default:0"`
	LastError      sql.NullString `gorm:"column:last_error;type:text"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime;index"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	PublishedAt    sql.NullTime   `gorm:"column:published_at"`
}

func (OutboxModel) TableName() string {
	return "outbox"
}

func (m *OutboxModel) toDTO() *out.OutboxEvent {
	event := &out.OutboxEvent{
		ID:             m.ID,
		EventType:      m.EventType,
		AggregateID:    m.AggregateID,
		ConversationID: m.ConversationID,
		Payload:        []byte(m.Payload),
		Status:         out.OutboxStatus(m.Status),
		RetryCount:     m.RetryCount,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}

	if m.MessageID.Valid {
		msgID := uint64(m.MessageID.Int64)
		event.MessageID = &msgID
	}

	if m.LastError.Valid {
		event.LastError = m.LastError.String
	}

	if m.PublishedAt.Valid {
		event.PublishedAt = &m.PublishedAt.Time
	}

	return event
}

func outboxModelFromDTO(e *out.OutboxEvent) *OutboxModel {
	model := &OutboxModel{
		ID:             e.ID,
		EventType:      e.EventType,
		AggregateID:    e.AggregateID,
		ConversationID: e.ConversationID,
		Payload:        string(e.Payload),
		Status:         int8(e.Status),
		RetryCount:     e.RetryCount,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}

	if e.MessageID != nil {
		model.MessageID = sql.NullInt64{Int64: int64(*e.MessageID), Valid: true}
	}

	if e.LastError != "" {
		model.LastError = sql.NullString{String: e.LastError, Valid: true}
	}

	if e.PublishedAt != nil {
		model.PublishedAt = sql.NullTime{Time: *e.PublishedAt, Valid: true}
	}

	return model
}

// OutboxRepositoryMySQL MySQL发件箱仓储实现
type OutboxRepositoryMySQL struct {
	db *gorm.DB
}

// NewOutboxRepositoryMySQL 创建MySQL发件箱仓储
func NewOutboxRepositoryMySQL(db *gorm.DB) out.OutboxRepository {
	return &OutboxRepositoryMySQL{db: db}
}

func (r *OutboxRepositoryMySQL) Create(ctx context.Context, event *out.OutboxEvent) error {
	model := outboxModelFromDTO(event)
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	event.ID = model.ID
	event.CreatedAt = model.CreatedAt
	event.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *OutboxRepositoryMySQL) CreateWithTx(ctx context.Context, tx interface{}, event *out.OutboxEvent) error {
	gormTx, ok := tx.(*gorm.DB)
	if !ok {
		return errors.New("invalid transaction type")
	}

	model := outboxModelFromDTO(event)
	if err := gormTx.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	event.ID = model.ID
	event.CreatedAt = model.CreatedAt
	event.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *OutboxRepositoryMySQL) GetPendingEvents(ctx context.Context, limit int) ([]*out.OutboxEvent, error) {
	var models []OutboxModel
	err := r.db.WithContext(ctx).
		Where("status = ?", out.OutboxStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	events := make([]*out.OutboxEvent, len(models))
	for i, m := range models {
		events[i] = m.toDTO()
	}
	return events, nil
}

func (r *OutboxRepositoryMySQL) GetFailedEvents(ctx context.Context, maxRetries int, limit int) ([]*out.OutboxEvent, error) {
	var models []OutboxModel
	err := r.db.WithContext(ctx).
		Where("status = ? AND retry_count < ?", out.OutboxStatusFailed, maxRetries).
		Order("created_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	events := make([]*out.OutboxEvent, len(models))
	for i, m := range models {
		events[i] = m.toDTO()
	}
	return events, nil
}

func (r *OutboxRepositoryMySQL) MarkAsPublished(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&OutboxModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       out.OutboxStatusPublished,
			"published_at": now,
		}).Error
}

func (r *OutboxRepositoryMySQL) MarkAsFailed(ctx context.Context, id uint64, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&OutboxModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     out.OutboxStatusFailed,
			"last_error": errMsg,
		}).Error
}

func (r *OutboxRepositoryMySQL) IncrRetryCount(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).
		Model(&OutboxModel{}).
		Where("id = ?", id).
		Update("retry_count", gorm.Expr("retry_count + 1")).Error
}

func (r *OutboxRepositoryMySQL) DeletePublished(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("status = ? AND published_at < ?", out.OutboxStatusPublished, before).
		Delete(&OutboxModel{})
	return result.RowsAffected, result.Error
}

func (r *OutboxRepositoryMySQL) GetEventsByMessageID(ctx context.Context, messageID uint64) ([]*out.OutboxEvent, error) {
	var models []OutboxModel
	err := r.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	events := make([]*out.OutboxEvent, len(models))
	for i, m := range models {
		events[i] = m.toDTO()
	}
	return events, nil
}

// TransactionalOutboxMySQL 事务性发件箱实现
type TransactionalOutboxMySQL struct {
	db *gorm.DB
}

// NewTransactionalOutboxMySQL 创建事务性发件箱
func NewTransactionalOutboxMySQL(db *gorm.DB) out.TransactionalOutbox {
	return &TransactionalOutboxMySQL{db: db}
}

func (r *TransactionalOutboxMySQL) SaveMessageAndEvent(ctx context.Context, msg *entity.Message, event *out.OutboxEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 保存消息
		msgModel := messageModelFromEntity(msg)
		if err := tx.Create(msgModel).Error; err != nil {
			return err
		}
		msg.ID = msgModel.ID
		msg.CreatedAt = msgModel.CreatedAt
		msg.UpdatedAt = msgModel.UpdatedAt

		// 更新事件中的消息ID
		event.MessageID = &msg.ID

		// 保存发件箱事件
		outboxModel := outboxModelFromDTO(event)
		if err := tx.Create(outboxModel).Error; err != nil {
			return err
		}
		event.ID = outboxModel.ID
		event.CreatedAt = outboxModel.CreatedAt
		event.UpdatedAt = outboxModel.UpdatedAt

		return nil
	})
}

func (r *TransactionalOutboxMySQL) SaveMessageAndEvents(ctx context.Context, msg *entity.Message, events []*out.OutboxEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 保存消息
		msgModel := messageModelFromEntity(msg)
		if err := tx.Create(msgModel).Error; err != nil {
			return err
		}
		msg.ID = msgModel.ID
		msg.CreatedAt = msgModel.CreatedAt
		msg.UpdatedAt = msgModel.UpdatedAt

		// 保存发件箱事件
		for _, event := range events {
			event.MessageID = &msg.ID
			outboxModel := outboxModelFromDTO(event)
			if err := tx.Create(outboxModel).Error; err != nil {
				return err
			}
			event.ID = outboxModel.ID
			event.CreatedAt = outboxModel.CreatedAt
			event.UpdatedAt = outboxModel.UpdatedAt
		}

		return nil
	})
}
