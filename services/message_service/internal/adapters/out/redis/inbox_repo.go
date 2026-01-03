package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

const (
	// 收件箱Key前缀 (Hash: field=conversationID, value=inboxJSON)
	inboxKeyPrefix = "im:inbox:user:"
	// 会话列表Key前缀 (ZSet: score=lastMsgTime, member=conversationID)
	convListKeyPrefix = "im:convlist:user:"
	// 收件箱过期时间
	inboxTTL = 24 * time.Hour
)

// InboxCacheItem 收件箱缓存项
type InboxCacheItem struct {
	ConversationID   uint64 `json:"conversation_id"`
	LastReadSeq      uint64 `json:"last_read_seq"`
	LastDeliveredSeq uint64 `json:"last_delivered_seq"`
	UnreadCount      int    `json:"unread_count"`
	IsMuted          bool   `json:"is_muted"`
	IsPinned         bool   `json:"is_pinned"`
	LastMsgSeq       uint64 `json:"last_msg_seq"`
	LastMsgTime      int64  `json:"last_msg_time"`
}

// Lua脚本：原子性更新已读位置并计算未读数
var updateReadSeqScript = redis.NewScript(`
local inbox_key = KEYS[1]
local conv_id = ARGV[1]
local new_read_seq = tonumber(ARGV[2])

local data = redis.call('HGET', inbox_key, conv_id)
if not data then
    return {err = 'inbox not found'}
end

local inbox = cjson.decode(data)
local old_read_seq = inbox.last_read_seq or 0

if new_read_seq > old_read_seq then
    inbox.last_read_seq = new_read_seq
    -- 重新计算未读数
    local delivered_seq = inbox.last_delivered_seq or 0
    if new_read_seq >= delivered_seq then
        inbox.unread_count = 0
    else
        inbox.unread_count = delivered_seq - new_read_seq
    end
    redis.call('HSET', inbox_key, conv_id, cjson.encode(inbox))
end

return inbox.unread_count
`)

// Lua脚本：原子性更新投递位置并增加未读数
var updateDeliveredSeqScript = redis.NewScript(`
local inbox_key = KEYS[1]
local convlist_key = KEYS[2]
local conv_id = ARGV[1]
local new_delivered_seq = tonumber(ARGV[2])
local msg_time = tonumber(ARGV[3])
local is_self = tonumber(ARGV[4])

local data = redis.call('HGET', inbox_key, conv_id)
local inbox

if not data then
    -- 创建新的收件箱
    inbox = {
        conversation_id = tonumber(conv_id),
        last_read_seq = 0,
        last_delivered_seq = new_delivered_seq,
        unread_count = 0,
        is_muted = false,
        is_pinned = false,
        last_msg_seq = new_delivered_seq,
        last_msg_time = msg_time
    }
    if is_self == 0 then
        inbox.unread_count = 1
    end
else
    inbox = cjson.decode(data)
    local old_delivered_seq = inbox.last_delivered_seq or 0
    
    if new_delivered_seq > old_delivered_seq then
        inbox.last_delivered_seq = new_delivered_seq
        inbox.last_msg_seq = new_delivered_seq
        inbox.last_msg_time = msg_time
        
        -- 如果不是自己发的消息，增加未读数
        if is_self == 0 then
            inbox.unread_count = (inbox.unread_count or 0) + 1
        end
    end
end

redis.call('HSET', inbox_key, conv_id, cjson.encode(inbox))
-- 更新会话列表排序
redis.call('ZADD', convlist_key, msg_time, conv_id)

return inbox.unread_count
`)

// InboxRepositoryRedis Redis收件箱仓储实现
type InboxRepositoryRedis struct {
	client *redis.Client
}

func NewInboxRepositoryRedis(client *redis.Client) *InboxRepositoryRedis {
	return &InboxRepositoryRedis{client: client}
}

func (r *InboxRepositoryRedis) getInboxKey(userID uint64) string {
	return fmt.Sprintf("%s%d", inboxKeyPrefix, userID)
}

func (r *InboxRepositoryRedis) getConvListKey(userID uint64) string {
	return fmt.Sprintf("%s%d", convListKeyPrefix, userID)
}

