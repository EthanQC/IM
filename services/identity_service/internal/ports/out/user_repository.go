package out

import (
	"context"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
)

// UserRepository 用户仓储接口
type UserRepository interface {
	// Create 创建用户
	Create(ctx context.Context, user *entity.User) error
	
	// GetByID 根据ID获取用户
	GetByID(ctx context.Context, id uint64) (*entity.User, error)
	
	// GetByUsername 根据用户名获取用户
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	
	// GetByPhone 根据手机号获取用户
	GetByPhone(ctx context.Context, phone string) (*entity.User, error)
	
	// GetByEmail 根据邮箱获取用户
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	
	// Update 更新用户
	Update(ctx context.Context, user *entity.User) error
	
	// ExistsByUsername 检查用户名是否存在
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	
	// ExistsByPhone 检查手机号是否存在
	ExistsByPhone(ctx context.Context, phone string) (bool, error)
	
	// ExistsByEmail 检查邮箱是否存在
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
