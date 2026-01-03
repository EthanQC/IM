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

// 确保实现接口
var _ out.PendingAckRepository = (*PendingAckRepositoryRedis)(nil)

// PendingAckRepositoryRedis 待确认消息仓储Redis实现
type PendingAckRepositoryRedis struct {
	client *redis.Client
	ttl    time.Duration
}

// NewPendingAckRepositoryRedis 创建待确认消息仓储
func NewPendingAckRepositoryRedis(client *redis.Client) *PendingAckRepositoryRedis {
	return &PendingAckRepositoryRedis{
		client: client,
		ttl:    30 * time.Minute, // 30分钟超时
	}
}

// 键名构造
func (r *PendingAckRepositoryRedis) pendingAckKey(userID uint64) string {
	return fmt.Sprintf("pending_ack:{%d}", userID)
}

func (r *PendingAckRepositoryRedis) pendingAckItemKey(userID, messageID uint64) string {
	return fmt.Sprintf("pending_ack_item:{%d}:{%d}", userID, messageID)
}

// Save 保存待确认记录
func (r *PendingAckRepositoryRedis) Save(ctx context.Context, item *entity.PendingAckItem) error {
	listKey := r.pendingAckKey(item.UserID)
	itemKey := r.pendingAckItemKey(item.UserID, item.MessageID)

	// 序列化项
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal pending ack item: %w", err)
	}

	pipe := r.client.Pipeline()

	// 存储详细信息
	pipe.Set(ctx, itemKey, data, r.ttl)

	// 添加到ZSet，score为发送时间
	pipe.ZAdd(ctx, listKey, redis.Z{
		Score:  float64(item.SentAt.UnixMilli()),
		Member: fmt.Sprintf("%d", item.MessageID),
	})
	pipe.Expire(ctx, listKey, r.ttl)

	_, err = pipe.Exec(ctx)
	return err
}

// Remove 移除待确认记录
func (r *PendingAckRepositoryRedis) Remove(ctx context.Context, userID, messageID uint64) error {
	listKey := r.pendingAckKey(userID)
	itemKey := r.pendingAckItemKey(userID, messageID)

	pipe := r.client.Pipeline()
	pipe.ZRem(ctx, listKey, fmt.Sprintf("%d", messageID))
	pipe.Del(ctx, itemKey)

	_, err := pipe.Exec(ctx)
	return err
}

// BatchRemove 批量移除
func (r *PendingAckRepositoryRedis) BatchRemove(ctx context.Context, userID uint64, messageIDs []uint64) error {
	if len(messageIDs) == 0 {
		return nil
	}

	listKey := r.pendingAckKey(userID)

	pipe := r.client.Pipeline()

	// 从ZSet中移除
	members := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		members[i] = fmt.Sprintf("%d", id)
	}
	pipe.ZRem(ctx, listKey, members...)

	// 删除详细信息
	itemKeys := make([]string, len(messageIDs))
	for i, id := range messageIDs {
		itemKeys[i] = r.pendingAckItemKey(userID, id)
	}
	pipe.Del(ctx, itemKeys...)

	_, err := pipe.Exec(ctx)
	return err
}

// GetPending 获取待确认列表
func (r *PendingAckRepositoryRedis) GetPending(ctx context.Context, userID uint64) ([]*entity.PendingAckItem, error) {
	listKey := r.pendingAckKey(userID)

	// 获取所有messageID
	messageIDStrs, err := r.client.ZRange(ctx, listKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get pending ack ids: %w", err)
	}

	if len(messageIDStrs) == 0 {
		return nil, nil
	}

	// 构建key列表
	keys := make([]string, len(messageIDStrs))
	for i, idStr := range messageIDStrs {
		var msgID uint64
		fmt.Sscanf(idStr, "%d", &msgID)
		keys[i] = r.pendingAckItemKey(userID, msgID)
	}

	// 批量获取
	results, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("get pending ack items: %w", err)
	}

	items := make([]*entity.PendingAckItem, 0, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}

		data, ok := result.(string)
		if !ok {
			continue
		}

		var item entity.PendingAckItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			continue
		}
		items = append(items, &item)
	}

	return items, nil
}

// IncrRetry 增加重试次数
func (r *PendingAckRepositoryRedis) IncrRetry(ctx context.Context, userID, messageID uint64) error {
	itemKey := r.pendingAckItemKey(userID, messageID)
	listKey := r.pendingAckKey(userID)

	// 获取当前项
	data, err := r.client.Get(ctx, itemKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // 不存在就忽略
		}
		return fmt.Errorf("get pending ack item: %w", err)
	}

	var item entity.PendingAckItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return fmt.Errorf("unmarshal pending ack item: %w", err)
	}

	// 更新重试信息
	item.RetryCount++
	item.SentAt = time.Now()

	// 保存
	newData, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal pending ack item: %w", err)
	}

	pipe := r.client.Pipeline()
	pipe.Set(ctx, itemKey, newData, r.ttl)
	// 更新score（用于下次超时检测）
	pipe.ZAdd(ctx, listKey, redis.Z{
		Score:  float64(item.SentAt.UnixMilli()),
		Member: fmt.Sprintf("%d", item.MessageID),
	})

	_, err = pipe.Exec(ctx)
	return err
}

// MarkFailed 标记为失败
func (r *PendingAckRepositoryRedis) MarkFailed(ctx context.Context, userID, messageID uint64) error {
	itemKey := r.pendingAckItemKey(userID, messageID)

	// 获取当前项
	data, err := r.client.Get(ctx, itemKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("get pending ack item: %w", err)
	}

	var item entity.PendingAckItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return fmt.Errorf("unmarshal pending ack item: %w", err)
	}

	// 标记为失败
	item.Status = "failed"

	// 保存
	newData, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal pending ack item: %w", err)
	}

	return r.client.Set(ctx, itemKey, newData, r.ttl).Err()
}

// GetExpiredPendingAcks 获取超时的待确认项（用于重试）
func (r *PendingAckRepositoryRedis) GetExpiredPendingAcks(ctx context.Context, userID uint64, timeout time.Duration) ([]*entity.PendingAckItem, error) {
	listKey := r.pendingAckKey(userID)
	expireTime := time.Now().Add(-timeout).UnixMilli()

	// 获取超时的messageID
	messageIDStrs, err := r.client.ZRangeByScore(ctx, listKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", expireTime),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("get expired ack ids: %w", err)
	}

	if len(messageIDStrs) == 0 {
		return nil, nil
	}

	// 构建key列表
	keys := make([]string, len(messageIDStrs))
	for i, idStr := range messageIDStrs {
		var msgID uint64
		fmt.Sscanf(idStr, "%d", &msgID)
		keys[i] = r.pendingAckItemKey(userID, msgID)
	}

	// 批量获取
	results, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("get expired ack items: %w", err)
	}

	items := make([]*entity.PendingAckItem, 0, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}

		data, ok := result.(string)
		if !ok {
			continue
		}

		var item entity.PendingAckItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			continue
		}
		items = append(items, &item)
	}

	return items, nil
}

// GetPendingCount 获取待确认消息数量
func (r *PendingAckRepositoryRedis) GetPendingCount(ctx context.Context, userID uint64) (int64, error) {
	listKey := r.pendingAckKey(userID)
	return r.client.ZCard(ctx, listKey).Result()
}
