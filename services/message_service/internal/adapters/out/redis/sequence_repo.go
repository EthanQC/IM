package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

const (
	// 序号Key前缀
	seqKeyPrefix = "im:seq:conv:"
)

// SequenceRepositoryRedis Redis序号仓储实现
// 使用Lua脚本保证原子性，实现单调递增的序号生成
type SequenceRepositoryRedis struct {
	client *redis.Client
}

// 确保实现接口
var _ out.SequenceRepository = (*SequenceRepositoryRedis)(nil)

// Lua脚本：原子递增并返回新序号
var incrSeqScript = redis.NewScript(`
local key = KEYS[1]
local current = redis.call('GET', key)
if not current then
    current = 0
end
local next_seq = tonumber(current) + 1
redis.call('SET', key, next_seq)
return next_seq
`)

// Lua脚本：批量获取并递增序号
var batchGetSeqScript = redis.NewScript(`
local results = {}
for i, key in ipairs(KEYS) do
    local current = redis.call('GET', key)
    if not current then
        current = 0
    end
    local next_seq = tonumber(current) + 1
    redis.call('SET', key, next_seq)
    results[i] = next_seq
end
return results
`)

// Lua脚本：初始化序号（仅当不存在时）
var initSeqScript = redis.NewScript(`
local key = KEYS[1]
local init_value = ARGV[1]
local current = redis.call('GET', key)
if not current then
    redis.call('SET', key, init_value)
    return init_value
end
return current
`)

func NewSequenceRepositoryRedis(client *redis.Client) *SequenceRepositoryRedis {
	return &SequenceRepositoryRedis{client: client}
}

func (r *SequenceRepositoryRedis) getKey(conversationID uint64) string {
	return fmt.Sprintf("%s%d", seqKeyPrefix, conversationID)
}

// GetNextSeq 获取并递增下一个序号（原子操作）
// 使用Lua脚本保证并发安全和原子性
func (r *SequenceRepositoryRedis) GetNextSeq(ctx context.Context, conversationID uint64) (uint64, error) {
	key := r.getKey(conversationID)

	result, err := incrSeqScript.Run(ctx, r.client, []string{key}).Result()
	if err != nil {
		return 0, fmt.Errorf("get next seq failed: %w", err)
	}

	seq, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("invalid seq result type")
	}

	return uint64(seq), nil
}

// GetCurrentSeq 获取当前序号
func (r *SequenceRepositoryRedis) GetCurrentSeq(ctx context.Context, conversationID uint64) (uint64, error) {
	key := r.getKey(conversationID)

	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("get current seq failed: %w", err)
	}

	seq, err := strconv.ParseUint(result, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse seq failed: %w", err)
	}

	return seq, nil
}

// BatchGetNextSeqs 批量获取下一个序号
func (r *SequenceRepositoryRedis) BatchGetNextSeqs(ctx context.Context, conversationIDs []uint64) (map[uint64]uint64, error) {
	if len(conversationIDs) == 0 {
		return make(map[uint64]uint64), nil
	}

	keys := make([]string, len(conversationIDs))
	for i, convID := range conversationIDs {
		keys[i] = r.getKey(convID)
	}

	result, err := batchGetSeqScript.Run(ctx, r.client, keys).Result()
	if err != nil {
		return nil, fmt.Errorf("batch get next seqs failed: %w", err)
	}

	seqs, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid batch result type")
	}

	seqMap := make(map[uint64]uint64, len(conversationIDs))
	for i, convID := range conversationIDs {
		if i < len(seqs) {
			if seq, ok := seqs[i].(int64); ok {
				seqMap[convID] = uint64(seq)
			}
		}
	}

	return seqMap, nil
}

// InitSeq 初始化序号（用于从MySQL同步时）
func (r *SequenceRepositoryRedis) InitSeq(ctx context.Context, conversationID uint64, seq uint64) error {
	key := r.getKey(conversationID)

	_, err := initSeqScript.Run(ctx, r.client, []string{key}, seq).Result()
	if err != nil {
		return fmt.Errorf("init seq failed: %w", err)
	}

	return nil
}

// SetSeq 强制设置序号（慎用）
func (r *SequenceRepositoryRedis) SetSeq(ctx context.Context, conversationID uint64, seq uint64) error {
	key := r.getKey(conversationID)

	return r.client.Set(ctx, key, seq, 0).Err()
}
