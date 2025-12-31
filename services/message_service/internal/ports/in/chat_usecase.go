package in

import (
	"context"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
)

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	ConversationID uint64
	SenderID       uint64
	ClientMsgID    string
	ContentType    entity.MessageContentType
	Content        entity.MessageContent
	ReplyToMsgID   *uint64
}

// MessageUseCase 消息用例接口
type MessageUseCase interface {
	// SendMessage 发送消息
	SendMessage(ctx context.Context, req *SendMessageRequest) (*entity.Message, error)

	// GetMessage 获取消息
	GetMessage(ctx context.Context, messageID uint64) (*entity.Message, error)

	// GetHistory 获取历史消息
	GetHistory(ctx context.Context, conversationID uint64, afterSeq uint64, limit int) ([]*entity.Message, error)

	// GetHistoryBefore 获取指定序号之前的消息
	GetHistoryBefore(ctx context.Context, conversationID uint64, beforeSeq uint64, limit int) ([]*entity.Message, error)

	// UpdateRead 更新已读位置
	UpdateRead(ctx context.Context, userID, conversationID, readSeq uint64) error

	// RevokeMessage 撤回消息
	RevokeMessage(ctx context.Context, userID, messageID uint64) error

	// DeleteMessage 删除消息（仅对自己）
	DeleteMessage(ctx context.Context, userID, messageID uint64) error

	// GetUnreadCount 获取未读数
	GetUnreadCount(ctx context.Context, userID, conversationID uint64) (int, error)
}
