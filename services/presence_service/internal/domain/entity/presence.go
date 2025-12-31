package entity

import "time"

// UserPresence 用户在线状态
type UserPresence struct {
	UserID       uint64    `json:"user_id"`
	Online       bool      `json:"online"`
	Status       string    `json:"status"` // online, away, busy, offline
	CustomStatus string    `json:"custom_status,omitempty"`
	NodeID       string    `json:"node_id,omitempty"` // 连接的服务器节点ID
	DeviceType   string    `json:"device_type,omitempty"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PresenceStatus 状态枚举
type PresenceStatus string

const (
	PresenceStatusOnline  PresenceStatus = "online"
	PresenceStatusAway    PresenceStatus = "away"
	PresenceStatusBusy    PresenceStatus = "busy"
	PresenceStatusOffline PresenceStatus = "offline"
)

// PresenceEvent 状态变更事件
type PresenceEvent struct {
	UserID    uint64         `json:"user_id"`
	OldStatus PresenceStatus `json:"old_status"`
	NewStatus PresenceStatus `json:"new_status"`
	Timestamp time.Time      `json:"timestamp"`
}
