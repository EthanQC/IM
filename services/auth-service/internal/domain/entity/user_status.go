package entity

import "time"

type UserStatus struct {
	UserID        string
	IsBlocked     bool      // 是否被封禁
	BlockReason   string    // 封禁原因
	BlockedAt     time.Time // 封禁时间
	BlockExpireAt time.Time // 封禁过期时间
}

func NewUserStatus(userID string) *UserStatus {
	return &UserStatus{
		UserID:    userID,
		IsBlocked: false,
	}
}

// 封禁用户
func (us *UserStatus) Block(reason string, duration time.Duration) {
	us.IsBlocked = true
	us.BlockReason = reason
	us.BlockedAt = time.Now()
	us.BlockExpireAt = us.BlockedAt.Add(duration)
}

// 解除封禁
func (us *UserStatus) Unblock() {
	us.IsBlocked = false
	us.BlockReason = ""
}

// 检查是否处于封禁状态
func (us *UserStatus) IsActive() bool {
	if !us.IsBlocked {
		return true
	}

	return time.Now().After(us.BlockExpireAt)
}
