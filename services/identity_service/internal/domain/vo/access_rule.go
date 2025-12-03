package vo

import "regexp"

type AccessRule struct {
	Path     string   // 访问路径
	Pattern  string   // 正则匹配模式
	Methods  []string // 允许的 HTTP 方法
	IsPublic bool     // 是否公开访问
}

func NewAccessRule(path string, pattern string, methods []string, isPublic bool) *AccessRule {
	return &AccessRule{
		Path:     path,
		Pattern:  pattern,
		Methods:  methods,
		IsPublic: isPublic,
	}
}

// 检查请求是否匹配规则
func (ar *AccessRule) Matches(path string, method string) bool {
	// 检查正则匹配
	if ar.Pattern != "" {
		matched, _ := regexp.MatchString(ar.Pattern, path)

		if !matched {
			return false
		}

	} else if ar.Path != path {
		return false
	}

	// 检查HTTP方法
	for _, m := range ar.Methods {
		if m == method {
			return true
		}
	}

	return false
}
