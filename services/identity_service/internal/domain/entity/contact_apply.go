package entity

import (
	"time"
)

// ContactApply 好友申请
type ContactApply struct {
	ID         uint64
	FromUserID uint64
	ToUserID   uint64
	Message    *string
	Status     ApplyStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ApplyStatus 申请状态
type ApplyStatus int8

const (
	ApplyStatusPending  ApplyStatus = 0 // 待处理
	ApplyStatusAccepted ApplyStatus = 1 // 已同意
	ApplyStatusRejected ApplyStatus = 2 // 已拒绝
	ApplyStatusExpired  ApplyStatus = 3 // 已过期
)

// IsPending 检查是否待处理
func (a *ContactApply) IsPending() bool {
	return a.Status == ApplyStatusPending
}

// Accept 接受申请
func (a *ContactApply) Accept() {
	a.Status = ApplyStatusAccepted
	a.UpdatedAt = time.Now()
}

// Reject 拒绝申请
func (a *ContactApply) Reject() {
	a.Status = ApplyStatusRejected
	a.UpdatedAt = time.Now()
}

// Expire 使申请过期
func (a *ContactApply) Expire() {
	a.Status = ApplyStatusExpired
	a.UpdatedAt = time.Now()
}
