package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"github.com/go-redis/redis/v8"
)

type AccessTokenRepoRedis struct {
	client       *redis.Client
	blackListTTL time.Duration
}

func NewAccessTokenRepoRedis(client *redis.Client, blackListTTL time.Duration) out.AccessTokenRepository {
	return &AccessTokenRepoRedis{
		client:       client,
		blackListTTL: blackListTTL,
	}
}

func (r *AccessTokenRepoRedis) Find(ctx context.Context, token string) (*entity.AuthToken, error) {
	key := fmt.Sprintf("blocked_token:%s", token)
	exists, err := r.client.Exists(ctx, key).Result()

	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, nil
	}

	// 如果在黑名单中，返回一个已撤销标志的 AuthToken
	return &entity.AuthToken{IsRevoked: true}, nil
}

func (r *AccessTokenRepoRedis) Revoke(ctx context.Context, token string) error {
	key := fmt.Sprintf("blocked_token:%s", token)

	return r.client.Set(ctx, key, 1, r.blackListTTL).Err()
}
