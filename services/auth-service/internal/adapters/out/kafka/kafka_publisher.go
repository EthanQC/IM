package kafka

import (
	"context"

	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"github.com/segmentio/kafka-go"
)

// KafkaPublisher 使用 segmentio/kafka-go 实现 EventPublisher
type KafkaPublisher struct {
	Writer *kafka.Writer
}

// NewKafkaPublisher 创建一个 KafkaPublisher
func NewKafkaPublisher(w *kafka.Writer) out.EventPublisher {
	return &KafkaPublisher{Writer: w}
}

func (p *KafkaPublisher) Publish(ctx context.Context, topic, key string, value []byte) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}
	// 异步写入
	return p.Writer.WriteMessages(ctx, msg)
}
