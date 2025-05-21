package out

import (
	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
)

type UserStatusRepository interface {
	Save(status *entity.UserStatus) error
	Find(userID string) (*entity.UserStatus, error)
	Delete(userID string) error
}
