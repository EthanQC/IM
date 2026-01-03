package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
)

const (
	// 消息Timeline Key前缀 (ZSet: score=seq, member=messageJSON)
	timelineKeyPrefix = "im:timeline:conv:"
	// 最近消息缓存条数
	defaultTimelineSize = 100
	// Timeline 过期时间 (7天)
	timelineTTL = 7 * 24 * time.Hour
)

// MessageCacheItem 消息缓存项
type MessageCacheItem struct {
	ID             uint64                `json:"id"`
	ConversationID uint64                `json:"conversation_id"`
	SenderID       uint64                `json:"sender_id"`
	ClientMsgID    string                `json:"client_msg_id"`
	Seq            uint64                `json:"seq"`
	ContentType    int8                  `json:"content_type"`
	Content        entity.MessageContent `json:"content"`
	Status         int8                  `json:"status"`
	ReplyToMsgID   *uint64               `json:"reply_to_msg_id,omitempty"`
	CreatedAt      int64                 `json:"created_at"`
}

// TimelineRepositoryRedis 消息Timeline仓储
// 使用 Redis ZSet 存储最近 N 条消息，以 seq 为 score
type TimelineRepositoryRedis struct {
	client       *redis.Client
	timelineSize int
}

func NewTimelineRepositoryRedis(client *redis.Client) *TimelineRepositoryRedis {
	return &TimelineRepositoryRedis{
		client:       client,
		timelineSize: defaultTimelineSize,
	}
}

func (r *TimelineRepositoryRedis) getKey(conversationID uint64) string {
	return fmt.Sprintf("%s%d", timelineKeyPrefix, conversationID)
}

// AddMessage 添加消息到Timeline
func (r *TimelineRepositoryRedis) AddMessage(ctx context.Context, conversationID uint64, msg *entity.Message) error {
	key := r.getKey(conversationID)

	item := MessageCacheItem{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		ClientMsgID:    msg.ClientMsgID,
		Seq:            msg.Seq,
		ContentType:    int8(msg.ContentType),
		Content:        msg.Content,
		Status:         int8(msg.Status),
		ReplyToMsgID:   msg.ReplyToMsgID,
		CreatedAt:      msg.CreatedAt.Unix(),
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	pipe := r.client.Pipeline()

	// 添加到ZSet，score为seq
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(msg.Seq),
		Member: string(data),
	})

	// 保留最近N条消息（移除最旧的）
	pipe.ZRemRangeByRank(ctx, key, 0, int64(-r.timelineSize-1))

	// 刷新过期时间
	pipe.Expire(ctx, key, timelineTTL)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("add message to timeline failed: %w", err)
	}

	return nil
}

// GetMessagesAfterSeq 获取指定seq之后的消息（用于增量拉取）
func (r *TimelineRepositoryRedis) GetMessagesAfterSeq(ctx context.Context, conversationID uint64, afterSeq uint64, limit int) ([]*entity.Message, error) {
	key := r.getKey(conversationID)

	// 使用 ZRANGEBYSCORE 获取 score > afterSeq 的消息
	// 分数范围: (afterSeq, +inf]
	results, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:   fmt.Sprintf("(%d", afterSeq), // 开区间，不包含 afterSeq
		Max:   "+inf",
		Count: int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("get messages after seq failed: %w", err)
	}

	return r.parseMessages(results)
}

// GetMessagesBeforeSeq 获取指定seq之前的消息（用于历史加载）
func (r *TimelineRepositoryRedis) GetMessagesBeforeSeq(ctx context.Context, conversationID uint64, beforeSeq uint64, limit int) ([]*entity.Message, error) {
	key := r.getKey(conversationID)

	// 使用 ZREVRANGEBYSCORE 获取 score < beforeSeq 的消息，逆序
	results, err := r.client.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("(%d", beforeSeq), // 开区间，不包含 beforeSeq
		Count: int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("get messages before seq failed: %w", err)
	}

	messages, err := r.parseMessages(results)
	if err != nil {
		return nil, err
	}

	// 反转顺序，使其按seq升序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetLatestMessages 获取最新的N条消息
