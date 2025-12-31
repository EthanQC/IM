package out

import (
	"context"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
)

type UserStatusRepository interface {
	Get(ctx context.Context, userID string) (*entity.UserBlockStatus, error)
	Save(ctx context.Context, s *entity.UserBlockStatus) error
}
