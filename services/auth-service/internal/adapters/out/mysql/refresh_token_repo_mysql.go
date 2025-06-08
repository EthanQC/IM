package mysql

import (
	"context"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"gorm.io/gorm"
)

type RefreshTokenRepoMysql struct {
	db *gorm.DB
}

func NewRefreshTokenRepoMysql(db *gorm.DB) out.RefreshTokenRepository {
	return &RefreshTokenRepoMysql{db: db}
}

func (r *RefreshTokenRepoMysql) Save(ctx context.Context, token *entity.AuthToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *RefreshTokenRepoMysql) Find(ctx context.Context, token string) (*entity.AuthToken, error) {
	var t entity.AuthToken
	err := r.db.WithContext(ctx).Where("refresh_jti = ?", token).First(&t).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}

	return &t, err
}

func (r *RefreshTokenRepoMysql) UpdateExpiry(ctx context.Context, token string, newExp int64) error {
	return r.db.WithContext(ctx).
		Model(&entity.AuthToken{}).
		Where("refresh_jti = ?", token).
		Update("refresh_exp_at", newExp).Error
}

func (r *RefreshTokenRepoMysql) Revoke(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).
		Model(&entity.AuthToken{}).
		Where("refresh_jti = ?", token).
		Update("is_revoked", true).Error
}
