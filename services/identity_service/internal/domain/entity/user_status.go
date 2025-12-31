package entity

import "time"

// UserBlockStatus 用户封禁状态
type UserBlockStatus struct {
	UserID        string
	IsBlocked     bool      // 是否被封禁
	BlockReason   string    // 封禁原因
	BlockedAt     time.Time // 封禁时间
	BlockExpireAt time.Time // 封禁过期时间
}

func NewUserBlockStatus(userID string) *UserBlockStatus {
	return &UserBlockStatus{
		UserID:    userID,
		IsBlocked: false,
	}
}

// Block 封禁用户
func (us *UserBlockStatus) Block(reason string, duration time.Duration) {
	us.IsBlocked = true
	us.BlockReason = reason
	us.BlockedAt = time.Now()
	us.BlockExpireAt = us.BlockedAt.Add(duration)
}

// Unblock 解除封禁
func (us *UserBlockStatus) Unblock() {
	us.IsBlocked = false
	us.BlockReason = ""
}

// IsActive 检查是否处于封禁状态
func (us *UserBlockStatus) IsActive() bool {
	if !us.IsBlocked {
		return true
	}

	return time.Now().After(us.BlockExpireAt)
}
