package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
	"github.com/EthanQC/IM/services/identity_service/pkg/jwt"
)

type RefreshTokenUseCase struct {
	RefreshRepo out.RefreshTokenRepository
	StatusRepo  out.UserStatusRepository
	JWTManager  jwt.Manager
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

func NewRefreshTokenUseCase(
	refreshRepo out.RefreshTokenRepository,
	statusRepo out.UserStatusRepository,
	jwtMgr jwt.Manager,
	accessTTL, refreshTTL time.Duration,
) *RefreshTokenUseCase {
	return &RefreshTokenUseCase{
		RefreshRepo: refreshRepo,
		StatusRepo:  statusRepo,
		JWTManager:  jwtMgr,
		AccessTTL:   accessTTL,
		RefreshTTL:  refreshTTL,
	}
}

// Execute 用旧 RefreshToken 颁发新一对 Access/Refresh Token，并返回新的 Token 记录
func (uc *RefreshTokenUseCase) Execute(ctx context.Context, oldJTI string) (*entity.AuthToken, error) {
	rec, err := uc.RefreshRepo.Find(ctx, oldJTI)
	if err != nil {
		return nil, fmt.Errorf("查询 RefreshToken 失败: %w", err)
	}
	if rec == nil || rec.IsRevoked {
		return nil, fmt.Errorf("无效或已撤销的 RefreshToken")
	}
	if time.Now().After(rec.RefreshExpiresAt) {
		return nil, fmt.Errorf("RefreshToken 已过期")
	}

	status, err := uc.StatusRepo.Get(ctx, rec.UserID)
	if err != nil {
		return nil, fmt.Errorf("获取用户状态失败: %w", err)
	}
	if status != nil && !status.IsActive() {
		return nil, fmt.Errorf("用户已被禁用: %s", rec.UserID)
	}

	if err := uc.RefreshRepo.Revoke(ctx, oldJTI); err != nil {
		return nil, fmt.Errorf("撤销旧 RefreshToken 失败: %w", err)
	}

	at := entity.NewAuthToken(rec.UserID)
	at.ID = rec.ID
	at.RefreshExpiresAt = time.Now().Add(uc.RefreshTTL)

	accessToken, err := uc.JWTManager.Generate(at.ID, rec.UserID, uc.AccessTTL)
	if err != nil {
		return nil, fmt.Errorf("生成新 AccessToken 失败: %w", err)
	}
	at.AccessToken = accessToken

	refreshToken, err := uc.JWTManager.Generate(at.ID, rec.UserID, uc.RefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("生成新 RefreshToken 失败: %w", err)
	}
	at.RefreshToken = refreshToken

	if err := uc.RefreshRepo.Save(ctx, at); err != nil {
		return nil, fmt.Errorf("保存新 RefreshToken 失败: %w", err)
	}

	return at, nil
}
