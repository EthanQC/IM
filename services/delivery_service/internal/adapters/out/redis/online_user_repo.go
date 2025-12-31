package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

const (
	// 在线用户Key前缀
	onlineUserKeyPrefix = "im:online:user:"
	// 在线用户过期时间
	onlineUserTTL = 5 * time.Minute
)

// OnlineUserRepositoryRedis Redis在线用户仓储实现
type OnlineUserRepositoryRedis struct {
	client *redis.Client
}

func NewOnlineUserRepositoryRedis(client *redis.Client) out.OnlineUserRepository {
	return &OnlineUserRepositoryRedis{client: client}
}

func (r *OnlineUserRepositoryRedis) getKey(userID uint64) string {
	return fmt.Sprintf("%s%d", onlineUserKeyPrefix, userID)
}

func (r *OnlineUserRepositoryRedis) SetOnline(ctx context.Context, user *entity.OnlineUser) error {
	key := r.getKey(user.UserID)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	// 使用HSET存储设备信息
	if err := r.client.HSet(ctx, key, user.DeviceID, string(data)).Err(); err != nil {
		return err
	}
	// 设置过期时间
	return r.client.Expire(ctx, key, onlineUserTTL).Err()
}

func (r *OnlineUserRepositoryRedis) SetOffline(ctx context.Context, userID uint64, deviceID string) error {
	key := r.getKey(userID)
	return r.client.HDel(ctx, key, deviceID).Err()
}

func (r *OnlineUserRepositoryRedis) GetOnlineDevices(ctx context.Context, userID uint64) ([]*entity.OnlineUser, error) {
	key := r.getKey(userID)
	result, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	devices := make([]*entity.OnlineUser, 0, len(result))
	for _, data := range result {
		var user entity.OnlineUser
		if err := json.Unmarshal([]byte(data), &user); err != nil {
			continue
		}
		devices = append(devices, &user)
	}
	return devices, nil
}

func (r *OnlineUserRepositoryRedis) IsOnline(ctx context.Context, userID uint64) (bool, error) {
	key := r.getKey(userID)
	count, err := r.client.HLen(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *OnlineUserRepositoryRedis) GetOnlineUsers(ctx context.Context, userIDs []uint64) (map[uint64][]*entity.OnlineUser, error) {
	result := make(map[uint64][]*entity.OnlineUser)

	pipe := r.client.Pipeline()
	cmds := make(map[uint64]*redis.MapStringStringCmd)

	for _, userID := range userIDs {
		key := r.getKey(userID)
		cmds[userID] = pipe.HGetAll(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	for userID, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil {
			continue
		}
		if len(data) == 0 {
			continue
		}

		devices := make([]*entity.OnlineUser, 0, len(data))
		for _, d := range data {
			var user entity.OnlineUser
			if err := json.Unmarshal([]byte(d), &user); err != nil {
				continue
			}
			devices = append(devices, &user)
		}
		if len(devices) > 0 {
			result[userID] = devices
		}
	}

	return result, nil
}

func (r *OnlineUserRepositoryRedis) UpdateLastPing(ctx context.Context, userID uint64, deviceID string) error {
	key := r.getKey(userID)
	
	// 先获取当前设备信息
	data, err := r.client.HGet(ctx, key, deviceID).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	var user entity.OnlineUser
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return err
	}

	user.LastPingAt = time.Now()
	newData, err := json.Marshal(user)
	if err != nil {
		return err
	}

	// 更新设备信息并刷新过期时间
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, deviceID, string(newData))
	pipe.Expire(ctx, key, onlineUserTTL)
	_, err = pipe.Exec(ctx)
	return err
}
