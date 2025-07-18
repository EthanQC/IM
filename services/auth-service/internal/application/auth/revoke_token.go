package auth

import (
	"context"
	"fmt"

	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
)

type RevokeTokenUseCase struct {
	AccessRepo  out.AccessTokenRepository
	RefreshRepo out.RefreshTokenRepository
}

func NewRevokeTokenUseCase(
	accessRepo out.AccessTokenRepository,
	refreshRepo out.RefreshTokenRepository,
) *RevokeTokenUseCase {
	return &RevokeTokenUseCase{
		AccessRepo:  accessRepo,
		RefreshRepo: refreshRepo,
	}
}

// Execute 撤销令牌；isRefresh=true 则撤销 RefreshToken，否则撤销 AccessToken
func (uc *RevokeTokenUseCase) Execute(ctx context.Context, token string, isRefresh bool) error {
	if isRefresh {
		if err := uc.RefreshRepo.Revoke(ctx, token); err != nil {
			return fmt.Errorf("revoke refresh token: %w", err)
		}
	} else {
		if err := uc.AccessRepo.Revoke(ctx, token); err != nil {
			return fmt.Errorf("revoke access token: %w", err)
		}
	}
	return nil
}
