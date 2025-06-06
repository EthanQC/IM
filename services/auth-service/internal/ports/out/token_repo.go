package out

import (
	"context"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
)

type TokenRepository interface {
	Save(ctx context.Context, token *entity.AuthToken) error
	Find(ctx context.Context, token string) (*entity.AuthToken, error)
	UpdateExpiry(ctx context.Context, token string, newExp int64) error
	Revoke(ctx context.Context, token string) error
}
