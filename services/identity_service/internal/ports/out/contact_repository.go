package out

import (
	"context"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
)

// ContactRepository 联系人仓储接口
type ContactRepository interface {
	// Create 创建联系人关系
	Create(ctx context.Context, contact *entity.Contact) error
	
	// GetByID 根据ID获取联系人
	GetByID(ctx context.Context, id uint64) (*entity.Contact, error)
	
	// GetContact 获取两个用户之间的联系人关系
	GetContact(ctx context.Context, userID, friendID uint64) (*entity.Contact, error)
	
	// List 列出用户的联系人列表
	List(ctx context.Context, userID uint64, status entity.ContactStatus, page, pageSize int) ([]*entity.Contact, int, error)
	
	// Update 更新联系人
	Update(ctx context.Context, contact *entity.Contact) error
	
	// Delete 删除联系人关系
	Delete(ctx context.Context, userID, friendID uint64) error
	
	// IsContact 检查是否为联系人
	IsContact(ctx context.Context, userID, friendID uint64) (bool, error)
}

// ContactApplyRepository 好友申请仓储接口
type ContactApplyRepository interface {
	// Create 创建好友申请
	Create(ctx context.Context, apply *entity.ContactApply) error
	
	// GetByID 根据ID获取申请
	GetByID(ctx context.Context, id uint64) (*entity.ContactApply, error)
	
	// GetPendingApply 获取待处理的申请
	GetPendingApply(ctx context.Context, fromUserID, toUserID uint64) (*entity.ContactApply, error)
	
	// ListReceived 列出收到的申请
	ListReceived(ctx context.Context, userID uint64, status entity.ApplyStatus, page, pageSize int) ([]*entity.ContactApply, int, error)
	
	// ListSent 列出发出的申请
	ListSent(ctx context.Context, userID uint64, status entity.ApplyStatus, page, pageSize int) ([]*entity.ContactApply, int, error)
	
	// Update 更新申请
	Update(ctx context.Context, apply *entity.ContactApply) error
}

// BlacklistRepository 黑名单仓储接口
type BlacklistRepository interface {
	// Add 添加到黑名单
	Add(ctx context.Context, userID, blockedUserID uint64) error
	
	// Remove 从黑名单移除
	Remove(ctx context.Context, userID, blockedUserID uint64) error
	
	// IsBlocked 检查是否在黑名单中
	IsBlocked(ctx context.Context, userID, blockedUserID uint64) (bool, error)
	
	// List 列出黑名单
	List(ctx context.Context, userID uint64, page, pageSize int) ([]uint64, int, error)
}
