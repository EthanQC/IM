package sms

import (
	"context"
	"fmt"

	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"github.com/EthanQC/IM/services/auth-service/pkg/errors"
)

type VerifyCodeUseCase struct {
	authCodeRepo out.AuthCodeRepository
}

// NewVerifyCodeUseCase 创建验证码校验用例
func NewVerifyCodeUseCase(repo out.AuthCodeRepository) *VerifyCodeUseCase {
	return &VerifyCodeUseCase{authCodeRepo: repo}
}

// Execute 校验输入的 code 是否与存储值一致，成功后删除
func (uc *VerifyCodeUseCase) Execute(ctx context.Context, phone, code string) error {
	stored, err := uc.authCodeRepo.Find(ctx, phone)
	if err != nil {
		return fmt.Errorf("get code: %w", err)
	}

	if stored.Code != code {
		return errors.ErrCodeExpired
	}

	if err := uc.authCodeRepo.Delete(ctx, phone); err != nil {
		return fmt.Errorf("delete code: %w", err)
	}

	return nil
}
