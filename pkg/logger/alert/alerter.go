package alert

import (
	"github.com/EthanQC/IM/pkg/logger/model"
)

type Alerter interface {
	Alert(msg string, fields ...model.Field) error
}

// 邮件告警实现
type EmailAlerter struct {
	// 邮件配置
}

// Webhook告警实现
type WebhookAlerter struct {
	// webhook配置
}
