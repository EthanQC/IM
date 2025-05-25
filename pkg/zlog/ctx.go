package zlog

import (
	"context" // goroutine 间上下文信息协同

	"go.uber.org/zap"
)

type ctxKey struct{}

// 将 logger 存进上下文
func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// 从上下文里取出 logger
func FromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return zap.L()
	}

	// 找值并做类型断言
	if l, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok && l != nil {
		return l
	}

	// 没存过就优雅回退
	return zap.L()
}
