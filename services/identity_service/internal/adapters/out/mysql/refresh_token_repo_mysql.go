package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
)

type RefreshTokenModel struct {
	ID               string    `gorm:"column:id;primaryKey;type:varchar(64)"`
	UserID           string    `gorm:"column:user_id;type:varchar(64);not null;index"`
	AccessToken      string    `gorm:"column:access_token;type:text"`
	RefreshToken     string    `gorm:"column:refresh_token;type:text;not null"`
	RefreshExpiresAt time.Time `gorm:"column:refresh_expires_at;not null"`
	IsRevoked        bool      `gorm:"column:is_revoked;not null;default:false"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (RefreshTokenModel) TableName() string {
	return "refresh_tokens"
}

func refreshTokenModelFromEntity(token *entity.AuthToken) *RefreshTokenModel {
	return &RefreshTokenModel{
		ID:               token.ID,
		UserID:           token.UserID,
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		RefreshExpiresAt: token.RefreshExpiresAt,
		IsRevoked:        token.IsRevoked,
		CreatedAt:        token.CreatedAt,
	}
}

func (m *RefreshTokenModel) toEntity() *entity.AuthToken {
	return &entity.AuthToken{
		ID:               m.ID,
		UserID:           m.UserID,
		AccessToken:      m.AccessToken,
		RefreshToken:     m.RefreshToken,
		RefreshExpiresAt: m.RefreshExpiresAt,
		IsRevoked:        m.IsRevoked,
		CreatedAt:        m.CreatedAt,
	}
}

type RefreshTokenRepoMysql struct {
	db *gorm.DB
}

func NewRefreshTokenRepoMysql(db *gorm.DB) out.RefreshTokenRepository {
	return &RefreshTokenRepoMysql{db: db}
}

func (r *RefreshTokenRepoMysql) Save(ctx context.Context, token *entity.AuthToken) error {
	model := refreshTokenModelFromEntity(token)
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *RefreshTokenRepoMysql) Find(ctx context.Context, token string) (*entity.AuthToken, error) {
	var model RefreshTokenModel
	err := r.db.WithContext(ctx).Where("id = ?", token).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return model.toEntity(), nil
}

func (r *RefreshTokenRepoMysql) UpdateExpiry(ctx context.Context, token string, newExp int64) error {
	return r.db.WithContext(ctx).
		Model(&RefreshTokenModel{}).
		Where("id = ?", token).
		Update("refresh_expires_at", time.Unix(newExp, 0)).Error
}

func (r *RefreshTokenRepoMysql) Revoke(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).
		Model(&RefreshTokenModel{}).
		Where("id = ?", token).
		Update("is_revoked", true).Error
}
