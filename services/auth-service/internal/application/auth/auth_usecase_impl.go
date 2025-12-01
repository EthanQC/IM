package auth

import (
	"context"
	"fmt"

	"github.com/EthanQC/IM/services/auth-service/internal/application/sms"
	"github.com/EthanQC/IM/services/auth-service/internal/application/status"
	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/in"
)

// DefaultAuthUseCase 实现登录/刷新/登出，密码校验目前假设由上游完成（无用户存储）。
type DefaultAuthUseCase struct {
	generator *GenerateTokenUseCase
	refresher *RefreshTokenUseCase
	revoker   *RevokeTokenUseCase
	statusUC  *status.CheckUserStatusUseCase
	verifySMS *sms.VerifyCodeUseCase
}

var _ in.AuthUseCase = (*DefaultAuthUseCase)(nil)

func NewDefaultAuthUseCase(
	generator *GenerateTokenUseCase,
	refresher *RefreshTokenUseCase,
	revoker *RevokeTokenUseCase,
	statusUC *status.CheckUserStatusUseCase,
	verifySMS *sms.VerifyCodeUseCase,
) *DefaultAuthUseCase {
	return &DefaultAuthUseCase{
		generator: generator,
		refresher: refresher,
		revoker:   revoker,
		statusUC:  statusUC,
		verifySMS: verifySMS,
	}
}

// LoginByPassword 目前将 identifier 视为 userID，真实校验应交给用户服务或统一账号中心。
func (uc *DefaultAuthUseCase) LoginByPassword(ctx context.Context, identifier string, _ vo.Password) (*entity.AuthToken, error) {
	if err := uc.statusUC.Execute(ctx, identifier); err != nil {
		return nil, err
	}
	access, refresh, err := uc.generator.Execute(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	return uc.buildToken(identifier, access, refresh), nil
}

func (uc *DefaultAuthUseCase) LoginBySMS(ctx context.Context, phone vo.Phone, code string) (*entity.AuthToken, error) {
	if err := uc.verifySMS.Execute(ctx, phone, code); err != nil {
		return nil, err
	}
	if err := uc.statusUC.Execute(ctx, phone.Number); err != nil {
		return nil, err
	}
	access, refresh, err := uc.generator.Execute(ctx, phone.Number)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	return uc.buildToken(phone.Number, access, refresh), nil
}

func (uc *DefaultAuthUseCase) RefreshToken(ctx context.Context, refreshJTI string) (*entity.AuthToken, error) {
	at, err := uc.refresher.Execute(ctx, refreshJTI)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	return at, nil
}

func (uc *DefaultAuthUseCase) Logout(ctx context.Context, accessJTI string) error {
	return uc.revoker.Execute(ctx, accessJTI, false)
}

func (uc *DefaultAuthUseCase) buildToken(userID, access, refresh string) *entity.AuthToken {
	return &entity.AuthToken{
		UserID:       userID,
		AccessToken:  access,
		RefreshToken: refresh,
	}
}
