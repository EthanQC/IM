package in

import (
	"context"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
)

type SMSUseCase interface {
	// 发送短信验证码
	SendSMSCode(ctx context.Context, phone vo.Phone, ip string) error
}
