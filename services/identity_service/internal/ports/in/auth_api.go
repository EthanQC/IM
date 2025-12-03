package in

import (
	"context"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/domain/vo"
)

type AuthUseCase interface {
	// 刷新 token
	RefreshToken(ctx context.Context, refreshJTI string) (*entity.AuthToken, error)

	// 登录相关
	LoginByPassword(ctx context.Context, identifier string, password vo.Password) (*entity.AuthToken, error)
	LoginBySMS(ctx context.Context, phone vo.Phone, code string) (*entity.AuthToken, error)
	Logout(ctx context.Context, accessJTI string) error
}
