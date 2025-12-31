package entity

import (
	"time"
)

// Contact 联系人关系
type Contact struct {
	ID        uint64
	UserID    uint64
	FriendID  uint64
	Remark    *string
	Status    ContactStatus
	Type      ContactType
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ContactStatus 联系人状态
type ContactStatus int8

const (
	ContactStatusDeleted  ContactStatus = 0 // 已删除
	ContactStatusNormal   ContactStatus = 1 // 正常
	ContactStatusBlocked  ContactStatus = 2 // 已拉黑
)

// ContactType 联系人类型
type ContactType int8

const (
	ContactTypeFriend   ContactType = 1 // 好友
	ContactTypeStranger ContactType = 2 // 陌生人
)

// IsBlocked 检查是否被拉黑
func (c *Contact) IsBlocked() bool {
	return c.Status == ContactStatusBlocked
}

// IsActive 检查联系人关系是否活跃
func (c *Contact) IsActive() bool {
	return c.Status == ContactStatusNormal
}

// Block 拉黑
func (c *Contact) Block() {
	c.Status = ContactStatusBlocked
	c.UpdatedAt = time.Now()
}

// Unblock 取消拉黑
func (c *Contact) Unblock() {
	c.Status = ContactStatusNormal
	c.UpdatedAt = time.Now()
}

// Delete 删除联系人
func (c *Contact) Delete() {
	c.Status = ContactStatusDeleted
	c.UpdatedAt = time.Now()
}
