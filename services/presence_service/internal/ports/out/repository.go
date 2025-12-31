package out

import (
	"context"

	"github.com/EthanQC/IM/services/presence_service/internal/domain/entity"
)

// PresenceRepository 在线状态仓储接口
type PresenceRepository interface {
	// SetOnline 设置用户上线
	SetOnline(ctx context.Context, userID uint64, nodeID string, deviceType string) error
	// SetOffline 设置用户下线
	SetOffline(ctx context.Context, userID uint64, nodeID string) error
	// UpdateStatus 更新状态
	UpdateStatus(ctx context.Context, userID uint64, status entity.PresenceStatus) error
	// SetCustomStatus 设置自定义状态
	SetCustomStatus(ctx context.Context, userID uint64, customStatus string) error
	// GetPresence 获取单个用户状态
	GetPresence(ctx context.Context, userID uint64) (*entity.UserPresence, error)
	// GetPresences 批量获取用户状态
	GetPresences(ctx context.Context, userIDs []uint64) (map[uint64]*entity.UserPresence, error)
	// UpdateHeartbeat 更新心跳
	UpdateHeartbeat(ctx context.Context, userID uint64) error
	// CleanExpired 清理过期的在线状态
	CleanExpired(ctx context.Context) (int64, error)
}

// EventPublisher 事件发布接口
type EventPublisher interface {
	// PublishPresenceChange 发布状态变更事件
	PublishPresenceChange(ctx context.Context, event *entity.PresenceEvent) error
}
