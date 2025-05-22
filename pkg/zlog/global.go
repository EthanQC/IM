package zlog

import (
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

// MustInitGlobal 创建 logger 并替换 zap 全局实例
func MustInitGlobal(cfg Config) {
	l, err := New(cfg, zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(l)
	setupSignalHandler()
}

// setupSignalHandler 监听 SIGHUP 触发级别切换为 debug，再次 SIGHUP 切回 info
func setupSignalHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func() {
		for range c {
			if GetLevel() == "debug" {
				SetLevel("info")
			} else {
				SetLevel("debug")
			}
			zap.L().Info("log level toggled", zap.String("now", GetLevel()))
		}
	}()
}