// GetOrCreate 获取或创建收件箱记录
func (r *InboxRepositoryRedis) GetOrCreate(ctx context.Context, userID, conversationID uint64) (*out.Inbox, error) {
	key := r.getInboxKey(userID)
	convIDStr := strconv.FormatUint(conversationID, 10)

	data, err := r.client.HGet(ctx, key, convIDStr).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("get inbox failed: %w", err)
	}

	if err == redis.Nil || data == "" {
		// 创建新的收件箱
		inbox := &out.Inbox{
			UserID:           userID,
			ConversationID:   conversationID,
			LastReadSeq:      0,
			LastDeliveredSeq: 0,
			UnreadCount:      0,
			IsMuted:          false,
			IsPinned:         false,
		}

		item := InboxCacheItem{
			ConversationID:   conversationID,
			LastReadSeq:      0,
			LastDeliveredSeq: 0,
			UnreadCount:      0,
			IsMuted:          false,
			IsPinned:         false,
			LastMsgSeq:       0,
			LastMsgTime:      time.Now().Unix(),
		}

		itemData, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("marshal inbox failed: %w", err)
		}

		pipe := r.client.Pipeline()
		pipe.HSet(ctx, key, convIDStr, string(itemData))
		pipe.Expire(ctx, key, inboxTTL)
		_, err = pipe.Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("create inbox failed: %w", err)
		}

		return inbox, nil
	}

	var item InboxCacheItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return nil, fmt.Errorf("unmarshal inbox failed: %w", err)
	}

	return &out.Inbox{
		UserID:           userID,
		ConversationID:   item.ConversationID,
		LastReadSeq:      item.LastReadSeq,
		LastDeliveredSeq: item.LastDeliveredSeq,
		UnreadCount:      item.UnreadCount,
		IsMuted:          item.IsMuted,
		IsPinned:         item.IsPinned,
	}, nil
}

// UpdateLastRead 更新已读位置
func (r *InboxRepositoryRedis) UpdateLastRead(ctx context.Context, userID, conversationID, readSeq uint64) error {
	key := r.getInboxKey(userID)
	convIDStr := strconv.FormatUint(conversationID, 10)

	_, err := updateReadSeqScript.Run(ctx, r.client, []string{key}, convIDStr, readSeq).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("update read seq failed: %w", err)
	}

	return nil
}

// UpdateLastDelivered 更新投递位置
func (r *InboxRepositoryRedis) UpdateLastDelivered(ctx context.Context, userID, conversationID, deliveredSeq uint64) error {
	inboxKey := r.getInboxKey(userID)
	convListKey := r.getConvListKey(userID)
	convIDStr := strconv.FormatUint(conversationID, 10)
	msgTime := time.Now().Unix()

	_, err := updateDeliveredSeqScript.Run(ctx, r.client,
		[]string{inboxKey, convListKey},
		convIDStr, deliveredSeq, msgTime, 1).Result() // 1 表示是自己发的
	if err != nil && err != redis.Nil {
		return fmt.Errorf("update delivered seq failed: %w", err)
	}

	return nil
}

// UpdateLastDeliveredWithUnread 更新投递位置并增加未读数（非发送者调用）
func (r *InboxRepositoryRedis) UpdateLastDeliveredWithUnread(ctx context.Context, userID, conversationID, deliveredSeq uint64) error {
	inboxKey := r.getInboxKey(userID)
	convListKey := r.getConvListKey(userID)
	convIDStr := strconv.FormatUint(conversationID, 10)
	msgTime := time.Now().Unix()

	_, err := updateDeliveredSeqScript.Run(ctx, r.client,
		[]string{inboxKey, convListKey},
		convIDStr, deliveredSeq, msgTime, 0).Result() // 0 表示不是自己发的
	if err != nil && err != redis.Nil {
		return fmt.Errorf("update delivered seq with unread failed: %w", err)
	}

	return nil
}

// IncrUnread 增加未读数
func (r *InboxRepositoryRedis) IncrUnread(ctx context.Context, userID, conversationID uint64, delta int) error {
	key := r.getInboxKey(userID)
	convIDStr := strconv.FormatUint(conversationID, 10)

	data, err := r.client.HGet(ctx, key, convIDStr).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("get inbox failed: %w", err)
	}

	var item InboxCacheItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return fmt.Errorf("unmarshal inbox failed: %w", err)
	}

	item.UnreadCount += delta
	if item.UnreadCount < 0 {
		item.UnreadCount = 0
	}

	newData, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal inbox failed: %w", err)
	}

	return r.client.HSet(ctx, key, convIDStr, string(newData)).Err()
}

// ClearUnread 清除未读数
func (r *InboxRepositoryRedis) ClearUnread(ctx context.Context, userID, conversationID uint64) error {
	key := r.getInboxKey(userID)
	convIDStr := strconv.FormatUint(conversationID, 10)

	data, err := r.client.HGet(ctx, key, convIDStr).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("get inbox failed: %w", err)
	}

	var item InboxCacheItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return fmt.Errorf("unmarshal inbox failed: %w", err)
	}

	item.UnreadCount = 0

	newData, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal inbox failed: %w", err)
	}

	return r.client.HSet(ctx, key, convIDStr, string(newData)).Err()
}

