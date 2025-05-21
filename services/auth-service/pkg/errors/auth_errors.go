package errors

import "errors"

var (
	// 验证码相关
	ErrCodeExpired     = errors.New("验证码已过期")
	ErrTooManyAttempts = errors.New("验证尝试次数过多")
	ErrInvalidCode     = errors.New("验证码错误")

	// 值对象验证相关
	ErrInvalidPhone    = errors.New("手机号码错误")
	ErrInvalidPassword = errors.New("密码错误")

	// 令牌相关
	ErrTokenExpired = errors.New("令牌已过期")
	ErrTokenRevoked = errors.New("令牌已被撤销")
	ErrInvalidToken = errors.New("无效的令牌")

	// 权限相关
	ErrPermissionDenied = errors.New("没有访问权限")

	// 用户状态相关
	ErrUserBlocked = errors.New("用户已被封禁")

	// 令牌刷新相关
	ErrRefreshTokenExpired = errors.New("刷新令牌已过期")

	// 访问规则相关
	ErrInvalidAccessRule = errors.New("无效的访问规则")
)
