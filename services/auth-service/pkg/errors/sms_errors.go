package errors

import "errors"

var (
	// 验证码相关
	ErrCodeExpired     = errors.New("验证码已过期")
	ErrTooManyAttempts = errors.New("验证尝试次数过多")
	ErrInvalidCode     = errors.New("验证码错误")
)
