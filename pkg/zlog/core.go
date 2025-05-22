package zlog

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 创建一个 *zap.Logger，不替换全局
func New(cfg Config, opts ...zap.Option) (*zap.Logger, error) {
	initLevel(cfg.Level)

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeCaller = zapcore.ShortCallerEncoder

	var encoder zapcore.Encoder
	if strings.ToLower(cfg.Encoding) == "console" {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encCfg)
	}

	core := zapcore.NewCore(
		encoder,
		buildWriteSyncer(cfg),
		dynamicLevel,
	)

	core = wrapWithMetric(core, cfg) // ⬅ Prometheus 装饰

	logger := zap.New(core,
		append(opts,
			zap.AddCaller(),
			zap.Fields(zap.String("service", cfg.Service)),
		)...,
	)
	return logger, nil
}
