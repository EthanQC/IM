package output

import "github.com/natefinch/lumberjack"

type FileWriter struct {
	lumberjack *lumberjack.Logger
}

func NewFileWriter(filename string, maxSize int, maxBackups int, maxAge int, compress bool) *FileWriter {
	return &FileWriter{
		lumberjack: &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   compress,
		},
	}
}

func (w *FileWriter) Write(p []byte) (n int, err error) {
	return w.lumberjack.Write(p)
}
