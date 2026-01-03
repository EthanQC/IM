package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

const (
	// 同步状态Key前缀
	syncStateKeyPrefix = "im:sync:state:"
	// 同步状态过期时间
	syncStateTTL = 7 * 24 * time.Hour
)

// Lua脚本：原子性更新ACK序号
var updateAckSeqScript = redis.NewScript(`
local key = KEYS[1]
local conv_id = ARGV[1]
local new_seq = tonumber(ARGV[2])

local data = redis.call('GET', key)
local state

if not data then
    state = {
        user_id = tonumber(ARGV[3]),
        conversation_ack_seqs = {},
        last_sync_at = 0
    }
else
    state = cjson.decode(data)
    if not state.conversation_ack_seqs then
        state.conversation_ack_seqs = {}
    end
end

local current_seq = state.conversation_ack_seqs[conv_id] or 0
if new_seq > current_seq then
    state.conversation_ack_seqs[conv_id] = new_seq
    redis.call('SET', key, cjson.encode(state))
    redis.call('EXPIRE', key, 604800) -- 7 days
end

return new_seq
`)

// SyncStateRepositoryRedis Redis同步状态仓储实现
type SyncStateRepositoryRedis struct {
	client *redis.Client
}

func NewSyncStateRepositoryRedis(client *redis.Client) out.SyncStateRepository {
	return &SyncStateRepositoryRedis{client: client}
}

func (r *SyncStateRepositoryRedis) getSyncStateKey(userID uint64) string {
	return fmt.Sprintf("%s%d", syncStateKeyPrefix, userID)
}

// GetSyncState 获取同步状态
func (r *SyncStateRepositoryRedis) GetSyncState(ctx context.Context, userID uint64) (*entity.SyncState, error) {
	key := r.getSyncStateKey(userID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get sync state failed: %w", err)
	}

	var state entity.SyncState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, fmt.Errorf("unmarshal sync state failed: %w", err)
	}

	return &state, nil
}

// UpdateAckSeq 更新ACK序号
func (r *SyncStateRepositoryRedis) UpdateAckSeq(ctx context.Context, userID, conversationID, ackSeq uint64) error {
	key := r.getSyncStateKey(userID)
	convIDStr := strconv.FormatUint(conversationID, 10)

	_, err := updateAckSeqScript.Run(ctx, r.client, []string{key}, convIDStr, ackSeq, userID).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("update ack seq failed: %w", err)
	}

	return nil
}

// UpdateLastSyncTime 更新最后同步时间
func (r *SyncStateRepositoryRedis) UpdateLastSyncTime(ctx context.Context, userID uint64, syncTime int64) error {
	key := r.getSyncStateKey(userID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get sync state failed: %w", err)
	}

	var state entity.SyncState
	if err == redis.Nil || data == "" {
		state = entity.SyncState{
			UserID:              userID,
			ConversationAckSeqs: make(map[uint64]uint64),
			LastSyncAt:          syncTime,
		}
	} else {
		if err := json.Unmarshal([]byte(data), &state); err != nil {
			return fmt.Errorf("unmarshal sync state failed: %w", err)
		}
		state.LastSyncAt = syncTime
	}

	newData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal sync state failed: %w", err)
	}

	return r.client.Set(ctx, key, string(newData), syncStateTTL).Err()
}

// BatchUpdateAckSeqs 批量更新ACK序号
func (r *SyncStateRepositoryRedis) BatchUpdateAckSeqs(ctx context.Context, userID uint64, ackSeqs map[uint64]uint64) error {
	key := r.getSyncStateKey(userID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get sync state failed: %w", err)
	}

	var state entity.SyncState
	if err == redis.Nil || data == "" {
		state = entity.SyncState{
			UserID:              userID,
			ConversationAckSeqs: make(map[uint64]uint64),
			LastSyncAt:          0,
		}
	} else {
		if err := json.Unmarshal([]byte(data), &state); err != nil {
			return fmt.Errorf("unmarshal sync state failed: %w", err)
		}
	}

	// 更新ACK序号（只更新更大的值）
	for convID, seq := range ackSeqs {
		if existingSeq, ok := state.ConversationAckSeqs[convID]; !ok || seq > existingSeq {
			state.ConversationAckSeqs[convID] = seq
		}
	}

	newData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal sync state failed: %w", err)
	}

	return r.client.Set(ctx, key, string(newData), syncStateTTL).Err()
}
