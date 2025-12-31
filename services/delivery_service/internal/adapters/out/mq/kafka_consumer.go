package mq

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/IBM/sarama"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

const (
	TopicMessageNew     = "im.message.new"
	TopicMessageRead    = "im.message.read"
	TopicMessageRevoked = "im.message.revoked"
)

// KafkaMessageConsumer Kafka消息消费者
type KafkaMessageConsumer struct {
	consumerGroup sarama.ConsumerGroup
	topics        []string
	deliveryUseCase in.DeliveryUseCase
	ready         chan bool
	cancel        context.CancelFunc
}

// NewKafkaMessageConsumer 创建Kafka消息消费者
func NewKafkaMessageConsumer(brokers []string, groupID string, deliveryUseCase in.DeliveryUseCase) (out.MessageConsumer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Return.Errors = true

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, err
	}

	return &KafkaMessageConsumer{
		consumerGroup:   consumerGroup,
		topics:          []string{TopicMessageNew, TopicMessageRead, TopicMessageRevoked},
		deliveryUseCase: deliveryUseCase,
		ready:           make(chan bool),
	}, nil
}

// Start 启动消费
func (c *KafkaMessageConsumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	handler := &consumerGroupHandler{
		deliveryUseCase: c.deliveryUseCase,
		ready:           c.ready,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
					log.Printf("Error from consumer: %v", err)
				}
				// 重置ready channel
				c.ready = make(chan bool)
				handler.ready = c.ready
			}
		}
	}()

	// 等待消费者准备就绪
	<-c.ready
	log.Println("Kafka consumer is ready")

	return nil
}

// Stop 停止消费
func (c *KafkaMessageConsumer) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	return c.consumerGroup.Close()
}

// consumerGroupHandler 消费组处理器
type consumerGroupHandler struct {
	deliveryUseCase in.DeliveryUseCase
	ready           chan bool
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.handleMessage(session.Context(), message)
			session.MarkMessage(message, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *consumerGroupHandler) handleMessage(ctx context.Context, message *sarama.ConsumerMessage) {
	switch message.Topic {
	case TopicMessageNew:
		h.handleNewMessage(ctx, message.Value)
	case TopicMessageRead:
		h.handleMessageRead(ctx, message.Value)
	case TopicMessageRevoked:
		h.handleMessageRevoked(ctx, message.Value)
	default:
		log.Printf("Unknown topic: %s", message.Topic)
	}
}

func (h *consumerGroupHandler) handleNewMessage(ctx context.Context, data []byte) {
	var event struct {
		MessageID      uint64    `json:"message_id"`
		ConversationID uint64    `json:"conversation_id"`
		SenderID       uint64    `json:"sender_id"`
		ReceiverIDs    []uint64  `json:"receiver_ids"`
		Seq            uint64    `json:"seq"`
		ContentType    int8      `json:"content_type"`
		Content        string    `json:"content"`
		CreatedAt      time.Time `json:"created_at"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal new message event: %v", err)
		return
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
		CreatedAt:      event.CreatedAt,
	}

	if err := h.deliveryUseCase.DeliverMessage(ctx, msgEvent); err != nil {
		log.Printf("Failed to deliver message: %v", err)
	}
}

func (h *consumerGroupHandler) handleMessageRead(ctx context.Context, data []byte) {
	var event struct {
		ConversationID uint64 `json:"conversation_id"`
		UserID         uint64 `json:"user_id"`
		ReadSeq        uint64 `json:"read_seq"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal message read event: %v", err)
		return
	}

	// 构建已读通知并投递
	payload, _ := json.Marshal(map[string]interface{}{
		"type": "message_read",
		"data": map[string]interface{}{
			"conversation_id": event.ConversationID,
			"user_id":         event.UserID,
			"read_seq":        event.ReadSeq,
		},
	})

	// TODO: 获取会话中的其他用户并投递
	if err := h.deliveryUseCase.DeliverToUser(ctx, event.UserID, payload); err != nil {
		log.Printf("Failed to deliver read receipt: %v", err)
	}
}

func (h *consumerGroupHandler) handleMessageRevoked(ctx context.Context, data []byte) {
	var event struct {
		MessageID      uint64   `json:"message_id"`
		ConversationID uint64   `json:"conversation_id"`
		SenderID       uint64   `json:"sender_id"`
		ReceiverIDs    []uint64 `json:"receiver_ids"`
	}

	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal message revoked event: %v", err)
		return
	}

	// 构建撤回通知
	payload, _ := json.Marshal(map[string]interface{}{
		"type": "message_revoked",
		"data": map[string]interface{}{
			"conversation_id": event.ConversationID,
			"message_id":      event.MessageID,
			"sender_id":       event.SenderID,
		},
	})

	// 投递给所有接收者
	msgEvent := &entity.MessageEvent{
		Type:           "message_revoked",
		MessageID:      event.MessageID,
		ConversationID: event.ConversationID,
		SenderID:       event.SenderID,
		ReceiverIDs:    event.ReceiverIDs,
		Content:        string(payload),
	}

	if err := h.deliveryUseCase.DeliverMessage(ctx, msgEvent); err != nil {
		log.Printf("Failed to deliver revoke notification: %v", err)
	}
}
