package out

import (
	"context"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
)

type AuthCodeRepository interface {
	Save(ctx context.Context, code *entity.AuthCode) error
	Find(ctx context.Context, phone string) (*entity.AuthCode, error)
	Delete(ctx context.Context, phone string) error
	IncrementAttempts(ctx context.Context, phone string) error
}
