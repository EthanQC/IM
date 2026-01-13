package zlog

import (
	"fmt"
	"time"

	"github.com/spf13/viper" // 配置管理工具库
)

// 定义本地轮转文件策略
// tag 被 viper 用来匹配字段
type FileConfig struct {
	Path       string `mapstructure:"path"`        // 日志文件路径
	MaxSizeMB  int    `mapstructure:"max_size"`    // 单个日志文件最大容量（MB）
	MaxBackups int    `mapstructure:"max_backups"` // 保留旧文件数量
	MaxAgeDay  int    `mapstructure:"max_age"`     // 最长保存天数
	Compress   bool   `mapstructure:"compress"`    // 是否压缩旧日志文件
}

// 日志配置
type Config struct {
	Service      string     `mapstructure:"service"`       // 归属服务名
	Level        string     `mapstructure:"level"`         // 日志级别，debug|info|warn|error
	Encoding     string     `mapstructure:"encoding"`      // 输出格式，json|console
	Stdout       bool       `mapstructure:"stdout"`        // 是否把日志同时输出到控制台
	File         FileConfig `mapstructure:"file"`          // 文件相关配置
	EnableMetric bool       `mapstructure:"enable_metric"` // 是否上报 Prometheus 指标
}

// 加载配置
func LoadConfig(filePath string) (*Config, error) {
	// 新建 viper 实例
	v := viper.New()

	// 指定要解析传入的配置文件
	v.SetConfigFile(filePath)

	// 在配置文件中找不到某个配置项时，自动去查找相应的环境变量
	v.AutomaticEnv()
	v.SetEnvPrefix("ZLOG") // 查环境变量时只会匹配 ZLOG 开头的环境变量

	// 读取并解析指定的配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取日志配置文件失败：%w", err)
	}

	// 默认值
	v.SetDefault("service", "unknown")
	v.SetDefault("level", "info")
	v.SetDefault("encoding", "json")
	v.SetDefault("stdout", true)
	v.SetDefault("file.max_size", 100)
	v.SetDefault("file.max_backups", 60)
	v.SetDefault("file.max_age", 1)
	v.SetDefault("enable_metric", true)

	// 反序列化加载到新的结构体
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("加载日志配置失败：%w", err)
	}

	// 严格校验
	if cfg.Service == "" {
		return nil, fmt.Errorf("配置错误：service 不能为空")
	}

	switch cfg.Level {
	case "debug", "info", "warn", "error":
	default:
		return nil, fmt.Errorf("配置错误：level 只能是 debug/info/warn/error")
	}

	switch cfg.Encoding {
	case "json", "console":
	default:
		return nil, fmt.Errorf("配置错误：encoding 只能是 json/console")
	}

	// 如果启用了 stdout，允许不设置文件路径
	if !cfg.Stdout && cfg.File.Path == "" {
		return nil, fmt.Errorf("配置错误：stdout 为 false 时，file.path 不能为空")
	}

	// 如果设置了文件路径，验证其他文件相关配置
	if cfg.File.Path != "" {
		if cfg.File.MaxSizeMB <= 0 {
			cfg.File.MaxSizeMB = 100 // 默认 100MB
		}

		if cfg.File.MaxBackups < 0 {
			cfg.File.MaxBackups = 60 // 默认 60 个
		}

		if cfg.File.MaxAgeDay < 0 {
			cfg.File.MaxAgeDay = 30 // 默认 30 天
		}
	}

	return &cfg, nil
}

func LogFilenameWithDate(base string) string {
	return base + "." + time.Now().Format("2006-01-02")
}