// GetUnreadCount 获取未读数
func (r *InboxRepositoryRedis) GetUnreadCount(ctx context.Context, userID, conversationID uint64) (int, error) {
	key := r.getInboxKey(userID)
	convIDStr := strconv.FormatUint(conversationID, 10)

	data, err := r.client.HGet(ctx, key, convIDStr).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("get inbox failed: %w", err)
	}

	var item InboxCacheItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return 0, fmt.Errorf("unmarshal inbox failed: %w", err)
	}

	return item.UnreadCount, nil
}

// GetRecentConversations 获取最近会话列表
func (r *InboxRepositoryRedis) GetRecentConversations(ctx context.Context, userID uint64, limit int) ([]uint64, error) {
	key := r.getConvListKey(userID)

	// 按时间倒序获取
	results, err := r.client.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("get recent conversations failed: %w", err)
	}

	convIDs := make([]uint64, 0, len(results))
	for _, s := range results {
		if convID, err := strconv.ParseUint(s, 10, 64); err == nil {
			convIDs = append(convIDs, convID)
		}
	}

	return convIDs, nil
}

// GetAllInboxes 获取用户的所有收件箱
func (r *InboxRepositoryRedis) GetAllInboxes(ctx context.Context, userID uint64) ([]*out.Inbox, error) {
	key := r.getInboxKey(userID)

	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("get all inboxes failed: %w", err)
	}

	inboxes := make([]*out.Inbox, 0, len(data))
	for _, d := range data {
		var item InboxCacheItem
		if err := json.Unmarshal([]byte(d), &item); err != nil {
			continue
		}

		inboxes = append(inboxes, &out.Inbox{
			UserID:           userID,
			ConversationID:   item.ConversationID,
			LastReadSeq:      item.LastReadSeq,
			LastDeliveredSeq: item.LastDeliveredSeq,
			UnreadCount:      item.UnreadCount,
			IsMuted:          item.IsMuted,
			IsPinned:         item.IsPinned,
		})
	}

	return inboxes, nil
}

// GetTotalUnreadCount 获取总未读数
func (r *InboxRepositoryRedis) GetTotalUnreadCount(ctx context.Context, userID uint64) (int, error) {
	inboxes, err := r.GetAllInboxes(ctx, userID)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, inbox := range inboxes {
		if !inbox.IsMuted {
			total += inbox.UnreadCount
		}
	}

	return total, nil
}

// BatchGetInboxes 批量获取收件箱
func (r *InboxRepositoryRedis) BatchGetInboxes(ctx context.Context, userID uint64, conversationIDs []uint64) (map[uint64]*out.Inbox, error) {
	if len(conversationIDs) == 0 {
		return make(map[uint64]*out.Inbox), nil
	}

	key := r.getInboxKey(userID)

	fields := make([]string, len(conversationIDs))
	for i, convID := range conversationIDs {
		fields[i] = strconv.FormatUint(convID, 10)
	}

	results, err := r.client.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, fmt.Errorf("batch get inboxes failed: %w", err)
	}

	inboxMap := make(map[uint64]*out.Inbox, len(conversationIDs))
	for i, result := range results {
		if result == nil {
			continue
		}

		data, ok := result.(string)
		if !ok {
			continue
		}

		var item InboxCacheItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			continue
		}

		inboxMap[conversationIDs[i]] = &out.Inbox{
			UserID:           userID,
			ConversationID:   item.ConversationID,
			LastReadSeq:      item.LastReadSeq,
			LastDeliveredSeq: item.LastDeliveredSeq,
			UnreadCount:      item.UnreadCount,
			IsMuted:          item.IsMuted,
			IsPinned:         item.IsPinned,
		}
	}

	return inboxMap, nil
}

// GetTotalUnread 获取总未读数（实现 InboxRepository 接口）
func (r *InboxRepositoryRedis) GetTotalUnread(ctx context.Context, userID uint64) (int, error) {
	return r.GetTotalUnreadCount(ctx, userID)
}

// GetUserInboxes 获取用户的所有收件箱（实现 InboxRepository 接口）
func (r *InboxRepositoryRedis) GetUserInboxes(ctx context.Context, userID uint64) ([]*entity.Inbox, error) {
	inboxes, err := r.GetAllInboxes(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]*entity.Inbox, len(inboxes))
	for i, inbox := range inboxes {
		result[i] = &entity.Inbox{
			UserID:           inbox.UserID,
			ConversationID:   inbox.ConversationID,
			LastReadSeq:      inbox.LastReadSeq,
			LastDeliveredSeq: inbox.LastDeliveredSeq,
			UnreadCount:      inbox.UnreadCount,
			IsMuted:          inbox.IsMuted,
			IsPinned:         inbox.IsPinned,
		}
	}

	return result, nil
}
