package auth

import (
	"context"
	"fmt"

	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
	"github.com/EthanQC/IM/services/identity_service/pkg/jwt"
	jwtgo "github.com/dgrijalva/jwt-go"
)

type VerifyTokenUseCase struct {
	AccessRepo out.AccessTokenRepository
	StatusRepo out.UserStatusRepository
	JWTManager jwt.Manager
}

func NewVerifyTokenUseCase(
	accessRepo out.AccessTokenRepository,
	statusRepo out.UserStatusRepository,
	jwtMgr jwt.Manager,
) *VerifyTokenUseCase {
	return &VerifyTokenUseCase{AccessRepo: accessRepo, StatusRepo: statusRepo, JWTManager: jwtMgr}
}

// Execute 验证 AccessToken：验签 → 黑名单 → 用户状态
func (uc *VerifyTokenUseCase) Execute(ctx context.Context, tokenStr string) (*jwtgo.StandardClaims, error) {
	claims, err := uc.JWTManager.Parse(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("解析 AccessToken 失败: %w", err)
	}
	rec, err := uc.AccessRepo.Find(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("检查 Token 黑名单 失败: %w", err)
	}
	if rec != nil && rec.IsRevoked {
		return nil, fmt.Errorf("AccessToken 已被撤销")
	}
	status, err := uc.StatusRepo.Get(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("获取用户状态失败: %w", err)
	}
	if status != nil && !status.IsActive() {
		return nil, fmt.Errorf("用户已被禁用: %s", claims.Subject)
	}
	return claims, nil
}
