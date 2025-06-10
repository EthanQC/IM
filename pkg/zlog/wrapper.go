package zlog

import (
	"go.uber.org/zap"
)

type Field = zap.Field

func String(key, val string) Field {
	return zap.String(key, val)
}

func Int(key string, val int) Field {
	return zap.Int(key, val)
}

func Bool(key string, val bool) Field {
	return zap.Bool(key, val)
}

func Any(key string, val interface{}) Field {
	return zap.Any(key, val)
}

func Debug(msg string, fields ...Field) {
	zap.L().Debug(msg, fields...)
}

func Info(msg string, fields ...Field) {
	zap.L().Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	zap.L().Warn(msg, fields...)
}

func Error(msg string, fields ...Field) {
	zap.L().Error(msg, fields...)
}

func Sync() error {
	return zap.L().Sync()
}
