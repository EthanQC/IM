package formatter

// Formatter 定义日志格式化接口
type Formatter interface {
	Format(level core.Level, msg string, fields []core.Field) ([]byte, error)
}
