package zlog

import (
	"os"        // 与操作系统交互标准包，常用于读写环境变量和操作文件等
	"os/signal" // 接收操作系统信号标准包，将外部发来的 Unix 信号转成可读的通道消息
	"syscall"   // 对底层操作系统系统调用的封装标准库，定义了各种系统信号、文件权限常量等

	"go.uber.org/zap"
)

// 创建 logger 并替换 zap 全局实例
func MustInitGlobal(cfg Config) {
	// zap.AddCallerSkip(1) 会在 AddCaller 的基础上再跳过一层调用栈
	// 这样日志中显示的文件行号会正确地指向业务代码，而不是 MustInitGlobal 内部
	l, err := New(cfg, zap.AddCallerSkip(1))

	if err != nil {
		panic(err)
	}

	zap.ReplaceGlobals(l)
	setupSignalHandler()
}

// 监听 SIGHUP 触发级别切换为 debug，再次 SIGHUP 切回 info
// 初始化全局实例后，可在 CLI 中使用 kill -HUP <pid> 命令来切换日志级别，方便运维管理
func setupSignalHandler() {
	// 声明一个 os.Signal 类型的、缓冲大小为 1 的 channel
	// os.Signal 是一个空接口，用来通用地标识任意操作系统信号类型
	c := make(chan os.Signal, 1)

	// 传给 Notify，让它将 SIGHUP 信号转给 channel c
	signal.Notify(c, syscall.SIGHUP)

	// 启动 goroutine
	go func() {
		// 不断从 channel c 接收消息，每收到一次就执行一次循环
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
