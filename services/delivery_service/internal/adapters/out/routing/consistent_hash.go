package routing

import (
	"fmt"
	"hash/crc32"
	"sort"
	"sync"
)

// ConsistentHash 一致性哈希环
type ConsistentHash struct {
	replicas int               // 虚拟节点数
	keys     []uint32          // 已排序的哈希值
	hashMap  map[uint32]string // 哈希值到节点的映射
	mu       sync.RWMutex
}

// NewConsistentHash 创建一致性哈希环
func NewConsistentHash(replicas int) *ConsistentHash {
	if replicas <= 0 {
		replicas = 150
	}
	return &ConsistentHash{
		replicas: replicas,
		hashMap:  make(map[uint32]string),
	}
}

// hash 计算哈希值
func (ch *ConsistentHash) hash(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

// AddNode 添加节点
func (ch *ConsistentHash) AddNode(node string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		key := ch.hash(fmt.Sprintf("%s#%d", node, i))
		ch.keys = append(ch.keys, key)
		ch.hashMap[key] = node
	}
	sort.Slice(ch.keys, func(i, j int) bool {
		return ch.keys[i] < ch.keys[j]
	})
}

// RemoveNode 移除节点
func (ch *ConsistentHash) RemoveNode(node string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		key := ch.hash(fmt.Sprintf("%s#%d", node, i))
		delete(ch.hashMap, key)
	}

	newKeys := make([]uint32, 0, len(ch.keys)-ch.replicas)
	for _, k := range ch.keys {
		if _, ok := ch.hashMap[k]; ok {
			newKeys = append(newKeys, k)
		}
	}
	ch.keys = newKeys
}

// GetNode 获取负责给定 key 的节点
func (ch *ConsistentHash) GetNode(key string) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.keys) == 0 {
		return ""
	}

	hash := ch.hash(key)
	idx := sort.Search(len(ch.keys), func(i int) bool {
		return ch.keys[i] >= hash
	})

	if idx >= len(ch.keys) {
		idx = 0
	}

	return ch.hashMap[ch.keys[idx]]
}

// GetNodes 获取所有节点
func (ch *ConsistentHash) GetNodes() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	nodeSet := make(map[string]struct{})
	for _, node := range ch.hashMap {
		nodeSet[node] = struct{}{}
	}

	nodes := make([]string, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}
	return nodes
}

// NodeCount 返回节点数量
func (ch *ConsistentHash) NodeCount() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	nodeSet := make(map[string]struct{})
	for _, node := range ch.hashMap {
		nodeSet[node] = struct{}{}
	}
	return len(nodeSet)
}
