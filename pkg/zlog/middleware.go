package zlog

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 在每个请求中放入 traceID / requestID
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		traceID := c.GetHeader("Trace-Id")
		reqID := c.GetHeader("Request-Id")

		base := FromContext(c.Request.Context())

		l := base.With(
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
