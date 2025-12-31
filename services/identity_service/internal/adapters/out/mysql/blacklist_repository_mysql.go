package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
)

// BlacklistModel GORM模型
type BlacklistModel struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID        uint64    `gorm:"column:user_id;not null;index"`
	BlockedUserID uint64    `gorm:"column:blocked_user_id;not null;index"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (BlacklistModel) TableName() string {
	return "blacklist"
}

// BlacklistRepositoryMySQL MySQL黑名单仓储实现
type BlacklistRepositoryMySQL struct {
	db *gorm.DB
}

func NewBlacklistRepositoryMySQL(db *gorm.DB) out.BlacklistRepository {
	return &BlacklistRepositoryMySQL{db: db}
}

func (r *BlacklistRepositoryMySQL) Add(ctx context.Context, userID, blockedUserID uint64) error {
	model := &BlacklistModel{
		UserID:        userID,
		BlockedUserID: blockedUserID,
	}
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *BlacklistRepositoryMySQL) Remove(ctx context.Context, userID, blockedUserID uint64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND blocked_user_id = ?", userID, blockedUserID).
		Delete(&BlacklistModel{}).Error
}

func (r *BlacklistRepositoryMySQL) IsBlocked(ctx context.Context, userID, blockedUserID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&BlacklistModel{}).
		Where("user_id = ? AND blocked_user_id = ?", userID, blockedUserID).
		Count(&count).Error
	return count > 0, err
}

func (r *BlacklistRepositoryMySQL) List(ctx context.Context, userID uint64, page, pageSize int) ([]uint64, int, error) {
	var models []BlacklistModel
	var total int64

	query := r.db.WithContext(ctx).Model(&BlacklistModel{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&models).Error; err != nil {
		return nil, 0, err
	}

	userIDs := make([]uint64, len(models))
	for i, m := range models {
		userIDs[i] = m.BlockedUserID
	}

	return userIDs, int(total), nil
}
