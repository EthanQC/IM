package in

import (
	"context"
)

// SyncUseCase 消息同步用例接口（推拉结合）
type SyncUseCase interface {
	// SyncMessages 同步消息（增量拉取）
	// 基于 lastAckSeq 获取之后的所有未读消息
	SyncMessages(ctx context.Context, req *SyncRequest) (*SyncResponse, error)

	// AckMessages 确认消息已收到（客户端ACK）
	AckMessages(ctx context.Context, userID, conversationID, ackSeq uint64) error

	// GetUnreadConversations 获取有未读消息的会话列表
	GetUnreadConversations(ctx context.Context, userID uint64) ([]*UnreadConversation, error)

	// GetSyncState 获取同步状态
	GetSyncState(ctx context.Context, userID uint64) (*SyncState, error)
}

// SyncRequest 同步请求
type SyncRequest struct {
	UserID uint64 `json:"user_id"`
	// 会话级同步点: conversationID -> lastAckSeq
	SyncPoints map[uint64]uint64 `json:"sync_points"`
	// 每个会话最多拉取条数
	Limit int `json:"limit"`
}

// SyncResponse 同步响应
type SyncResponse struct {
	// 会话消息列表
	Messages map[uint64][]*SyncMessage `json:"messages"`
	// 是否还有更多消息
	HasMore map[uint64]bool `json:"has_more"`
	// 最新序号
	LatestSeqs map[uint64]uint64 `json:"latest_seqs"`
}

// SyncMessage 同步消息
type SyncMessage struct {
	ID             uint64 `json:"id"`
	ConversationID uint64 `json:"conversation_id"`
	SenderID       uint64 `json:"sender_id"`
	Seq            uint64 `json:"seq"`
	ContentType    int8   `json:"content_type"`
	Content        string `json:"content"`
	Status         int8   `json:"status"`
	CreatedAt      int64  `json:"created_at"`
}

// UnreadConversation 未读会话
type UnreadConversation struct {
	ConversationID uint64 `json:"conversation_id"`
	UnreadCount    int    `json:"unread_count"`
	LastMsgSeq     uint64 `json:"last_msg_seq"`
	LastMsgTime    int64  `json:"last_msg_time"`
	LastAckSeq     uint64 `json:"last_ack_seq"`
}

// SyncState 同步状态
type SyncState struct {
	UserID uint64 `json:"user_id"`
	// 会话级ACK序号
	ConversationAckSeqs map[uint64]uint64 `json:"conversation_ack_seqs"`
	// 总未读数
	TotalUnread int `json:"total_unread"`
	// 最后同步时间
	LastSyncAt int64 `json:"last_sync_at"`
}

// AckUseCase ACK确认用例接口
type AckUseCase interface {
	// MessageAck 消息ACK（单条）
	MessageAck(ctx context.Context, userID, conversationID, messageID, seq uint64) error

	// BatchMessageAck 批量消息ACK
	BatchMessageAck(ctx context.Context, userID uint64, acks []*MessageAckItem) error

	// GetPendingAcks 获取待确认的消息
	GetPendingAcks(ctx context.Context, userID uint64) ([]*PendingAck, error)

	// ResendUnacked 重发未确认的消息
	ResendUnacked(ctx context.Context, userID uint64) error
}

// MessageAckItem 消息ACK项
type MessageAckItem struct {
	ConversationID uint64 `json:"conversation_id"`
	MessageID      uint64 `json:"message_id"`
	Seq            uint64 `json:"seq"`
}

// PendingAck 待确认消息
type PendingAck struct {
	MessageID      uint64 `json:"message_id"`
	ConversationID uint64 `json:"conversation_id"`
	Seq            uint64 `json:"seq"`
	SentAt         int64  `json:"sent_at"`
	RetryCount     int    `json:"retry_count"`
}
