package out

import (
	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
)

type TokenRepository interface {
	Save(token *entity.AuthToken) error
	FindByID(id string) (*entity.AuthToken, error)
	FindByAccessToken(accessToken string) (*entity.AuthToken, error)
	FindByRefreshToken(refreshToken string) (*entity.AuthToken, error)
	Delete(id string) error
}
