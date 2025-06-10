package zlog

import (
	"os"

	// 日志轮转工具库，能按照文件大小、天数、备份数量自动切分日志文件，也可以对旧日志进行 gzip 压缩
	// 还实现了 io.Writer 接口，能像写普通文件一样直接往它写日志
	"gopkg.in/natefinch/lumberjack.v2"

	"go.uber.org/zap/zapcore"
)

// buildWriteSyncer 根据配置组装所有输出
func buildWriteSyncer(cfg Config) zapcore.WriteSyncer {
	// 存放所有想写到的目标
	var syncers []zapcore.WriteSyncer

	// 日志输出到终端
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

	return zapcore.NewMultiWriteSyncer(syncers...)
}
