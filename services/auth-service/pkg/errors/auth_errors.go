package errors

import "errors"

var (
	// 身份验证相关
	ErrInvalidToken = errors.New("无效或已过期的令牌")

	// 验证码相关
	ErrInvalidAuthCode = errors.New("验证码错误")
	ErrCodeExpired     = errors.New("验证码已过期")

	// 值对象验证相关
	ErrInvalidPhone    = errors.New("手机号码错误")
	ErrInvalidPassword = errors.New("密码错误")
)
