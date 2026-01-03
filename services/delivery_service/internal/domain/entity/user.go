package entity

import "time"

// OnlineUser 在线用户
type OnlineUser struct {
	UserID      uint64    `json:"user_id"`
	DeviceID    string    `json:"device_id"`
	Platform    string    `json:"platform"` // ios, android, web, desktop
	ServerAddr  string    `json:"server_addr"`
	ConnectedAt time.Time `json:"connected_at"`
	LastPingAt  time.Time `json:"last_ping_at"`
}

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeIOS     DeviceType = "ios"
	DeviceTypeAndroid DeviceType = "android"
	DeviceTypeWeb     DeviceType = "web"
	DeviceTypeDesktop DeviceType = "desktop"
)

// DeliveryStatus 投递状态
type DeliveryStatus int8

const (
	DeliveryStatusPending   DeliveryStatus = 0 // 待投递
	DeliveryStatusDelivered DeliveryStatus = 1 // 已投递
	DeliveryStatusFailed    DeliveryStatus = 2 // 投递失败
	DeliveryStatusExpired   DeliveryStatus = 3 // 已过期
)

// PendingMessage 待投递消息
type PendingMessage struct {
	ID             uint64         `json:"id"`
	UserID         uint64         `json:"user_id"`
	MessageID      uint64         `json:"message_id"`
	ConversationID uint64         `json:"conversation_id"`
	Payload        string         `json:"payload"`
	Status         DeliveryStatus `json:"status"`
	RetryCount     int            `json:"retry_count"`
	CreatedAt      time.Time      `json:"created_at"`
	DeliveredAt    *time.Time     `json:"delivered_at"`
}

// MessageEvent Kafka消息事件
type MessageEvent struct {
	Type           string    `json:"type"`
	MessageID      uint64    `json:"message_id"`
	ConversationID uint64    `json:"conversation_id"`
	SenderID       uint64    `json:"sender_id"`
	ReceiverIDs    []uint64  `json:"receiver_ids"`
	Seq            uint64    `json:"seq"`
	ContentType    int8      `json:"content_type"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// PushNotification 推送通知
type PushNotification struct {
	UserID   uint64            `json:"user_id"`
	DeviceID string            `json:"device_id"`
	Platform string            `json:"platform"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Data     map[string]string `json:"data"`
}

// SyncState 同步状态
type SyncState struct {
	UserID              uint64            `json:"user_id"`
	ConversationAckSeqs map[uint64]uint64 `json:"conversation_ack_seqs"`
	LastSyncAt          int64             `json:"last_sync_at"`
}

// MessageInfo 消息信息（用于同步）
type MessageInfo struct {
	ID             uint64 `json:"id"`
	ConversationID uint64 `json:"conversation_id"`
	SenderID       uint64 `json:"sender_id"`
	Seq            uint64 `json:"seq"`
	ContentType    int8   `json:"content_type"`
	Content        string `json:"content"`
	Status         int8   `json:"status"`
	CreatedAt      int64  `json:"created_at"`
}

// InboxInfo 收件箱信息
type InboxInfo struct {
	ConversationID   uint64 `json:"conversation_id"`
	LastReadSeq      uint64 `json:"last_read_seq"`
	LastDeliveredSeq uint64 `json:"last_delivered_seq"`
	UnreadCount      int    `json:"unread_count"`
	LastMsgTime      int64  `json:"last_msg_time"`
	IsMuted          bool   `json:"is_muted"`
}

// PendingAckItem 待确认消息项
type PendingAckItem struct {
	AckID          string    `json:"ack_id"`
	UserID         uint64    `json:"user_id"`
	MessageID      uint64    `json:"message_id"`
	ConversationID uint64    `json:"conversation_id"`
	Seq            uint64    `json:"seq"`
	SentAt         time.Time `json:"sent_at"`
	RetryCount     int       `json:"retry_count"`
	Status         string    `json:"status"` // pending, failed
}
