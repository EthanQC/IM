package service

import (
	"context"
	"sync"
)

// DiffusionStrategy 消息扩散策略
type DiffusionStrategy string

const (
	// WriteDiffusion 写扩散：消息写入到所有接收者的收件箱
	WriteDiffusion DiffusionStrategy = "write"
	// ReadDiffusion 读扩散：消息只写入发送者的发件箱，接收者读取时聚合
	ReadDiffusion DiffusionStrategy = "read"
)

// MessageDistributor 消息分发器
type MessageDistributor struct {
	groupMemberThreshold int // 群聊人数阈值，超过则使用读扩散
	mu                   sync.RWMutex
}

// NewMessageDistributor 创建消息分发器
func NewMessageDistributor(threshold int) *MessageDistributor {
	if threshold <= 0 {
		threshold = 500
	}
	return &MessageDistributor{
		groupMemberThreshold: threshold,
	}
}

// DistributionPlan 分发计划
type DistributionPlan struct {
	Strategy   DiffusionStrategy
	Recipients []string // 写扩散时的接收者列表
	GroupID    string   // 读扩散时的群组ID
	MessageID  string
}

// DetermineStrategy 根据接收者数量确定扩散策略
func (d *MessageDistributor) DetermineStrategy(recipientCount int) DiffusionStrategy {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if recipientCount > d.groupMemberThreshold {
		return ReadDiffusion
	}
	return WriteDiffusion
}

// PlanDistribution 规划消息分发
func (d *MessageDistributor) PlanDistribution(
	ctx context.Context,
	messageID string,
	senderID string,
	recipientIDs []string,
	groupID string,
) *DistributionPlan {
	strategy := d.DetermineStrategy(len(recipientIDs))

	plan := &DistributionPlan{
		Strategy:  strategy,
		MessageID: messageID,
		GroupID:   groupID,
	}

	if strategy == WriteDiffusion {
		plan.Recipients = recipientIDs
	}

	return plan
}

// ExecuteWriteDiffusion 执行写扩散
func (d *MessageDistributor) ExecuteWriteDiffusion(
	ctx context.Context,
	plan *DistributionPlan,
	writeFunc func(ctx context.Context, userID string, messageID string) error,
) []error {
	var (
		errs []error
		mu   sync.Mutex
		wg   sync.WaitGroup
	)

	// 并发写入每个接收者的收件箱
	sem := make(chan struct{}, 50) // 并发限制
	for _, userID := range plan.Recipients {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := writeFunc(ctx, uid, plan.MessageID); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(userID)
	}
	wg.Wait()

	return errs
}

// SetThreshold 动态调整阈值
func (d *MessageDistributor) SetThreshold(threshold int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groupMemberThreshold = threshold
}

// GetThreshold 获取当前阈值
func (d *MessageDistributor) GetThreshold() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.groupMemberThreshold
}
