package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"github.com/EthanQC/IM/services/auth-service/pkg/jwt"
)

type GenerateTokenUseCase struct {
	RefreshRepo out.RefreshTokenRepository
	StatusRepo  out.UserStatusRepository
	JWTManager  jwt.Manager
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

func NewGenerateTokenUseCase(
	refreshRepo out.RefreshTokenRepository,
	statusRepo out.UserStatusRepository,
	jwtMgr jwt.Manager,
	accessTTL,
	refreshTTL time.Duration,
) *GenerateTokenUseCase {
	return &GenerateTokenUseCase{
		RefreshRepo: refreshRepo,
		StatusRepo:  statusRepo,
		JWTManager:  jwtMgr,
		AccessTTL:   accessTTL,
		RefreshTTL:  refreshTTL,
	}
}

// Execute 签发一对 Access/Refresh Token，并持久化 RefreshToken
func (uc *GenerateTokenUseCase) Execute(ctx context.Context, userID string) (string, string, error) {
	status, err := uc.StatusRepo.Get(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("获取用户状态失败: %w", err)
	}
	if status != nil && !status.IsActive() {
		return "", "", fmt.Errorf("用户已被禁用: %s", userID)
	}

	at := entity.NewAuthToken(userID)
	at.ID = uuid.New().String()
	at.RefreshExpiresAt = time.Now().Add(uc.RefreshTTL)

	accessToken, err := uc.JWTManager.Generate(at.ID, userID, uc.AccessTTL)
	if err != nil {
		return "", "", fmt.Errorf("生成 AccessToken 失败: %w", err)
	}
	at.AccessToken = accessToken

	refreshToken, err := uc.JWTManager.Generate(at.ID, userID, uc.RefreshTTL)
	if err != nil {
		return "", "", fmt.Errorf("生成 RefreshToken 失败: %w", err)
	}
	at.RefreshToken = refreshToken

	if err := uc.RefreshRepo.Save(ctx, at); err != nil {
		return "", "", fmt.Errorf("保存 RefreshToken 失败: %w", err)
	}

	return accessToken, refreshToken, nil
}
