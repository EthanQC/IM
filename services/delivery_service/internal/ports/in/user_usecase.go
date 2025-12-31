package in

import (
	"context"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
)

// DeliveryUseCase 消息投递用例接口
type DeliveryUseCase interface {
	// DeliverMessage 投递消息
	DeliverMessage(ctx context.Context, event *entity.MessageEvent) error
	// DeliverToUser 投递消息给指定用户
	DeliverToUser(ctx context.Context, userID uint64, message []byte) error
	// ProcessPendingMessages 处理待投递消息
	ProcessPendingMessages(ctx context.Context, userID uint64) error
}

// ConnectionUseCase 连接管理用例接口
type ConnectionUseCase interface {
	// UserConnect 用户连接
	UserConnect(ctx context.Context, userID uint64, deviceID, platform, serverAddr string) error
	// UserDisconnect 用户断开连接
	UserDisconnect(ctx context.Context, userID uint64, deviceID string) error
	// Heartbeat 心跳
	Heartbeat(ctx context.Context, userID uint64, deviceID string) error
	// GetOnlineStatus 获取在线状态
	GetOnlineStatus(ctx context.Context, userIDs []uint64) (map[uint64]bool, error)
}
