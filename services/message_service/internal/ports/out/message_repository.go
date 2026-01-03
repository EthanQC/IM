package out

import (
	"context"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
)

// MessageRepository 消息仓储接口
type MessageRepository interface {
	// Create 创建消息
	Create(ctx context.Context, msg *entity.Message) error

	// GetByID 根据ID获取消息
	GetByID(ctx context.Context, id uint64) (*entity.Message, error)

	// GetByClientMsgID 根据客户端消息ID获取（用于幂等判断）
	GetByClientMsgID(ctx context.Context, senderID uint64, clientMsgID string) (*entity.Message, error)

	// GetHistoryAfter 获取指定序号之后的消息
	GetHistoryAfter(ctx context.Context, conversationID uint64, afterSeq uint64, limit int) ([]*entity.Message, error)

	// GetHistoryBefore 获取指定序号之前的消息
	GetHistoryBefore(ctx context.Context, conversationID uint64, beforeSeq uint64, limit int) ([]*entity.Message, error)

	// Update 更新消息
	Update(ctx context.Context, msg *entity.Message) error

	// GetLatestSeq 获取会话最新消息序号
	GetLatestSeq(ctx context.Context, conversationID uint64) (uint64, error)
}

// SequenceRepository 序列号仓储接口
type SequenceRepository interface {
	// GetNextSeq 获取并递增下一个序号（原子操作）
	GetNextSeq(ctx context.Context, conversationID uint64) (uint64, error)

	// GetCurrentSeq 获取当前序号
	GetCurrentSeq(ctx context.Context, conversationID uint64) (uint64, error)
}

// InboxRepository 收件箱仓储接口
type InboxRepository interface {
	// GetOrCreate 获取或创建收件箱记录
	GetOrCreate(ctx context.Context, userID, conversationID uint64) (*Inbox, error)

	// UpdateLastRead 更新已读位置
	UpdateLastRead(ctx context.Context, userID, conversationID, readSeq uint64) error

	// UpdateLastDelivered 更新投递位置
	UpdateLastDelivered(ctx context.Context, userID, conversationID, deliveredSeq uint64) error

	// IncrUnread 增加未读数
	IncrUnread(ctx context.Context, userID, conversationID uint64, delta int) error

	// ClearUnread 清除未读数
	ClearUnread(ctx context.Context, userID, conversationID uint64) error

	// GetUnreadCount 获取未读数
	GetUnreadCount(ctx context.Context, userID, conversationID uint64) (int, error)

	// GetTotalUnread 获取用户总未读数
	GetTotalUnread(ctx context.Context, userID uint64) (int, error)

	// GetUserInboxes 获取用户的所有收件箱
	GetUserInboxes(ctx context.Context, userID uint64) ([]*entity.Inbox, error)
}

// TimelineRepository 消息时间线仓储接口（Redis热数据缓存）
type TimelineRepository interface {
	// AddMessage 添加消息到时间线
	AddMessage(ctx context.Context, conversationID uint64, msg *entity.Message) error

	// GetMessagesAfterSeq 获取指定序号之后的消息
	GetMessagesAfterSeq(ctx context.Context, conversationID uint64, afterSeq uint64, limit int) ([]*entity.Message, error)

	// GetMessagesBeforeSeq 获取指定序号之前的消息
	GetMessagesBeforeSeq(ctx context.Context, conversationID uint64, beforeSeq uint64, limit int) ([]*entity.Message, error)

	// GetLatestSeq 获取最新序号
	GetLatestSeq(ctx context.Context, conversationID uint64) (uint64, error)

	// RemoveMessage 移除消息
	RemoveMessage(ctx context.Context, conversationID uint64, seq uint64) error
}

// Inbox 收件箱
type Inbox struct {
	UserID           uint64
	ConversationID   uint64
	LastReadSeq      uint64
	LastDeliveredSeq uint64
	UnreadCount      int
	IsMuted          bool
	IsPinned         bool
}
