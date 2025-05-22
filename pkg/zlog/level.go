package zlog

import (
	"net/http"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var dynamicLevel zap.AtomicLevel // 全局可变级别
var levelName atomic.Value       // 存一下字符串形式

func initLevel(lvl string) {
	levelName.Store(lvl)
	dynamicLevel = zap.NewAtomicLevelAt(parseLevel(lvl))
}

// parseLevel 将字符串转 zapcore.Level
func parseLevel(lvl string) zapcore.Level {
	switch strings.ToLower(lvl) {
	case "debug":
		return zap.DebugLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "panic":
		return zap.PanicLevel
	default:
		return zap.InfoLevel
	}
}

// SetLevel 热更新日志级别
func SetLevel(lvl string) {
	dynamicLevel.SetLevel(parseLevel(lvl))
	levelName.Store(lvl)
}

// GetLevel 返回当前级别字符串
func GetLevel() string {
	if v, ok := levelName.Load().(string); ok {
		return v
	}
	return "info"
}

// LevelHTTPHandler 用于注册到 /log/level (PUT: debug/info/error)
func LevelHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			lvl := r.URL.Query().Get("v")
			if lvl == "" {
				lvl = r.FormValue("v")
			}
			SetLevel(lvl)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		_, _ = w.Write([]byte(GetLevel()))
	}
}
