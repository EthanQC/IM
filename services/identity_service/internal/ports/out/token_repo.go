package out

import (
	"context"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
)

type RefreshTokenRepository interface {
	Save(ctx context.Context, token *entity.AuthToken) error
	Find(ctx context.Context, token string) (*entity.AuthToken, error)
	UpdateExpiry(ctx context.Context, token string, newExp int64) error
	Revoke(ctx context.Context, token string) error
}

// AccessToken 不持久化，只签名后发给客户端
// 如果用户登出或被封禁，再把对应的 jti 写到 Redis 黑名单
// Redis 设置和 AccessToken 相同的过期 TTL，之后验证时直接查 Redis
type AccessTokenRepository interface {
	Find(ctx context.Context, token string) (*entity.AuthToken, error)
	Revoke(ctx context.Context, token string) error
}
