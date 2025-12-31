package entity

import (
	"time"
)

// Conversation 会话聚合根
type Conversation struct {
	ID          uint64
	Type        ConversationType
	Title       *string
	AvatarURL   *string
	OwnerID     *uint64
	MemberLimit int
	JoinMode    JoinMode
	MuteAll     bool
	Status      ConversationStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ConversationType 会话类型
type ConversationType int8

const (
	ConversationTypeSingle ConversationType = 1 // 单聊
	ConversationTypeGroup  ConversationType = 2 // 群聊
)

// ConversationStatus 会话状态
type ConversationStatus int8

const (
	ConversationStatusDissolved ConversationStatus = 0 // 已解散
	ConversationStatusNormal    ConversationStatus = 1 // 正常
)

// JoinMode 加入方式
type JoinMode int8

const (
	JoinModeApproval JoinMode = 0 // 需要审批
	JoinModeFree     JoinMode = 1 // 自由加入
)

// IsSingle 是否为单聊
func (c *Conversation) IsSingle() bool {
	return c.Type == ConversationTypeSingle
}

// IsGroup 是否为群聊
func (c *Conversation) IsGroup() bool {
	return c.Type == ConversationTypeGroup
}

// IsActive 会话是否活跃
func (c *Conversation) IsActive() bool {
	return c.Status == ConversationStatusNormal
}

// Update 更新会话信息
func (c *Conversation) Update(title, avatarURL *string) {
	if title != nil {
		c.Title = title
	}
	if avatarURL != nil {
		c.AvatarURL = avatarURL
	}
	c.UpdatedAt = time.Now()
}

// Dissolve 解散会话
func (c *Conversation) Dissolve() {
	c.Status = ConversationStatusDissolved
	c.UpdatedAt = time.Now()
}

// SetMuteAll 设置全员禁言
func (c *Conversation) SetMuteAll(mute bool) {
	c.MuteAll = mute
	c.UpdatedAt = time.Now()
}

// NewSingleConversation 创建单聊会话
func NewSingleConversation() *Conversation {
	now := time.Now()
	return &Conversation{
		Type:        ConversationTypeSingle,
		MemberLimit: 2,
		JoinMode:    JoinModeApproval,
		Status:      ConversationStatusNormal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewGroupConversation 创建群聊会话
func NewGroupConversation(title string, ownerID uint64) *Conversation {
	now := time.Now()
	return &Conversation{
		Type:        ConversationTypeGroup,
		Title:       &title,
		OwnerID:     &ownerID,
		MemberLimit: 500,
		JoinMode:    JoinModeApproval,
		Status:      ConversationStatusNormal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
