package zlog

import (
	"os"
	"strings"

	// 使用 uber 开源的 zap 日志库
	"go.uber.org/zap"         // 更高层的 api
	"go.uber.org/zap/zapcore" // 底层构件
)

// 创建一个 *zap.Logger，不替换全局
// opts 可传可不传
func New(cfg Config, opts ...zap.Option) (*zap.Logger, error) {
	// 初始化全局可变日志级别
	initLevel(cfg.Level)

	// 配置编码器
	env := strings.ToLower(os.Getenv("APP_ENV"))
	var encCfg zapcore.EncoderConfig

	if env == "dev" || env == "test" {
		encCfg = zap.NewDevelopmentEncoderConfig()
	} else {
		encCfg = zap.NewProductionEncoderConfig()
	}

	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder   // 时间字段按 YYYY-MM-DDThh:mm:ss.sssZ07:00（ISO8601）格式输出，可读性更好
	encCfg.EncodeCaller = zapcore.ShortCallerEncoder // 调用位置只输出 file.go:123，不带全路径，节省空间

	// 根据不同配置选择编码
	var encoder zapcore.Encoder
	if strings.ToLower(cfg.Encoding) == "console" {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encCfg)
	}

	// 构建底层核心
	core := zapcore.NewCore(
		encoder,
		buildWriteSyncer(cfg),
		dynamicLevel,
	)

	// Prometheus 埋点
	if cfg.EnableMetric {
		core = wrapWithMetric(core, cfg)
	}

	allOpts := append(opts,
		zap.AddCaller(),
		zap.Fields(zap.String("service", cfg.Service)),
	)

	logger := zap.New(core, allOpts...)

	return logger, nil
}
