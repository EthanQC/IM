package application

import (
	"context"
	"time"

	"github.com/EthanQC/IM/services/delivery_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

// ConnectionUseCaseImpl 连接管理用例实现
type ConnectionUseCaseImpl struct {
	onlineUserRepo out.OnlineUserRepository
	deliveryUseCase in.DeliveryUseCase
}

// NewConnectionUseCase 创建连接管理用例
func NewConnectionUseCase(
	onlineUserRepo out.OnlineUserRepository,
	deliveryUseCase in.DeliveryUseCase,
) in.ConnectionUseCase {
	return &ConnectionUseCaseImpl{
		onlineUserRepo:  onlineUserRepo,
		deliveryUseCase: deliveryUseCase,
	}
}

// UserConnect 用户连接
func (uc *ConnectionUseCaseImpl) UserConnect(ctx context.Context, userID uint64, deviceID, platform, serverAddr string) error {
	user := &entity.OnlineUser{
		UserID:      userID,
		DeviceID:    deviceID,
		Platform:    platform,
		ServerAddr:  serverAddr,
		ConnectedAt: time.Now(),
		LastPingAt:  time.Now(),
	}

	if err := uc.onlineUserRepo.SetOnline(ctx, user); err != nil {
		return err
	}

	// 处理待投递的消息
	go func() {
		bgCtx := context.Background()
		if uc.deliveryUseCase != nil {
			uc.deliveryUseCase.ProcessPendingMessages(bgCtx, userID)
		}
	}()

	return nil
}

// UserDisconnect 用户断开连接
func (uc *ConnectionUseCaseImpl) UserDisconnect(ctx context.Context, userID uint64, deviceID string) error {
	return uc.onlineUserRepo.SetOffline(ctx, userID, deviceID)
}

// Heartbeat 心跳
func (uc *ConnectionUseCaseImpl) Heartbeat(ctx context.Context, userID uint64, deviceID string) error {
	return uc.onlineUserRepo.UpdateLastPing(ctx, userID, deviceID)
}

// GetOnlineStatus 获取在线状态
func (uc *ConnectionUseCaseImpl) GetOnlineStatus(ctx context.Context, userIDs []uint64) (map[uint64]bool, error) {
	result := make(map[uint64]bool)
	
	onlineUsers, err := uc.onlineUserRepo.GetOnlineUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	for _, userID := range userIDs {
		devices, ok := onlineUsers[userID]
		result[userID] = ok && len(devices) > 0
	}

	return result, nil
}