func (r *TimelineRepositoryRedis) GetLatestMessages(ctx context.Context, conversationID uint64, limit int) ([]*entity.Message, error) {
	key := r.getKey(conversationID)

	// 使用 ZREVRANGE 获取最新的消息
	results, err := r.client.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("get latest messages failed: %w", err)
	}

	messages, err := r.parseMessages(results)
	if err != nil {
		return nil, err
	}

	// 反转顺序，使其按seq升序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetLatestSeq 从Timeline获取最新序号
func (r *TimelineRepositoryRedis) GetLatestSeq(ctx context.Context, conversationID uint64) (uint64, error) {
	key := r.getKey(conversationID)

	// 获取score最大的元素
	results, err := r.client.ZRevRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil {
		return 0, fmt.Errorf("get latest seq failed: %w", err)
	}

	if len(results) == 0 {
		return 0, nil
	}

	return uint64(results[0].Score), nil
}

// RemoveMessage 从Timeline移除消息（撤回时使用）
func (r *TimelineRepositoryRedis) RemoveMessage(ctx context.Context, conversationID uint64, seq uint64) error {
	key := r.getKey(conversationID)

	// 按score精确删除
	_, err := r.client.ZRemRangeByScore(ctx, key,
		strconv.FormatUint(seq, 10),
		strconv.FormatUint(seq, 10)).Result()
	if err != nil {
		return fmt.Errorf("remove message from timeline failed: %w", err)
	}

	return nil
}

// UpdateMessageStatus 更新消息状态（如撤回）
func (r *TimelineRepositoryRedis) UpdateMessageStatus(ctx context.Context, msg *entity.Message) error {
	// 先删除旧的，再添加新的
	if err := r.RemoveMessage(ctx, msg.ConversationID, msg.Seq); err != nil {
		return err
	}

	return r.AddMessage(ctx, msg.ConversationID, msg)
}

// Exists 检查Timeline是否存在
func (r *TimelineRepositoryRedis) Exists(ctx context.Context, conversationID uint64) (bool, error) {
	key := r.getKey(conversationID)

	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check timeline exists failed: %w", err)
	}

	return exists > 0, nil
}

// parseMessages 解析消息列表
func (r *TimelineRepositoryRedis) parseMessages(results []string) ([]*entity.Message, error) {
	messages := make([]*entity.Message, 0, len(results))

	for _, data := range results {
		var item MessageCacheItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			continue // 跳过解析失败的消息
		}

		msg := &entity.Message{
			ID:             item.ID,
			ConversationID: item.ConversationID,
			SenderID:       item.SenderID,
			ClientMsgID:    item.ClientMsgID,
			Seq:            item.Seq,
			ContentType:    entity.MessageContentType(item.ContentType),
			Content:        item.Content,
			Status:         entity.MessageStatus(item.Status),
			ReplyToMsgID:   item.ReplyToMsgID,
			CreatedAt:      time.Unix(item.CreatedAt, 0),
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// BatchAddMessages 批量添加消息到Timeline
func (r *TimelineRepositoryRedis) BatchAddMessages(ctx context.Context, conversationID uint64, messages []*entity.Message) error {
	if len(messages) == 0 {
		return nil
	}

	key := r.getKey(conversationID)

	members := make([]redis.Z, 0, len(messages))
	for _, msg := range messages {
		item := MessageCacheItem{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			ClientMsgID:    msg.ClientMsgID,
			Seq:            msg.Seq,
			ContentType:    int8(msg.ContentType),
			Content:        msg.Content,
			Status:         int8(msg.Status),
			ReplyToMsgID:   msg.ReplyToMsgID,
			CreatedAt:      msg.CreatedAt.Unix(),
		}

		data, err := json.Marshal(item)
		if err != nil {
			continue
		}

		members = append(members, redis.Z{
			Score:  float64(msg.Seq),
			Member: string(data),
		})
	}

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, key, members...)
	pipe.ZRemRangeByRank(ctx, key, 0, int64(-r.timelineSize-1))
	pipe.Expire(ctx, key, timelineTTL)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("batch add messages to timeline failed: %w", err)
	}

	return nil
}
