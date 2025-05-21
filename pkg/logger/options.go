package logger

import (
	"kama_chat_server/pkg/logger/formatter"

	"github.com/EthanQC/IM/pkg/logger/alert"
)

type Option func(*Logger)

// WithLevel 设置日志级别
func WithLevel(level core.Level) Option {
	return func(l *Logger) {
		l.config.Level = string(level)
	}
}

// WithFormatter 设置格式化器
func WithFormatter(formatter formatter.Formatter) Option {
	return func(l *Logger) {
		l.formatter = formatter
	}
}

// WithAlerter 添加告警器
func WithAlerter(alerter alert.Alerter) Option {
	return func(l *Logger) {
		l.alerters = append(l.alerters, alerter)
	}
}
