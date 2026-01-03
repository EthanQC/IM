package in

import (
	"context"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
)

// UserUseCase 用户用例接口
type UserUseCase interface {
	// Register 用户注册
	Register(ctx context.Context, username, password, displayName string, phone, email *string) (*entity.User, error)
	
	// Login 用户登录
	Login(ctx context.Context, username, password string) (*entity.User, string, error)
	
	// GetProfile 获取用户资料
	GetProfile(ctx context.Context, userID uint64) (*entity.User, error)
	
	// UpdateProfile 更新用户资料
	UpdateProfile(ctx context.Context, userID uint64, displayName string, avatarURL *string) (*entity.User, error)
}

// ContactUseCase 联系人用例接口
type ContactUseCase interface {
	// ApplyContact 申请添加联系人
	ApplyContact(ctx context.Context, fromUserID, toUserID uint64, message *string) error
	
	// RespondContact 响应联系人申请
	RespondContact(ctx context.Context, fromUserID uint64, userID uint64, accept bool) error
	
	// RemoveContact 删除联系人
	RemoveContact(ctx context.Context, userID, friendID uint64) error
	
	// ListContacts 列出联系人列表
	ListContacts(ctx context.Context, userID uint64, page, pageSize int) ([]*entity.Contact, int, error)
	
	// AddToBlacklist 添加到黑名单
	AddToBlacklist(ctx context.Context, userID, blockedUserID uint64) error
	
	// RemoveFromBlacklist 从黑名单移除
	RemoveFromBlacklist(ctx context.Context, userID, blockedUserID uint64) error
}
