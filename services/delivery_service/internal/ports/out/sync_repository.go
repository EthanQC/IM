package out

import (
	"context"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
)

// SyncStateRepository 同步状态仓储接口
type SyncStateRepository interface {
	// GetSyncState 获取同步状态
	GetSyncState(ctx context.Context, userID uint64) (*entity.SyncState, error)
	// UpdateAckSeq 更新ACK序号
	UpdateAckSeq(ctx context.Context, userID, conversationID, ackSeq uint64) error
	// UpdateLastSyncTime 更新最后同步时间
	UpdateLastSyncTime(ctx context.Context, userID uint64, syncTime int64) error
	// BatchUpdateAckSeqs 批量更新ACK序号
	BatchUpdateAckSeqs(ctx context.Context, userID uint64, ackSeqs map[uint64]uint64) error
}

// MessageQueryRepository 消息查询仓储接口（用于同步）
type MessageQueryRepository interface {
	// GetMessagesAfterSeq 获取指定序号之后的消息
	GetMessagesAfterSeq(ctx context.Context, conversationID uint64, afterSeq uint64, limit int) ([]*entity.MessageInfo, error)
	// GetLatestSeq 获取最新序号
	GetLatestSeq(ctx context.Context, conversationID uint64) (uint64, error)
	// GetMessagesByIDs 批量获取消息
	GetMessagesByIDs(ctx context.Context, messageIDs []uint64) ([]*entity.MessageInfo, error)
}

// InboxQueryRepository 收件箱查询仓储接口
type InboxQueryRepository interface {
	// GetUserConversationIDs 获取用户的所有会话ID
	GetUserConversationIDs(ctx context.Context, userID uint64) ([]uint64, error)
	// GetUserInboxes 获取用户的所有收件箱
	GetUserInboxes(ctx context.Context, userID uint64) ([]*entity.InboxInfo, error)
	// UpdateLastRead 更新已读位置
	UpdateLastRead(ctx context.Context, userID, conversationID, readSeq uint64) error
	// GetTotalUnread 获取总未读数
	GetTotalUnread(ctx context.Context, userID uint64) (int, error)
}

// PendingAckRepository 待确认消息仓储接口
type PendingAckRepository interface {
	// Save 保存待确认记录
	Save(ctx context.Context, item *entity.PendingAckItem) error
	// Remove 移除待确认记录
	Remove(ctx context.Context, userID, messageID uint64) error
	// BatchRemove 批量移除
	BatchRemove(ctx context.Context, userID uint64, messageIDs []uint64) error
	// GetPending 获取待确认列表
	GetPending(ctx context.Context, userID uint64) ([]*entity.PendingAckItem, error)
	// IncrRetry 增加重试次数
	IncrRetry(ctx context.Context, userID, messageID uint64) error
	// MarkFailed 标记为失败
	MarkFailed(ctx context.Context, userID, messageID uint64) error
}
