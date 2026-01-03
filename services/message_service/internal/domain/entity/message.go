package entity

import (
	"encoding/json"
	"time"
)

// Message 消息聚合根
type Message struct {
	ID             uint64
	ConversationID uint64
	SenderID       uint64
	ClientMsgID    string
	Seq            uint64
	ContentType    MessageContentType
	Content        MessageContent
	Status         MessageStatus
	ReplyToMsgID   *uint64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// MessageContentType 消息内容类型
type MessageContentType int8

const (
	MessageContentTypeText     MessageContentType = 1 // 文本
	MessageContentTypeImage    MessageContentType = 2 // 图片
	MessageContentTypeAudio    MessageContentType = 3 // 语音
	MessageContentTypeVideo    MessageContentType = 4 // 视频
	MessageContentTypeFile     MessageContentType = 5 // 文件
	MessageContentTypeLocation MessageContentType = 6 // 位置
	MessageContentTypeSystem   MessageContentType = 7 // 系统通知
)

// MessageStatus 消息状态
type MessageStatus int8

const (
	MessageStatusRevoked MessageStatus = 0 // 已撤回
	MessageStatusNormal  MessageStatus = 1 // 正常
	MessageStatusDeleted MessageStatus = 2 // 已删除
)

// MessageContent 消息内容
type MessageContent struct {
	Text     *TextContent     `json:"text,omitempty"`
	Image    *MediaContent    `json:"image,omitempty"`
	Audio    *MediaContent    `json:"audio,omitempty"`
	Video    *MediaContent    `json:"video,omitempty"`
	File     *MediaContent    `json:"file,omitempty"`
	Location *LocationContent `json:"location,omitempty"`
	System   *SystemContent   `json:"system,omitempty"`
}

// TextContent 文本内容
type TextContent struct {
	Text string `json:"text"`
}

// MediaContent 媒体内容
type MediaContent struct {
	ObjectKey    string `json:"object_key"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	DurationSec  int    `json:"duration_sec,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	ThumbnailKey string `json:"thumbnail_key,omitempty"`
}

// LocationContent 位置内容
type LocationContent struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
}

// SystemContent 系统消息内容
type SystemContent struct {
	Type    string          `json:"type"` // member_join, member_leave, etc.
	Payload json.RawMessage `json:"payload"`
}

// IsRevoked 是否已撤回
func (m *Message) IsRevoked() bool {
	return m.Status == MessageStatusRevoked
}

// IsNormal 是否正常
func (m *Message) IsNormal() bool {
	return m.Status == MessageStatusNormal
}

// Revoke 撤回消息
func (m *Message) Revoke() {
	m.Status = MessageStatusRevoked
	m.UpdatedAt = time.Now()
}

// Delete 删除消息
func (m *Message) Delete() {
	m.Status = MessageStatusDeleted
	m.UpdatedAt = time.Now()
}

// CanRevoke 是否可以撤回（2分钟内）
func (m *Message) CanRevoke() bool {
	return time.Since(m.CreatedAt) <= 2*time.Minute && m.Status == MessageStatusNormal
}

// NewTextMessage 创建文本消息
func NewTextMessage(conversationID, senderID uint64, clientMsgID, text string) *Message {
	now := time.Now()
	return &Message{
		ConversationID: conversationID,
		SenderID:       senderID,
		ClientMsgID:    clientMsgID,
		ContentType:    MessageContentTypeText,
		Content: MessageContent{
			Text: &TextContent{Text: text},
		},
		Status:    MessageStatusNormal,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewMediaMessage 创建媒体消息
func NewMediaMessage(conversationID, senderID uint64, clientMsgID string, contentType MessageContentType, media *MediaContent) *Message {
	now := time.Now()
	content := MessageContent{}
	switch contentType {
	case MessageContentTypeImage:
		content.Image = media
	case MessageContentTypeAudio:
		content.Audio = media
	case MessageContentTypeVideo:
		content.Video = media
	case MessageContentTypeFile:
		content.File = media
	}
	return &Message{
		ConversationID: conversationID,
		SenderID:       senderID,
		ClientMsgID:    clientMsgID,
		ContentType:    contentType,
		Content:        content,
		Status:         MessageStatusNormal,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Inbox 收件箱实体
type Inbox struct {
	UserID           uint64    `json:"user_id"`
	ConversationID   uint64    `json:"conversation_id"`
	LastReadSeq      uint64    `json:"last_read_seq"`
	LastDeliveredSeq uint64    `json:"last_delivered_seq"`
	UnreadCount      int       `json:"unread_count"`
	IsMuted          bool      `json:"is_muted"`
	IsPinned         bool      `json:"is_pinned"`
	LastMsgTime      time.Time `json:"last_msg_time"`
}
