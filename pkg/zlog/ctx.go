package zlog

import (
	"context"

	"go.uber.org/zap"
)

type ctxKey struct{}

func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return zap.L()
	}
	if l, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok && l != nil {
		return l
	}
	return zap.L()
}

// C 是简写，常在业务层使用
func C(ctx context.Context) *zap.Logger { return FromContext(ctx) }
