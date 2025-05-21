package config

type Config struct {
	// 基础配置
	ServiceName string // 服务名称
	Level       string // 日志级别

	// 输出配置
	Console bool // 是否输出到控制台
	File    FileConfig

	// 格式化配置
	Format     string // json/text
	TimeFormat string // 时间格式

	// 告警配置
	Alert AlertConfig

	// 监控配置
	Metrics MetricsConfig
}

type FileConfig struct {
	Filename   string // 日志文件路径
	MaxSize    int    // 单个文件最大大小(MB)
	MaxAge     int    // 日志保留天数
	MaxBackups int    // 保留的旧文件个数
	Compress   bool   // 是否压缩
}

type AlertConfig struct {
	Enabled  bool     // 是否启用告警
	Level    string   // 触发告警的级别
	Channels []string // 告警通道(email/webhook等)
}

type MetricsConfig struct {
	Enabled bool   // 是否启用监控
	Address string // Prometheus地址
}
