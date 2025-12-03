package mysql

import (
	"context"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
	"gorm.io/gorm"
)

type UserStatusRepoMysql struct {
	db *gorm.DB
}

func NewUserStatusRepoMysql(db *gorm.DB) out.UserStatusRepository {
	return &UserStatusRepoMysql{db: db}
}

func (r *UserStatusRepoMysql) Get(ctx context.Context, userID string) (*entity.UserStatus, error) {
	var s entity.UserStatus
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&s).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}

	return &s, err
}

func (r *UserStatusRepoMysql) Save(ctx context.Context, s *entity.UserStatus) error {
	return r.db.WithContext(ctx).Save(s).Error
}
