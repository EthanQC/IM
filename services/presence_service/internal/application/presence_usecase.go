package application

import (
	"context"
	"time"

	"github.com/EthanQC/IM/services/presence_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/presence_service/internal/ports/in"
	"github.com/EthanQC/IM/services/presence_service/internal/ports/out"
)

// PresenceUseCaseImpl 在线状态用例实现
type PresenceUseCaseImpl struct {
	presenceRepo   out.PresenceRepository
	eventPublisher out.EventPublisher
}

// NewPresenceUseCase 创建在线状态用例
func NewPresenceUseCase(
	presenceRepo out.PresenceRepository,
	eventPublisher out.EventPublisher,
) in.PresenceUseCase {
	return &PresenceUseCaseImpl{
		presenceRepo:   presenceRepo,
		eventPublisher: eventPublisher,
	}
}

// ReportOnline 上报上线
func (uc *PresenceUseCaseImpl) ReportOnline(ctx context.Context, userID uint64, nodeID, deviceType string) error {
	// 获取旧状态
	oldPresence, _ := uc.presenceRepo.GetPresence(ctx, userID)
	var oldStatus entity.PresenceStatus
	if oldPresence != nil {
		oldStatus = entity.PresenceStatus(oldPresence.Status)
	} else {
		oldStatus = entity.PresenceStatusOffline
	}

	// 设置上线
	if err := uc.presenceRepo.SetOnline(ctx, userID, nodeID, deviceType); err != nil {
		return err
	}

	// 发布状态变更事件
	if oldStatus != entity.PresenceStatusOnline && uc.eventPublisher != nil {
		event := &entity.PresenceEvent{
			UserID:    userID,
			OldStatus: oldStatus,
			NewStatus: entity.PresenceStatusOnline,
			Timestamp: time.Now(),
		}
		go uc.eventPublisher.PublishPresenceChange(context.Background(), event)
	}

	return nil
}

// ReportOffline 上报下线
func (uc *PresenceUseCaseImpl) ReportOffline(ctx context.Context, userID uint64, nodeID string) error {
	// 获取旧状态
	oldPresence, _ := uc.presenceRepo.GetPresence(ctx, userID)
	var oldStatus entity.PresenceStatus
	if oldPresence != nil && oldPresence.Online {
		oldStatus = entity.PresenceStatus(oldPresence.Status)
	} else {
		return nil // 本来就是离线状态
	}

	// 设置下线
	if err := uc.presenceRepo.SetOffline(ctx, userID, nodeID); err != nil {
		return err
	}

	// 发布状态变更事件
	if uc.eventPublisher != nil {
		event := &entity.PresenceEvent{
			UserID:    userID,
			OldStatus: oldStatus,
			NewStatus: entity.PresenceStatusOffline,
			Timestamp: time.Now(),
		}
		go uc.eventPublisher.PublishPresenceChange(context.Background(), event)
	}

	return nil
}

// UpdateStatus 更新状态
func (uc *PresenceUseCaseImpl) UpdateStatus(ctx context.Context, userID uint64, status entity.PresenceStatus) error {
	// 获取旧状态
	oldPresence, _ := uc.presenceRepo.GetPresence(ctx, userID)
	var oldStatus entity.PresenceStatus
	if oldPresence != nil {
		oldStatus = entity.PresenceStatus(oldPresence.Status)
	}

	// 更新状态
	if err := uc.presenceRepo.UpdateStatus(ctx, userID, status); err != nil {
		return err
	}

	// 发布状态变更事件
	if oldStatus != status && uc.eventPublisher != nil {
		event := &entity.PresenceEvent{
			UserID:    userID,
			OldStatus: oldStatus,
			NewStatus: status,
			Timestamp: time.Now(),
		}
		go uc.eventPublisher.PublishPresenceChange(context.Background(), event)
	}

	return nil
}

// SetCustomStatus 设置自定义状态
func (uc *PresenceUseCaseImpl) SetCustomStatus(ctx context.Context, userID uint64, customStatus string) error {
	return uc.presenceRepo.SetCustomStatus(ctx, userID, customStatus)
}

// GetPresence 获取用户状态
func (uc *PresenceUseCaseImpl) GetPresence(ctx context.Context, userID uint64) (*entity.UserPresence, error) {
	return uc.presenceRepo.GetPresence(ctx, userID)
}

// GetPresences 批量获取用户状态
func (uc *PresenceUseCaseImpl) GetPresences(ctx context.Context, userIDs []uint64) (map[uint64]*entity.UserPresence, error) {
	return uc.presenceRepo.GetPresences(ctx, userIDs)
}

// Heartbeat 心跳
func (uc *PresenceUseCaseImpl) Heartbeat(ctx context.Context, userID uint64) error {
	return uc.presenceRepo.UpdateHeartbeat(ctx, userID)
}
