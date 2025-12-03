package sms

import (
	"context"
	"fmt"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/vo"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
	"github.com/EthanQC/IM/services/identity_service/pkg/errors"
)

type VerifyCodeUseCase struct {
	AuthCodeRepo out.AuthCodeRepository
	MaxAttempts  int
}

func NewVerifyCodeUseCase(repo out.AuthCodeRepository, maxAttempts int) *VerifyCodeUseCase {
	return &VerifyCodeUseCase{
		AuthCodeRepo: repo,
		MaxAttempts:  maxAttempts,
	}
}

// Execute 校验验证码：存在→未过期→未超限→匹配→删除
func (uc *VerifyCodeUseCase) Execute(ctx context.Context, phone vo.Phone, code string) error {
	stored, err := uc.AuthCodeRepo.Find(ctx, phone.Number)
	if err != nil {
		return fmt.Errorf("获取验证码失败: %w", err)
	}
	if stored == nil {
		return errors.ErrCodeNotFound
	}
	if stored.IsExpired() {
		// 过期直接删除
		_ = uc.AuthCodeRepo.Delete(ctx, phone.Number)
		return errors.ErrCodeExpired
	}
	if stored.AttemptCnt >= uc.MaxAttempts {
		return errors.ErrTooManyAttempts
	}
	if stored.Code != code {
		// 累计一次失败
		_ = uc.AuthCodeRepo.IncrementAttempts(ctx, phone.Number)
		return errors.ErrCodeInvalid
	}
	// 成功后删除
	if err := uc.AuthCodeRepo.Delete(ctx, phone.Number); err != nil {
		return fmt.Errorf("删除验证码失败: %w", err)
	}
	return nil
}
