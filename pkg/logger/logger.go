package logger

import (
	"github.com/EthanQC/IM/pkg/logger/alert"
	"github.com/EthanQC/IM/pkg/logger/config"
	"go.uber.org/zap"
)

type Logger struct {
	cfg     *config.Config
	zap     *zap.Logger
	alerter alert.Alerter
	metrics *Metrics
}

// 创建新的logger实例
func NewLogger(cfg *config.Config, opts ...Option) (*Logger, error) {
	if cfg == nil {
		cfg = defaultConfig()
	}

	// 1. 初始化zap
	zapLogger, err := initZap(cfg)
	if err != nil {
		return nil, err
	}

	// 2. 初始化告警
	alerter, err := initAlerter(cfg.Alert)
	if err != nil {
		return nil, err
	}

	// 3. 初始化监控
	metrics, err := initMetrics(cfg.Metrics)
	if err != nil {
		return nil, err
	}

	l := &Logger{
		cfg:     cfg,
		zap:     zapLogger,
		alerter: alerter,
		metrics: metrics,
	}

	// 应用选项
	for _, opt := range opts {
		opt(l)
	}

	return l, nil
}

// 日志记录方法
func (l *Logger) Info(msg string, fields ...core.Field) {
	// 1. 记录日志
	l.zap.Info(msg, fieldsToZap(fields)...)

	// 2. 更新指标
	l.metrics.IncLogCounter("info")
}

func (l *Logger) Error(msg string, fields ...core.Field) {
	// 1. 记录日志
	l.zap.Error(msg, fieldsToZap(fields)...)

	// 2. 触发告警
	if l.alerter != nil {
		l.alerter.Alert(msg, fields...)
	}

	// 3. 更新指标
	l.metrics.IncLogCounter("error")
}

// 提供给其他服务使用的辅助方法
func (l *Logger) WithService(service string) *Logger {
	return l.With(core.String("service", service))
}

func (l *Logger) WithTraceID(traceID string) *Logger {
	return l.With(core.String("trace_id", traceID))
}
