package entity

import (
	"time"
)

// User 用户聚合根
type User struct {
	ID           uint64
	Username     string
	PasswordHash string
	Phone        *string
	Email        *string
	DisplayName  string
	AvatarURL    *string
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// UserStatus 用户状态
type UserStatus int8

const (
	UserStatusDisabled UserStatus = 0 // 禁用
	UserStatusNormal   UserStatus = 1 // 正常
	UserStatusFrozen   UserStatus = 2 // 冻结
)

// IsActive 检查用户是否处于活跃状态
func (u *User) IsActive() bool {
	return u.Status == UserStatusNormal && u.DeletedAt == nil
}

// CanLogin 检查用户是否可以登录
func (u *User) CanLogin() bool {
	return u.IsActive()
}

// Update 更新用户信息
func (u *User) Update(displayName string, avatarURL *string) {
	if displayName != "" {
		u.DisplayName = displayName
	}
	if avatarURL != nil {
		u.AvatarURL = avatarURL
	}
	u.UpdatedAt = time.Now()
}

// Disable 禁用用户
func (u *User) Disable() {
	u.Status = UserStatusDisabled
	u.UpdatedAt = time.Now()
}

// Enable 启用用户
func (u *User) Enable() {
	u.Status = UserStatusNormal
	u.UpdatedAt = time.Now()
}

// Freeze 冻结用户
func (u *User) Freeze() {
	u.Status = UserStatusFrozen
	u.UpdatedAt = time.Now()
}
