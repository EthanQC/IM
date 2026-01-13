package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

const (
	// 死信队列 Topic
	TopicDeadLetter = "im.message.dead_letter"
	// 重试队列 Topic
	TopicRetry = "im.message.retry"

	// 最大重试次数
	MaxRetryCount = 3
	// 重试间隔基数（指数退避）
	RetryBaseInterval = time.Second * 5
)

// DeadLetterMessage 死信消息
type DeadLetterMessage struct {
	OriginalTopic string          `json:"original_topic"`
	OriginalKey   string          `json:"original_key"`
	Payload       json.RawMessage `json:"payload"`
	ErrorMsg      string          `json:"error_msg"`
	RetryCount    int             `json:"retry_count"`
	CreatedAt     int64           `json:"created_at"`
	LastRetryAt   int64           `json:"last_retry_at"`
}

// RetryMessage 重试消息
type RetryMessage struct {
	OriginalTopic string          `json:"original_topic"`
	OriginalKey   string          `json:"original_key"`
	Payload       json.RawMessage `json:"payload"`
	RetryCount    int             `json:"retry_count"`
	NextRetryAt   int64           `json:"next_retry_at"`
}

// ReliableKafkaConsumer 可靠的Kafka消费者（带重试和死信队列）
type ReliableKafkaConsumer struct {
	consumerGroup   sarama.ConsumerGroup
	producer        sarama.SyncProducer
	topics          []string
	deliveryUseCase in.DeliveryUseCase
	ready           chan bool
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// NewReliableKafkaConsumer 创建可靠的Kafka消费者
func NewReliableKafkaConsumer(brokers []string, groupID string, deliveryUseCase in.DeliveryUseCase) (out.MessageConsumer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Return.Errors = true
	// 手动提交offset，确保消息处理完成后才提交
	config.Consumer.Offsets.AutoCommit.Enable = false

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("create consumer group failed: %w", err)
	}

	// 创建Producer用于发送重试和死信消息
	producerConfig := sarama.NewConfig()
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Retry.Max = 3

	producer, err := sarama.NewSyncProducer(brokers, producerConfig)
	if err != nil {
		consumerGroup.Close()
		return nil, fmt.Errorf("create producer failed: %w", err)
	}

	return &ReliableKafkaConsumer{
		consumerGroup:   consumerGroup,
		producer:        producer,
		topics:          []string{TopicMessageNew, TopicMessageRead, TopicMessageRevoked, TopicRetry},
		deliveryUseCase: deliveryUseCase,
		ready:           make(chan bool),
	}, nil
}

// Start 启动消费
func (c *ReliableKafkaConsumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	handler := &reliableConsumerHandler{
		deliveryUseCase: c.deliveryUseCase,
		producer:        c.producer,
		ready:           c.ready,
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
					zap.L().Warn("Error from consumer", zap.Error(err))
				}
				// 重置ready channel
				c.ready = make(chan bool)
				handler.ready = c.ready
			}
		}
	}()

	// 启动重试消息调度器
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.retryScheduler(ctx)
	}()

	// 等待消费者准备就绪
	<-c.ready
	zap.L().Info("Reliable Kafka consumer is ready")

	return nil
}

// Stop 停止消费
func (c *ReliableKafkaConsumer) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	if err := c.producer.Close(); err != nil {
		zap.L().Warn("Close producer error", zap.Error(err))
	}

	return c.consumerGroup.Close()
}

// retryScheduler 重试调度器
func (c *ReliableKafkaConsumer) retryScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 这里可以从Redis或其他存储中获取待重试的消息
			// 当前实现依赖于Kafka retry topic的延迟消费
		}
	}
}

// reliableConsumerHandler 可靠消费组处理器
type reliableConsumerHandler struct {
	deliveryUseCase in.DeliveryUseCase
	producer        sarama.SyncProducer
	ready           chan bool
}

func (h *reliableConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *reliableConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *reliableConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			// 处理消息，带重试逻辑
			if err := h.handleMessageWithRetry(session.Context(), message); err != nil {
				zap.L().Warn("Message handling failed after retries", zap.Error(err))
			}

			// 手动提交offset
			session.MarkMessage(message, "")
			session.Commit()

		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *reliableConsumerHandler) handleMessageWithRetry(ctx context.Context, message *sarama.ConsumerMessage) error {
	var retryCount int
	var originalTopic string
	var originalKey string
	var payload []byte

	// 检查是否是重试消息
	if message.Topic == TopicRetry {
		var retryMsg RetryMessage
		if err := json.Unmarshal(message.Value, &retryMsg); err != nil {
			return h.sendToDeadLetter(message.Topic, string(message.Key), message.Value, err.Error(), 0)
		}

		// 检查是否到达重试时间
		if time.Now().Unix() < retryMsg.NextRetryAt {
			// 还未到重试时间，重新发送到重试队列
			return h.sendToRetry(retryMsg.OriginalTopic, retryMsg.OriginalKey, retryMsg.Payload, retryMsg.RetryCount)
		}

		retryCount = retryMsg.RetryCount
		originalTopic = retryMsg.OriginalTopic
		originalKey = retryMsg.OriginalKey
		payload = retryMsg.Payload
	} else {
		retryCount = 0
		originalTopic = message.Topic
		originalKey = string(message.Key)
		payload = message.Value
	}

	// 尝试处理消息
	err := h.processMessage(ctx, originalTopic, payload)
	if err != nil {
		retryCount++

		if retryCount >= MaxRetryCount {
			// 达到最大重试次数，发送到死信队列
			return h.sendToDeadLetter(originalTopic, originalKey, payload, err.Error(), retryCount)
		}

		// 发送到重试队列
		return h.sendToRetry(originalTopic, originalKey, payload, retryCount)
	}

	return nil
}

