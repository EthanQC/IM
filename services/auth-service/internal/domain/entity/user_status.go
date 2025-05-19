package entity

import "time"

type UserStatus struct {
	UserID        string
	IsBlocked     bool   // 是否被封禁
	BlockReason   string // 封禁原因
	BlockedAt     time.Time
	BlockExpireAt time.Time
}

func (us *UserStatus) IsActive() bool {
	return !us.IsBlocked || time.Now().After(us.BlockExpireAt)
}
