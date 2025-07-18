package jwt

import (
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// Manager 负责 JWT 的签发与解析
type Manager interface {
	Generate(jti, subject string, ttl time.Duration) (string, error)
	Parse(tokenStr string) (*jwt.StandardClaims, error)
}

type manager struct {
	secret []byte
}

// NewManager 用给定的 secret 构造 Manager
func NewManager(secret string) Manager {
	return &manager{secret: []byte(secret)}
}

// Generate 生成一个带 jti 和 subject 的 JWT，ttl 控制过期时间
func (m *manager) Generate(jti, subject string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.StandardClaims{
		Id:        jti,
		Subject:   subject,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Parse 验签并解析 JWT，返回 StandardClaims
func (m *manager) Parse(tokenStr string) (*jwt.StandardClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &jwt.StandardClaims{}, func(_ *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := tok.Claims.(*jwt.StandardClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
