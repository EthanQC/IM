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
	authCodeRepo out.AuthCodeRepository
	smsClient    out.SMSClient
	codeTTL      time.Duration
}

func NewSendCodeUseCase(repo out.AuthCodeRepository, client out.SMSClient, ttl time.Duration) in.SMSUseCase {
	return &SendCodeUseCase{
		authCodeRepo: repo,
		smsClient:    client,
		codeTTL:      ttl,
	}
}

func (u *SendCodeUseCase) SendSMSCode(ctx context.Context, phone vo.Phone, ip string) error {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	authCode := entity.NewAuthCode(phone.Number, phone.IP)
	authCode.Code = code

	if err := u.authCodeRepo.Save(ctx, authCode); err != nil {
		return fmt.Errorf("保存验证码失败：%w", err)
	}

	if err := u.smsClient.Send(ctx, phone.Number, code); err != nil {
		return fmt.Errorf("发送短信失败：%w", err)
	}

	return nil
}
