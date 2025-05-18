package errors

import "errors"

var (
	// 错误定义
	ErrInvalidPhone    = errors.New("手机号码错误")
	ErrInvalidPassword = errors.New("密码错误")
	ErrInvalidAuthCode = errors.New("验证码错误")
	ErrCodeExpired     = errors.New("验证码已过期")
	ErrTooManyAttempts = errors.New("过多次验证尝试")
	ErrTooManyRequests = errors.New("过多次请求")
	ErrCodeResendLimit = errors.New("验证码重发过多次")
)
