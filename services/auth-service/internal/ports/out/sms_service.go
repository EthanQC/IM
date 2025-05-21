package out

import (
	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
)

type SmsService interface {
	SendCode(phone string, code string) error
	VerifyCode(phone string, code string) (bool, error)
}

type CodeRepository interface {
	Save(code *entity.AuthCode) error
	Find(phone string) (*entity.AuthCode, error)
	Delete(phone string) error
}
