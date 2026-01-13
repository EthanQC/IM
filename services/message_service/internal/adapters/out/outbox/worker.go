package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

// WorkerConfig Outbox Worker 配置
type WorkerConfig struct {
	PollInterval   time.Duration
	BatchSize      int
	MaxRetries     int
	CleanupAfter   time.Duration
	WorkerCount    int
}

// DefaultWorkerConfig 默认配置
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		PollInterval:   100 * time.Millisecond,
		BatchSize:      100,
		MaxRetries:     5,
		CleanupAfter:   7 * 24 * time.Hour,
		WorkerCount:    2,
	}
}

// Worker Outbox 异步投递 Worker
type Worker struct {
	config     WorkerConfig
	outboxRepo out.OutboxRepository
	publisher  out.EventPublisher
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
	running    bool
}

// NewWorker 创建 Outbox Worker
func NewWorker(
	outboxRepo out.OutboxRepository,
	publisher out.EventPublisher,
	config WorkerConfig,
) *Worker {
	return &Worker{
		config:     config,
		outboxRepo: outboxRepo,
		publisher:  publisher,
	}
}

// Start 启动 Worker
func (w *Worker) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("worker already running")
	}
	w.running = true
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.mu.Unlock()

	for i := 0; i < w.config.WorkerCount; i++ {
		w.wg.Add(1)
		go w.pollLoop(i)
	}

	w.wg.Add(1)
	go w.cleanupLoop()

	zap.L().Info("Outbox worker started", zap.Int("workerCount", w.config.WorkerCount))
	return nil
}

// Stop 停止 Worker
func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	zap.L().Info("Outbox worker stopped")
}

// pollLoop 轮询循环
func (w *Worker) pollLoop(workerID int) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.processBatch(); err != nil {
				zap.L().Warn("Worker process batch error",
					zap.Int("workerID", workerID),
					zap.Error(err))
			}
		}
	}
}

// processBatch 处理一批待发布事件
func (w *Worker) processBatch() error {
	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	events, err := w.outboxRepo.GetPendingEvents(ctx, w.config.BatchSize)
	if err != nil {
		return fmt.Errorf("get pending events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		if err := w.processEvent(ctx, event); err != nil {
			zap.L().Warn("Process event failed",
				zap.Uint64("eventID", event.ID),
				zap.Error(err))
		}
	}

	return nil
}

// processEvent 处理单个事件
func (w *Worker) processEvent(ctx context.Context, event *out.OutboxEvent) error {
	var err error
	switch event.EventType {
	case "message.sent":
		err = w.publishMessageSent(ctx, event)
	case "message.read":
		err = w.publishMessageRead(ctx, event)
	case "message.revoked":
		err = w.publishMessageRevoked(ctx, event)
	default:
		zap.L().Warn("Unknown event type", zap.String("eventType", event.EventType))
		return nil
	}

	if err != nil {
		if incrErr := w.outboxRepo.IncrRetryCount(ctx, event.ID); incrErr != nil {
			zap.L().Warn("Incr retry count failed", zap.Error(incrErr))
		}

		if event.RetryCount >= w.config.MaxRetries {
			if markErr := w.outboxRepo.MarkAsFailed(ctx, event.ID, err.Error()); markErr != nil {
				zap.L().Warn("Mark as failed error", zap.Error(markErr))
			}
			return fmt.Errorf("max retries exceeded: %w", err)
		}

		return err
	}

	if err := w.outboxRepo.MarkAsPublished(ctx, event.ID); err != nil {
		return fmt.Errorf("mark as published: %w", err)
	}

	return nil
}

func (w *Worker) publishMessageSent(ctx context.Context, event *out.OutboxEvent) error {
	var msgEvent out.MessageSentEvent
	if err := json.Unmarshal(event.Payload, &msgEvent); err != nil {
		return fmt.Errorf("unmarshal message sent event: %w", err)
	}
	return w.publisher.PublishMessageSent(ctx, &msgEvent)
}

func (w *Worker) publishMessageRead(ctx context.Context, event *out.OutboxEvent) error {
	var readEvent out.MessageReadEvent
	if err := json.Unmarshal(event.Payload, &readEvent); err != nil {
		return fmt.Errorf("unmarshal message read event: %w", err)
	}
	return w.publisher.PublishMessageRead(ctx, &readEvent)
}

func (w *Worker) publishMessageRevoked(ctx context.Context, event *out.OutboxEvent) error {
	var revokeEvent out.MessageRevokedEvent
	if err := json.Unmarshal(event.Payload, &revokeEvent); err != nil {
		return fmt.Errorf("unmarshal message revoked event: %w", err)
	}
	return w.publisher.PublishMessageRevoked(ctx, &revokeEvent)
}

func (w *Worker) cleanupLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.cleanup(); err != nil {
				zap.L().Warn("Cleanup error", zap.Error(err))
			}
		}
	}
}

func (w *Worker) cleanup() error {
	ctx, cancel := context.WithTimeout(w.ctx, time.Minute)
	defer cancel()

	before := time.Now().Add(-w.config.CleanupAfter)
	deleted, err := w.outboxRepo.DeletePublished(ctx, before)
	if err != nil {
		return fmt.Errorf("delete published events: %w", err)
	}

	if deleted > 0 {
		zap.L().Info("Cleaned up old outbox events", zap.Int64("count", deleted))
	}

	return nil
}

func (w *Worker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}
