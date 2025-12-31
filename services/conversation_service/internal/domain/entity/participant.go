package entity

import (
	"time"
)

// Participant 会话成员
type Participant struct {
	ID             uint64
	ConversationID uint64
	UserID         uint64
	Role           ParticipantRole
	Nickname       *string
	Muted          bool
	MutedUntil     *time.Time
	JoinedAt       time.Time
	LastReadSeq    uint64
}

// ParticipantRole 成员角色
type ParticipantRole int8

const (
	ParticipantRoleMember ParticipantRole = 0 // 普通成员
	ParticipantRoleAdmin  ParticipantRole = 1 // 管理员
	ParticipantRoleOwner  ParticipantRole = 2 // 群主
)

// IsOwner 是否为群主
func (p *Participant) IsOwner() bool {
	return p.Role == ParticipantRoleOwner
}

// IsAdmin 是否为管理员
func (p *Participant) IsAdmin() bool {
	return p.Role == ParticipantRoleAdmin
}

// CanManageMembers 是否可以管理成员
func (p *Participant) CanManageMembers() bool {
	return p.Role == ParticipantRoleOwner || p.Role == ParticipantRoleAdmin
}

// IsMuted 是否被禁言
func (p *Participant) IsMuted() bool {
	if !p.Muted {
		return false
	}
	if p.MutedUntil == nil {
		return true
	}
	return time.Now().Before(*p.MutedUntil)
}

// Mute 禁言
func (p *Participant) Mute(until *time.Time) {
	p.Muted = true
	p.MutedUntil = until
}

// Unmute 取消禁言
func (p *Participant) Unmute() {
	p.Muted = false
	p.MutedUntil = nil
}

// SetRole 设置角色
func (p *Participant) SetRole(role ParticipantRole) {
	p.Role = role
}

// UpdateLastReadSeq 更新最后已读序号
func (p *Participant) UpdateLastReadSeq(seq uint64) {
	if seq > p.LastReadSeq {
		p.LastReadSeq = seq
	}
}

// NewParticipant 创建新成员
func NewParticipant(conversationID, userID uint64, role ParticipantRole) *Participant {
	return &Participant{
		ConversationID: conversationID,
		UserID:         userID,
		Role:           role,
		Muted:          false,
		JoinedAt:       time.Now(),
		LastReadSeq:    0,
	}
}
