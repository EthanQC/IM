package service

import (
	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
	"github.com/EthanQC/IM/services/auth-service/pkg/errors"
)

type AuthDomainService struct{}

// 验证访问权限
func (s *AuthDomainService) ValidateAccess(rule *vo.AccessRule, token *entity.AuthToken) error {
	// 检查是否公开访问
	if rule.IsPublic {
		return nil
	}

	// 检查令牌状态
	if token.IsExpired() || token.IsRevoked {
		return errors.ErrInvalidToken
	}

	// 检查用户角色权限
	for _, role := range token.Roles {
		if role.HasPermission(rule.Path) {
			return nil
		}
	}

	return errors.ErrPermissionDenied
}

// 验证验证码
func (s *AuthDomainService) ValidateAuthCode(code *entity.AuthCode, token *entity.AuthToken) error {
	if code.IsExpired() {
		return errors.ErrCodeExpired
	}

	if code.HasExceededMaxAttempts() {
		return errors.ErrTooManyAttempts
	}

	return nil
}

// ValidateToken 验证令牌状态
func (s *AuthDomainService) ValidateToken(token *entity.AuthToken, userStatus *entity.UserStatus) error {
	// 1. 检查令牌是否过期
	if token.IsExpired() {
		return errors.ErrTokenExpired
	}

	// 2. 检查令牌是否被撤销
	if token.IsRevoked {
		return errors.ErrTokenRevoked
	}

	// 3. 检查用户是否被封禁
	if !userStatus.IsActive() {
		return errors.ErrUserBlocked
	}

	return nil
}