func (h *reliableConsumerHandler) processMessage(ctx context.Context, topic string, payload []byte) error {
	switch topic {
	case TopicMessageNew:
		return h.handleNewMessage(ctx, payload)
	case TopicMessageRead:
		return h.handleMessageRead(ctx, payload)
	case TopicMessageRevoked:
		return h.handleMessageRevoked(ctx, payload)
	default:
		return fmt.Errorf("unknown topic: %s", topic)
	}
}

func (h *reliableConsumerHandler) sendToRetry(originalTopic, originalKey string, payload []byte, retryCount int) error {
	// 计算下次重试时间（指数退避）
	delay := RetryBaseInterval * time.Duration(1<<uint(retryCount))
	nextRetryAt := time.Now().Add(delay).Unix()

	retryMsg := RetryMessage{
		OriginalTopic: originalTopic,
		OriginalKey:   originalKey,
		Payload:       payload,
		RetryCount:    retryCount,
		NextRetryAt:   nextRetryAt,
	}

	data, err := json.Marshal(retryMsg)
	if err != nil {
		return fmt.Errorf("marshal retry message failed: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: TopicRetry,
		Key:   sarama.StringEncoder(originalKey),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("retry_count"), Value: []byte(fmt.Sprintf("%d", retryCount))},
			{Key: []byte("next_retry_at"), Value: []byte(fmt.Sprintf("%d", nextRetryAt))},
		},
	}

	_, _, err = h.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send to retry queue failed: %w", err)
	}

	zap.L().Info("Message sent to retry queue",
		zap.String("topic", originalTopic),
		zap.String("key", originalKey),
		zap.Int("retryCount", retryCount),
		zap.Int64("nextRetryAt", nextRetryAt))

	return nil
}

func (h *reliableConsumerHandler) sendToDeadLetter(originalTopic, originalKey string, payload []byte, errorMsg string, retryCount int) error {
	dlMsg := DeadLetterMessage{
		OriginalTopic: originalTopic,
		OriginalKey:   originalKey,
		Payload:       payload,
		ErrorMsg:      errorMsg,
		RetryCount:    retryCount,
		CreatedAt:     time.Now().Unix(),
		LastRetryAt:   time.Now().Unix(),
	}

	data, err := json.Marshal(dlMsg)
	if err != nil {
		return fmt.Errorf("marshal dead letter message failed: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: TopicDeadLetter,
		Key:   sarama.StringEncoder(originalKey),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("original_topic"), Value: []byte(originalTopic)},
			{Key: []byte("error_msg"), Value: []byte(errorMsg)},
		},
	}

	_, _, err = h.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send to dead letter queue failed: %w", err)
	}

	zap.L().Warn("Message sent to dead letter queue",
		zap.String("topic", originalTopic),
		zap.String("key", originalKey),
		zap.String("error", errorMsg))

	return nil
}

func (h *reliableConsumerHandler) handleNewMessage(ctx context.Context, data []byte) error {
	var event struct {
		MessageID      uint64   `json:"message_id"`
		ConversationID uint64   `json:"conversation_id"`
		SenderID       uint64   `json:"sender_id"`
		ReceiverIDs    []uint64 `json:"receiver_ids"`
		Seq            uint64   `json:"seq"`
		ContentType    int8     `json:"content_type"`
		Content        string   `json:"content"`
		CreatedAt      int64    `json:"created_at"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal new message event failed: %w", err)
	}

	msgEvent := &entity.MessageEvent{
		Type:           "new_message",
		MessageID:      event.MessageID,
		ConversationID: event.ConversationID,
		SenderID:       event.SenderID,
		ReceiverIDs:    event.ReceiverIDs,
		Seq:            event.Seq,
		ContentType:    event.ContentType,
		Content:        event.Content,
		CreatedAt:      time.Unix(event.CreatedAt, 0),
	}

	return h.deliveryUseCase.DeliverMessage(ctx, msgEvent)
}

func (h *reliableConsumerHandler) handleMessageRead(ctx context.Context, data []byte) error {
	var event struct {
		ConversationID uint64   `json:"conversation_id"`
		UserID         uint64   `json:"user_id"`
		ReceiverIDs    []uint64 `json:"receiver_ids"`
		ReadSeq        uint64   `json:"read_seq"`
		ReadAt         int64    `json:"read_at"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal message read event failed: %w", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type": "message_read",
		"data": map[string]interface{}{
			"conversation_id": event.ConversationID,
			"user_id":         event.UserID,
			"read_seq":        event.ReadSeq,
			"read_at":         event.ReadAt,
		},
	})

	var lastErr error
	for _, receiverID := range event.ReceiverIDs {
		if receiverID == event.UserID {
			continue
		}
		if err := h.deliveryUseCase.DeliverToUser(ctx, receiverID, payload); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

func (h *reliableConsumerHandler) handleMessageRevoked(ctx context.Context, data []byte) error {
	var event struct {
		MessageID      uint64   `json:"message_id"`
		ConversationID uint64   `json:"conversation_id"`
		SenderID       uint64   `json:"sender_id"`
		ReceiverIDs    []uint64 `json:"receiver_ids"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal message revoked event failed: %w", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type": "message_revoked",
		"data": map[string]interface{}{
			"conversation_id": event.ConversationID,
			"message_id":      event.MessageID,
			"sender_id":       event.SenderID,
		},
	})

	msgEvent := &entity.MessageEvent{
		Type:           "message_revoked",
		MessageID:      event.MessageID,
		ConversationID: event.ConversationID,
		SenderID:       event.SenderID,
		ReceiverIDs:    event.ReceiverIDs,
		Content:        string(payload),
	}

	return h.deliveryUseCase.DeliverMessage(ctx, msgEvent)
}
