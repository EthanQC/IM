package eventbus

import (
	"context"
	"encoding/json"

	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

// KafkaEventBus 用 Sarama 实现 EventBus 接口，将事件发送到 Kafka
type KafkaEventBus struct {
	producer sarama.SyncProducer
}

// NewKafkaEventBus 创建一个 KafkaEventBus 实例
func NewKafkaEventBus(brokers []string) (out.EventBus, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	return &KafkaEventBus{producer: producer}, nil
}

// Publish 将事件序列化为 JSON 并发送到指定 Kafka topic
func (k *KafkaEventBus) Publish(ctx context.Context, topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		zap.L().Error("事件序列化失败", zap.String("topic", topic), zap.Error(err))
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	}
	partition, offset, err := k.producer.SendMessage(msg)
	if err != nil {
		zap.L().Error("Kafka Publish 失败", zap.String("topic", topic), zap.Error(err))
		return err
	}
	zap.L().Info("Kafka Publish 成功",
		zap.String("topic", topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
	)
	return nil
}
