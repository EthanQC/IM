package implementations

import (
	"fmt"
	"net/smtp"
)

type EmailAlerter struct {
	smtp     *smtp.Client
	from     string
	to       []string
	minLevel core.Level
}

func (a *EmailAlerter) Alert(level core.Level, msg string, fields []core.Field) error {
	if level < a.minLevel {
		return nil
	}

	// 构建邮件内容
	content := fmt.Sprintf("Level: %s\nMessage: %s\n", level, msg)
	for _, f := range fields {
		content += fmt.Sprintf("%s: %v\n", f.Key, f.Value)
	}

	// 发送告警邮件
	return smtp.SendMail(a.smtp, a.from, a.to, "Log Alert", content)
}
