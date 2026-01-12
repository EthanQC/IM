package out

import (
	"context"
	"time"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
)

// OutboxStatus 发件箱状态
type OutboxStatus int8

const (
	OutboxStatusPending   OutboxStatus = 0 // 待发布
	OutboxStatusPublished OutboxStatus = 1 // 已发布
	OutboxStatusFailed    OutboxStatus = 2 // 失败
)

// OutboxEvent 发件箱事件
type OutboxEvent struct {
	ID             uint64       `json:"id"`
	EventType      string       `json:"event_type"`
	AggregateID    string       `json:"aggregate_id"`
	ConversationID uint64       `json:"conversation_id"`
	MessageID      *uint64      `json:"message_id,omitempty"`
	Payload        []byte       `json:"payload"`
	Status         OutboxStatus `json:"status"`
	RetryCount     int          `json:"retry_count"`
	LastError      string       `json:"last_error,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	PublishedAt    *time.Time   `json:"published_at,omitempty"`
}

// OutboxRepository 事务发件箱仓储接口
type OutboxRepository interface {
	// Create 创建发件箱事件（在事务中调用）
	Create(ctx context.Context, event *OutboxEvent) error

	// CreateWithTx 在指定事务中创建发件箱事件
	CreateWithTx(ctx context.Context, tx interface{}, event *OutboxEvent) error

	// GetPendingEvents 获取待发布的事件
	GetPendingEvents(ctx context.Context, limit int) ([]*OutboxEvent, error)

	// GetFailedEvents 获取失败的事件（用于重试）
	GetFailedEvents(ctx context.Context, maxRetries int, limit int) ([]*OutboxEvent, error)

	// MarkAsPublished 标记为已发布
	MarkAsPublished(ctx context.Context, id uint64) error

	// MarkAsFailed 标记为失败
	MarkAsFailed(ctx context.Context, id uint64, errMsg string) error

	// IncrRetryCount 增加重试次数
	IncrRetryCount(ctx context.Context, id uint64) error

	// DeletePublished 删除已发布的事件（清理）
	DeletePublished(ctx context.Context, before time.Time) (int64, error)

	// GetEventsByMessageID 根据消息ID获取事件
	GetEventsByMessageID(ctx context.Context, messageID uint64) ([]*OutboxEvent, error)
}

// TransactionalOutbox 事务性发件箱操作接口
type TransactionalOutbox interface {
	// SaveMessageAndEvent 在同一事务中保存消息和发件箱事件
	SaveMessageAndEvent(ctx context.Context, msg *entity.Message, event *OutboxEvent) error

	// SaveMessageAndEvents 在同一事务中保存消息和多个发件箱事件
	SaveMessageAndEvents(ctx context.Context, msg *entity.Message, events []*OutboxEvent) error
}
