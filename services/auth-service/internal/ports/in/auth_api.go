package in

import (
	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
)

type AuthApi interface {
	// Token 相关
	GenerateToken(userID string) (*entity.AuthToken, error)
	ValidateToken(token string) (*entity.AuthToken, error)
	RefreshToken(refreshToken string) (*entity.AuthToken, error)
	RevokeToken(tokenStr string) error

	// 权限相关
	ValidateAccess(path string, method string, token *entity.AuthToken) error
	UpdateUserRoles(userID string, roles []string) error
}
