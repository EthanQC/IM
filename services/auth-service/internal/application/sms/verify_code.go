package sms

import (
	"github.com/EthanQC/IM/services/auth-service/internal/ports/in"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
)

type VerifyCodeUseCase struct {
	authCodeRepo out.AuthCodeRepository
}

func NewVerifyCodeUseCase(repo out.AuthCodeRepository) in.SMSUseCase {
	return &VerifyCodeUseCase{authCodeRepo: repo}
}
