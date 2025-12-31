package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"

	"github.com/EthanQC/IM/services/message_service/internal/ports/out"
)

const (
	// Kafka Topic 定义
	TopicMessageNew     = "im.message.new"
	TopicMessageRead    = "im.message.read"
	TopicMessageRevoked = "im.message.revoked"
)

// KafkaEventPublisher Kafka事件发布器
type KafkaEventPublisher struct {
	producer sarama.SyncProducer
}

// NewKafkaEventPublisher 创建Kafka事件发布器
func NewKafkaEventPublisher(brokers []string) (out.EventPublisher, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3
	config.Producer.Timeout = 10 * time.Second
	// 确保消息顺序性 - 相同会话的消息发到同一分区
	config.Producer.Partitioner = sarama.NewHashPartitioner

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	return &KafkaEventPublisher{producer: producer}, nil
}

func (p *KafkaEventPublisher) PublishNewMessage(ctx context.Context, event *out.NewMessageEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal new message event failed: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: TopicMessageNew,
		Key:   sarama.StringEncoder(fmt.Sprintf("%d", event.ConversationID)), // 按会话分区
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("event_type"), Value: []byte("new_message")},
			{Key: []byte("timestamp"), Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		},
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("publish new message event failed: %w", err)
	}

	return nil
}

func (p *KafkaEventPublisher) PublishMessageRead(ctx context.Context, event *out.MessageReadEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal message read event failed: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: TopicMessageRead,
		Key:   sarama.StringEncoder(fmt.Sprintf("%d", event.ConversationID)),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("event_type"), Value: []byte("message_read")},
			{Key: []byte("timestamp"), Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		},
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("publish message read event failed: %w", err)
	}

	return nil
}

func (p *KafkaEventPublisher) PublishMessageRevoked(ctx context.Context, event *out.MessageRevokedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal message revoked event failed: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: TopicMessageRevoked,
		Key:   sarama.StringEncoder(fmt.Sprintf("%d", event.ConversationID)),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("event_type"), Value: []byte("message_revoked")},
			{Key: []byte("timestamp"), Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		},
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("publish message revoked event failed: %w", err)
	}

	return nil
}

func (p *KafkaEventPublisher) Close() error {
	return p.producer.Close()
}
