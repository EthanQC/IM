package in

import (
	"context"

	"github.com/EthanQC/IM/services/presence_service/internal/domain/entity"
)

// PresenceUseCase 在线状态用例接口
type PresenceUseCase interface {
	// ReportOnline 上报上线
	ReportOnline(ctx context.Context, userID uint64, nodeID, deviceType string) error
	// ReportOffline 上报下线
	ReportOffline(ctx context.Context, userID uint64, nodeID string) error
	// UpdateStatus 更新状态
	UpdateStatus(ctx context.Context, userID uint64, status entity.PresenceStatus) error
	// SetCustomStatus 设置自定义状态
	SetCustomStatus(ctx context.Context, userID uint64, customStatus string) error
	// GetPresence 获取用户状态
	GetPresence(ctx context.Context, userID uint64) (*entity.UserPresence, error)
	// GetPresences 批量获取用户状态
	GetPresences(ctx context.Context, userIDs []uint64) (map[uint64]*entity.UserPresence, error)
	// Heartbeat 心跳
	Heartbeat(ctx context.Context, userID uint64) error
}
