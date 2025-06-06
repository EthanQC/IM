package out

import "context"

// EventBus 若未来要广播封禁、登录失败等事件再实现
type EventBus interface {
	Publish(ctx context.Context, topic string, payload any) error
}
