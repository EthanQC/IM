package out

import (
	"context"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
)

// OnlineUserRepository 在线用户仓储接口
type OnlineUserRepository interface {
	// SetOnline 设置用户在线
	SetOnline(ctx context.Context, user *entity.OnlineUser) error
	// SetOffline 设置用户离线
	SetOffline(ctx context.Context, userID uint64, deviceID string) error
	// GetOnlineDevices 获取用户的所有在线设备
	GetOnlineDevices(ctx context.Context, userID uint64) ([]*entity.OnlineUser, error)
	// IsOnline 检查用户是否在线
	IsOnline(ctx context.Context, userID uint64) (bool, error)
	// GetOnlineUsers 批量获取在线用户
	GetOnlineUsers(ctx context.Context, userIDs []uint64) (map[uint64][]*entity.OnlineUser, error)
	// UpdateLastPing 更新最后心跳时间
	UpdateLastPing(ctx context.Context, userID uint64, deviceID string) error
}

// PendingMessageRepository 待投递消息仓储接口
type PendingMessageRepository interface {
	// Save 保存待投递消息
	Save(ctx context.Context, msg *entity.PendingMessage) error
	// GetPending 获取待投递消息
	GetPending(ctx context.Context, userID uint64, limit int) ([]*entity.PendingMessage, error)
	// MarkDelivered 标记为已投递
	MarkDelivered(ctx context.Context, id uint64) error
	// MarkFailed 标记为投递失败
	MarkFailed(ctx context.Context, id uint64) error
	// IncrRetryCount 增加重试次数
	IncrRetryCount(ctx context.Context, id uint64) error
	// DeleteExpired 删除过期消息
	DeleteExpired(ctx context.Context, before int64) (int64, error)
}

// ConnectionManager 连接管理器接口
type ConnectionManager interface {
	// Register 注册连接
	Register(userID uint64, deviceID string, conn Connection) error
	// Unregister 注销连接
	Unregister(userID uint64, deviceID string) error
	// GetConnections 获取用户的所有连接
	GetConnections(userID uint64) []Connection
	// Send 发送消息给用户
	Send(userID uint64, message []byte) error
	// SendToDevice 发送消息给指定设备
	SendToDevice(userID uint64, deviceID string, message []byte) error
	// Broadcast 广播消息给多个用户
	Broadcast(userIDs []uint64, message []byte) error
}

// Connection WebSocket连接接口
type Connection interface {
	// Send 发送消息
	Send(message []byte) error
	// Close 关闭连接
	Close() error
	// UserID 获取用户ID
	UserID() uint64
	// DeviceID 获取设备ID
	DeviceID() string
}

// PushService 推送服务接口
type PushService interface {
	// Push 推送通知
	Push(ctx context.Context, notification *entity.PushNotification) error
	// PushBatch 批量推送
	PushBatch(ctx context.Context, notifications []*entity.PushNotification) error
}

// MessageConsumer 消息消费者接口
type MessageConsumer interface {
	// Start 启动消费
	Start(ctx context.Context) error
	// Stop 停止消费
	Stop() error
}
