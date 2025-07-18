package sms

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/in"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
)

type SendCodeUseCase struct {
	AuthCodeRepo out.AuthCodeRepository
	SMSClient    out.SMSClient
	CodeTTL      time.Duration
	rng          *rand.Rand
}

func NewSendCodeUseCase(
	repo out.AuthCodeRepository,
	client out.SMSClient,
	ttl time.Duration,
) in.SMSUseCase {
	return &SendCodeUseCase{
		AuthCodeRepo: repo,
		SMSClient:    client,
		CodeTTL:      ttl,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SendSMSCode 生成验证码、保存到 Redis 并下发
func (u *SendCodeUseCase) SendSMSCode(ctx context.Context, phone vo.Phone, ip string) error {
	// 生成 6 位随机码
	code := fmt.Sprintf("%06d", u.rng.Intn(1_000_000))

	// 组装实体并保存
	authCode := entity.NewAuthCode(phone.Number, ip)
	authCode.Code = code
	authCode.ExpireTime = time.Now().Add(u.CodeTTL)

	if err := u.AuthCodeRepo.Save(ctx, authCode); err != nil {
		return fmt.Errorf("保存验证码失败: %w", err)
	}

	// 下发短信
	if err := u.SMSClient.Send(ctx, phone.Number, code); err != nil {
		return fmt.Errorf("发送短信失败: %w", err)
	}

	return nil
}
