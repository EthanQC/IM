package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/EthanQC/IM/services/presence_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/presence_service/internal/ports/out"
)

const (
	// 在线状态Key前缀
	presenceKeyPrefix = "im:presence:"
	// 在线状态过期时间（心跳间隔的3倍）
	presenceTTL = 3 * time.Minute
	// 最后活跃时间Key前缀
	lastSeenKeyPrefix = "im:lastseen:"
)

// PresenceRepositoryRedis Redis在线状态仓储实现
type PresenceRepositoryRedis struct {
	client *redis.Client
}

func NewPresenceRepositoryRedis(client *redis.Client) out.PresenceRepository {
	return &PresenceRepositoryRedis{client: client}
}

func (r *PresenceRepositoryRedis) getKey(userID uint64) string {
	return fmt.Sprintf("%s%d", presenceKeyPrefix, userID)
}

func (r *PresenceRepositoryRedis) getLastSeenKey(userID uint64) string {
	return fmt.Sprintf("%s%d", lastSeenKeyPrefix, userID)
}

func (r *PresenceRepositoryRedis) SetOnline(ctx context.Context, userID uint64, nodeID string, deviceType string) error {
	presence := &entity.UserPresence{
		UserID:     userID,
		Online:     true,
		Status:     string(entity.PresenceStatusOnline),
		NodeID:     nodeID,
		DeviceType: deviceType,
		LastSeenAt: time.Now(),
		UpdatedAt:  time.Now(),
	}

	data, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	key := r.getKey(userID)
	return r.client.Set(ctx, key, string(data), presenceTTL).Err()
}

func (r *PresenceRepositoryRedis) SetOffline(ctx context.Context, userID uint64, nodeID string) error {
	key := r.getKey(userID)
	
	// 获取当前状态检查nodeID是否匹配
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	var presence entity.UserPresence
	if err := json.Unmarshal([]byte(data), &presence); err != nil {
		return err
	}

	// 只有当nodeID匹配时才设置离线
	if presence.NodeID != nodeID {
		return nil
	}

	// 更新最后活跃时间
	lastSeenKey := r.getLastSeenKey(userID)
	r.client.Set(ctx, lastSeenKey, time.Now().Unix(), 7*24*time.Hour) // 保留7天

	// 删除在线状态
	return r.client.Del(ctx, key).Err()
}

func (r *PresenceRepositoryRedis) UpdateStatus(ctx context.Context, userID uint64, status entity.PresenceStatus) error {
	key := r.getKey(userID)
	
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	var presence entity.UserPresence
	if err := json.Unmarshal([]byte(data), &presence); err != nil {
		return err
	}

	presence.Status = string(status)
	presence.UpdatedAt = time.Now()

	newData, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, string(newData), presenceTTL).Err()
}

func (r *PresenceRepositoryRedis) SetCustomStatus(ctx context.Context, userID uint64, customStatus string) error {
	key := r.getKey(userID)
	
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	var presence entity.UserPresence
	if err := json.Unmarshal([]byte(data), &presence); err != nil {
		return err
	}

	presence.CustomStatus = customStatus
	presence.UpdatedAt = time.Now()

	newData, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, string(newData), presenceTTL).Err()
}

func (r *PresenceRepositoryRedis) GetPresence(ctx context.Context, userID uint64) (*entity.UserPresence, error) {
	key := r.getKey(userID)
	
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// 用户离线，尝试获取最后活跃时间
			lastSeenKey := r.getLastSeenKey(userID)
			lastSeen, _ := r.client.Get(ctx, lastSeenKey).Int64()
			return &entity.UserPresence{
				UserID:     userID,
				Online:     false,
				Status:     string(entity.PresenceStatusOffline),
				LastSeenAt: time.Unix(lastSeen, 0),
			}, nil
		}
		return nil, err
	}

	var presence entity.UserPresence
	if err := json.Unmarshal([]byte(data), &presence); err != nil {
		return nil, err
	}

	return &presence, nil
}

func (r *PresenceRepositoryRedis) GetPresences(ctx context.Context, userIDs []uint64) (map[uint64]*entity.UserPresence, error) {
	result := make(map[uint64]*entity.UserPresence)
	
	if len(userIDs) == 0 {
		return result, nil
	}

	// 使用Pipeline批量获取
	pipe := r.client.Pipeline()
	cmds := make(map[uint64]*redis.StringCmd)
	
	for _, userID := range userIDs {
		key := r.getKey(userID)
		cmds[userID] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	// 获取最后活跃时间的pipeline
	lastSeenPipe := r.client.Pipeline()
	lastSeenCmds := make(map[uint64]*redis.StringCmd)

	for userID, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil {
			if err == redis.Nil {
				// 离线用户，需要获取最后活跃时间
				lastSeenKey := r.getLastSeenKey(userID)
				lastSeenCmds[userID] = lastSeenPipe.Get(ctx, lastSeenKey)
				continue
			}
			continue
		}

		var presence entity.UserPresence
		if err := json.Unmarshal([]byte(data), &presence); err != nil {
			continue
		}
		result[userID] = &presence
	}

	// 执行获取最后活跃时间
	if len(lastSeenCmds) > 0 {
		lastSeenPipe.Exec(ctx)
		for userID, cmd := range lastSeenCmds {
			lastSeen, _ := cmd.Int64()
			result[userID] = &entity.UserPresence{
				UserID:     userID,
				Online:     false,
				Status:     string(entity.PresenceStatusOffline),
				LastSeenAt: time.Unix(lastSeen, 0),
			}
		}
	}

	return result, nil
}

func (r *PresenceRepositoryRedis) UpdateHeartbeat(ctx context.Context, userID uint64) error {
	key := r.getKey(userID)
	
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	var presence entity.UserPresence
	if err := json.Unmarshal([]byte(data), &presence); err != nil {
		return err
	}

	presence.LastSeenAt = time.Now()
	presence.UpdatedAt = time.Now()

	newData, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	// 更新并刷新TTL
	return r.client.Set(ctx, key, string(newData), presenceTTL).Err()
}

func (r *PresenceRepositoryRedis) CleanExpired(ctx context.Context) (int64, error) {
	// Redis的TTL机制会自动清理过期的Key
	// 这里主要用于清理last_seen中超过保留期的数据
	// 实际生产中可以使用SCAN命令遍历清理
	return 0, nil
}
