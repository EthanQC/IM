package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
	"github.com/go-redis/redis/v8"
)

type AuthCodeRepoRedis struct {
	client *redis.Client
	ttl    time.Duration
}

func NewAuthCodeRepoRedis(client *redis.Client, ttl time.Duration) out.AuthCodeRepository {
	return &AuthCodeRepoRedis{client: client, ttl: ttl}
}

func (r *AuthCodeRepoRedis) Save(ctx context.Context, code *entity.AuthCode) error {
	key := fmt.Sprintf("sms_code:%s", code.Phone)
	b, err := json.Marshal(code)
	if err != nil {
		return fmt.Errorf("序列化验证码失败: %w", err)
	}
	return r.client.Set(ctx, key, b, r.ttl).Err()
}

func (r *AuthCodeRepoRedis) Find(ctx context.Context, phone string) (*entity.AuthCode, error) {
	key := fmt.Sprintf("sms_code:%s", phone)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("读取验证码失败: %w", err)
	}

	var ac entity.AuthCode
	if err := json.Unmarshal(data, &ac); err != nil {
		return nil, fmt.Errorf("反序列化验证码失败: %w", err)
	}
	return &ac, nil
}

func (r *AuthCodeRepoRedis) Delete(ctx context.Context, phone string) error {
	key := fmt.Sprintf("sms_code:%s", phone)
	return r.client.Del(ctx, key).Err()
}

func (r *AuthCodeRepoRedis) IncrementAttempts(ctx context.Context, phone string) error {
	ac, err := r.Find(ctx, phone)
	if err != nil {
		return err
	}
	if ac == nil {
		return nil
	}
	ac.AttemptCnt++
	// 保持原 TTL
	return r.Save(ctx, ac)
}
