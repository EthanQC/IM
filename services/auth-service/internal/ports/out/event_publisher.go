package out

import "context"

// EventPublisher 定义了发布业务事件的接口
type EventPublisher interface {
	// Publish 向指定 topic 发布 key + value 消息
	Publish(ctx context.Context, topic string, key string, value []byte) error
}
