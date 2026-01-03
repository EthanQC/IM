package out

import "context"

// EventPublisher 事件发布器接口
type EventPublisher interface {
	// PublishMessageSent 发布消息发送事件
	PublishMessageSent(ctx context.Context, event *MessageSentEvent) error

	// PublishMessageRevoked 发布消息撤回事件
	PublishMessageRevoked(ctx context.Context, event *MessageRevokedEvent) error

	// PublishMessageRead 发布消息已读事件
	PublishMessageRead(ctx context.Context, event *MessageReadEvent) error
}

// MessageSentEvent 消息发送事件
type MessageSentEvent struct {
	MessageID      uint64 `json:"message_id"`
	ConversationID uint64 `json:"conversation_id"`
	SenderID       uint64 `json:"sender_id"`
	ReceiverIDs    []uint64 `json:"receiver_ids"`
	Seq            uint64 `json:"seq"`
	ContentType    int8   `json:"content_type"`
	Content        string `json:"content"`
	CreatedAt      int64  `json:"created_at"`
}

// MessageRevokedEvent 消息撤回事件
type MessageRevokedEvent struct {
	MessageID      uint64 `json:"message_id"`
	ConversationID uint64 `json:"conversation_id"`
	SenderID       uint64 `json:"sender_id"`
	ReceiverIDs    []uint64 `json:"receiver_ids"`
	RevokedAt      int64  `json:"revoked_at"`
}

// MessageReadEvent 消息已读事件
type MessageReadEvent struct {
	UserID         uint64 `json:"user_id"`
	ConversationID uint64 `json:"conversation_id"`
	ReceiverIDs    []uint64 `json:"receiver_ids"`
	ReadSeq        uint64 `json:"read_seq"`
	ReadAt         int64  `json:"read_at"`
}
