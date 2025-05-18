package entity

import "time"

type AuthCode struct {
	ID         string    // 验证码唯一标识
	Phone      string    // 手机号
	Code       string    // 验证码
	ExpireTime time.Time // 过期时间
	AttemptCnt int       // 验证尝试次数
	Used       bool      // 是否已使用
	CreateTime time.Time // 创建时间
	IPAddress  string    // 请求 IP，用于限流
}

func NewAuthCode(phone string, ip string) *AuthCode {
	return &AuthCode{
		Phone:      phone,
		CreateTime: time.Now(),
		ExpireTime: time.Now().Add(5 * time.Minute), // 五分钟有效期
		IPAddress:  ip,
	}
}

// 业务方法

// 验证码是否过期
func (ac *AuthCode) IsExpired() bool {
	return time.Now().After(ac.ExpireTime)
}

// 验证码是否能重发
func (ac *AuthCode) CanResend() bool {
	return time.Now().After(ac.CreateTime.Add(time.Minute)) // 一分钟内不能重发
}

// 增加尝试次数
func (ac *AuthCode) IncrementAttempt() {
	ac.AttemptCnt++
}

// 验证码尝试次数是否超过上限
func (ac *AuthCode) HasExceededMaxAttempts() bool {
	return ac.AttemptCnt >= 5 // 最多五次验证尝试
}
