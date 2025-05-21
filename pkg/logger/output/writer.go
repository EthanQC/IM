package output

// Writer 定义日志输出接口
type Writer interface {
	Write(p []byte) (n int, err error)
	Sync() error
}
