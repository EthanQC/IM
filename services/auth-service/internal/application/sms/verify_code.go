package sms

import (
	"context"
	"errors"
	"fmt"

	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"github.com/EthanQC/IM/services/auth-service/pkg/errors"
)

type VerifyCodeUseCase struct {
	TokenRepo out.AccessTokenRepository
}

// NewVerifyCodeUseCase 创建验证码校验用例
func NewVerifyCodeUseCase(tokenRepo out.AccessTokenRepository) *VerifyCodeUseCase {
	return &VerifyCodeUseCase{TokenRepo: tokenRepo}
}

// Execute 校验输入的 code 是否与存储值一致，成功后删除
func (uc *VerifyCodeUseCase) Execute(ctx context.Context, phone, code string) error {
	stored, err := uc.TokenRepo.GetCode(ctx, phone)
	if err != nil {
		return fmt.Errorf("get code: %w", err)
	}
	if stored != code {
		return errors.ErrInvalidCode
	}
	if err := uc.TokenRepo.DeleteCode(ctx, phone); err != nil {
		return fmt.Errorf("delete code: %w", err)
	}
	return nil
}
