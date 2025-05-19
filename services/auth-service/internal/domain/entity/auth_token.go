package entity

import (
	"time"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
)

type AuthToken struct {
	ID           string    // Token 唯一标识
	UserID       string    // 关联的用户 ID
	AccessToken  string    // 访问令牌
	RefreshToken string    // 刷新令牌
	ExpiresAt    time.Time // 过期时间
	CreatedAt    time.Time // 创建时间
	IsRevoked    bool      // 是否已撤销
	Roles        []vo.Role // 用户角色列表
}

func NewAuthToken(userID string) *AuthToken {
	return &AuthToken{
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // 默认 24 小时过期
		IsRevoked: false,
	}
}

// 业务方法

// 是否过期
func (at *AuthToken) IsExpired() bool {
	return time.Now().After(at.ExpiresAt)
}

// 撤销令牌
func (at *AuthToken) Revoke() {
	at.IsRevoked = true
}
