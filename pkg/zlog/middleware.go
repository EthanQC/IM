package zlog

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GinLogger 在每个请求中放入 traceID / requestID，你可替换成自家 header
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		traceID := c.GetHeader("X-Trace-Id")
		reqID := c.GetHeader("X-Request-Id")

		l := zap.L().With(
			zap.String("trace_id", traceID),
			zap.String("request_id", reqID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
		)
		c.Request = c.Request.WithContext(WithContext(c.Request.Context(), l))
		c.Next()

		l.Info("access",
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.Int64("bytes_in", c.Request.ContentLength),
			zap.Int("bytes_out", c.Writer.Size()),
		)
	}
}
