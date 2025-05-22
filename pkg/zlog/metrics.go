package zlog

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap/zapcore"
)

var (
	logCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_log_total",
			Help: "Number of log entries by level.",
		},
		[]string{"service", "level"},
	)
)

// RegisterMetrics 在外层 main 包里调用 prometheus.MustRegister
func RegisterMetrics(reg prometheus.Registerer) {
	reg.MustRegister(logCounter)
}

// metricsCore 装饰一个 zapcore.Core，记录日志条数
type metricsCore struct {
	zapcore.Core
	service string
}

func (m metricsCore) With(fields []zapcore.Field) zapcore.Core {
	return metricsCore{Core: m.Core.With(fields), service: m.service}
}

func (m metricsCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	logCounter.WithLabelValues(m.service, ent.Level.String()).Inc()
	return m.Core.Check(ent, ce)
}

// wrapWithMetric 根据 cfg 决定是否包一层
func wrapWithMetric(c zapcore.Core, cfg Config) zapcore.Core {
	if cfg.EnableMetric {
		return metricsCore{Core: c, service: cfg.Service}
	}
	return c
}
