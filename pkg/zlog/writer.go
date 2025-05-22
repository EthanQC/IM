package zlog

import (
	"os"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap/zapcore"
)

// buildWriteSyncer 根据配置组装所有输出
func buildWriteSyncer(cfg Config) zapcore.WriteSyncer {
	var syncers []zapcore.WriteSyncer

	if cfg.Stdout {
		syncers = append(syncers, zapcore.AddSync(os.Stdout))
	}

	if p := cfg.File.Path; p != "" {
		lj := &lumberjack.Logger{
			Filename:   p,
			MaxSize:    cfg.File.MaxSizeMB,
			MaxAge:     cfg.File.MaxAgeDay,
			MaxBackups: cfg.File.MaxBackups,
			Compress:   cfg.File.Compress,
		}
		syncers = append(syncers, zapcore.AddSync(lj))
	}

	// 预留：Kafka / Loki / OTLP
	return zapcore.NewMultiWriteSyncer(syncers...)
}
