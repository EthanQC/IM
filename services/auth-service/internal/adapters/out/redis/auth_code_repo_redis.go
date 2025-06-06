package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
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
	b, _ := json.Marshal(code) // 序列化整个实体

	return r.client.Set(ctx, key, b, r.ttl).Err()
}

func (r *AuthCodeRepoRedis) Find(ctx context.Context, phone string) (*entity.AuthCode, error) {
	key := fmt.Sprintf("sms_code:%s", phone)
	data, err := r.client.Get(ctx, key).Bytes()

	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var code entity.AuthCode
	if err := json.Unmarshal(data, &code); err != nil {
		return nil, err
	}

	return &code, nil
}

func (r *AuthCodeRepoRedis) Delete(ctx context.Context, phone string) error {
	key := fmt.Sprintf("sms_code:%s", phone)

	return r.client.Del(ctx, key).Err()
}

func (r *AuthCodeRepoRedis) IncrementAttempts(ctx context.Context, phone string) error {
	key := fmt.Sprintf("sms_code:%s", phone)
	data, err := r.client.Get(ctx, key).Bytes()

	if err != nil {
		return err
	}

	var code entity.AuthCode
	if err := json.Unmarshal(data, &code); err != nil {
		return err
	}

	code.AttemptCnt++
	b, _ := json.Marshal(&code)

	return r.client.Set(ctx, key, b, r.ttl).Err()
}
