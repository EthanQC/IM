package formatter

import (
	"encoding/json"
	"time"
)

type JsonFormatter struct{}

func (f *JsonFormatter) Format(level core.Level, msg string, fields []core.Field) ([]byte, error) {
	// 将日志内容格式化为JSON
	data := map[string]interface{}{
		"level":     level,
		"message":   msg,
		"timestamp": time.Now(),
	}

	// 添加额外字段
	for _, field := range fields {
		data[field.Key] = field.Value
	}

	return json.Marshal(data)
}
