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

// NewKafkaEventBus 构造函数
func NewKafkaEventBus(brokers []string) (out.EventBus, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	// clientID、ACK 等可以根据需要配置
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	return &KafkaEventBus{producer: producer}, nil
}

func (k *KafkaEventBus) Publish(ctx context.Context, topic string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	}
	partition, offset, err := k.producer.SendMessage(msg)
	if err != nil {
		return err
	}
	zap.L().Info("Kafka Publish 成功", zap.String("topic", topic), zap.Int32("partition", partition), zap.Int64("offset", offset))
	return nil
}
