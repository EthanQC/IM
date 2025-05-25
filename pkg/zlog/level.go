package zlog

import (
	"net/http"    // go 的 http 标准库
	"strings"     // go 的标准字符串工具包
	"sync/atomic" // go 的标准原子操作包

	"go.uber.org/zap"         // Uber 开源的高性能结构化日志库 zap，提供最上层的 logger 和多种 option，以及友好的 api
	"go.uber.org/zap/zapcore" // zap 的核心抽象层，定义了很多底层组件，用于构造和过滤日志
)

// zap.AtomicLevel 是 zap 提供的一个级别控制结构
// 这里用来作为过滤器
var dynamicLevel zap.AtomicLevel

// atomic.Value 是可存任意类型值、并发读写安全的容器
// 用 Store(interface{}) 存值，用 Load() 取值，且不会发生数据竞争
// 这里用来存当前级别的原始字符串
var levelName atomic.Value

func initLevel(lvl string) {
	levelName.Store(lvl)
	dynamicLevel = zap.NewAtomicLevelAt(parseLevel(lvl))
}

// 将字符串转为 zapcore.Level
func parseLevel(lvl string) zapcore.Level {
	// ToLower 将任意字符串中的大写字母都转成小写
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

// 热更新日志级别
func SetLevel(lvl string) {
	dynamicLevel.SetLevel(parseLevel(lvl))
	levelName.Store(lvl)
}

// 返回当前级别字符串
func GetLevel() string {
	// Load 会返回一个 interface{}，可能包含任意类型的值
	// 这里就是把这个 interface{} 当作 string 拿出来，做类型断言
	if v, ok := levelName.Load().(string); ok {
		return v
	}

	return "info"
}

// 热更新日志级别管理接口，用于注册到 /log/level (PUT: debug/info/error)
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

		// 如果不是 PUT，会被默认当成是 GET
		_, _ = w.Write([]byte(GetLevel()))
	}
}
