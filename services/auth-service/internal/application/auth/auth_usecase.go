package auth

import (
	"context"
	"time"

	"github.com/EthanQC/IM/pkg/zlog"
	"github.com/EthanQC/IM/services/auth-service/internal/domain/entity"
	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/in"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
	"github.com/EthanQC/IM/services/auth-service/pkg/errors"
)

type AuthUseCase struct {
	refreshRepo out.RefreshTokenRepository
	accessRepo  out.AccessTokenRepository
	codeRepo    out.AuthCodeRepository
	eventBus    out.EventBus
}

func NewAuthUseCase(
	refreshRepo out.RefreshTokenRepository,
	accessRepo out.AccessTokenRepository,
	codeRepo out.AuthCodeRepository,
	eventBus out.EventBus,
) in.AuthUseCase {
	return &AuthUseCase{
		refreshRepo: refreshRepo,
		accessRepo:  accessRepo,
		codeRepo:    codeRepo,
		eventBus:    eventBus,
	}
}

// RefreshToken 刷新访问令牌
func (u *AuthUseCase) RefreshToken(ctx context.Context, refreshJTI string) (*entity.AuthToken, error) {
	at, err := u.refreshRepo.Find(ctx, refreshJTI)
	if err != nil {
		zlog.FromContext(ctx).Error("查找刷新令牌失败", zlog.Error(err))
		return nil, err
	}
	if at == nil {
		return nil, errors.ErrInvalidToken
	}
	if err := at.Refresh(); err != nil {
		zlog.FromContext(ctx).Error("令牌刷新失败", zlog.Error(err))
		return nil, err
	}
	// 更新数据库中的 expires_at
	if err := u.refreshRepo.UpdateExpiry(ctx, refreshJTI, at.ExpiresAt.Unix()); err != nil {
		zlog.FromContext(ctx).Error("更新刷新令牌过期时间失败", zlog.Error(err))
		return nil, err
	}
	_ = u.eventBus.Publish(ctx, "user.token_refreshed", at)
	zlog.FromContext(ctx).Info("刷新令牌成功并发布事件", zlog.String("jti", refreshJTI))
	return at, nil
}

// LoginByPassword 密码登录（示例占位，实际应调用用户服务校验）
func (u *AuthUseCase) LoginByPassword(ctx context.Context, identifier string, password vo.Password) (*entity.AuthToken, error) {
	// TODO: 调用用户服务校验 identifier & password.Verify(...)
	// 这里只示例直接生成
	at := entity.NewAuthToken(identifier)
	if err := u.refreshRepo.Save(ctx, at); err != nil {
		zlog.FromContext(ctx).Error("保存 AuthToken 失败", zlog.Error(err))
		return nil, err
	}
	_ = u.eventBus.Publish(ctx, "user.authenticated", at)
	zlog.FromContext(ctx).Info("密码登录成功并发布事件", zlog.String("user_id", identifier))
	return at, nil
}

// LoginBySMS 短信验证码登录
func (u *AuthUseCase) LoginBySMS(ctx context.Context, phone vo.Phone, code string) (*entity.AuthToken, error) {
	ac, err := u.codeRepo.Find(ctx, phone.Number)
	if err != nil {
		zlog.FromContext(ctx).Error("查找验证码失败", zlog.Error(err))
		return nil, err
	}
	if ac == nil || ac.Used || ac.Code != code || time.Now().After(ac.ExpireTime) {
		return nil, errors.ErrInvalidCode
	}
	// 删除或标记已用
	if err := u.codeRepo.Delete(ctx, phone.Number); err != nil {
		zlog.FromContext(ctx).Error("删除验证码失败", zlog.Error(err))
		return nil, err
	}
	at := entity.NewAuthToken(phone.Number)
	if err := u.refreshRepo.Save(ctx, at); err != nil {
		zlog.FromContext(ctx).Error("保存 AuthToken 失败", zlog.Error(err))
		return nil, err
	}
	_ = u.eventBus.Publish(ctx, "user.authenticated", at)
	zlog.FromContext(ctx).Info("短信登录成功并发布事件", zlog.String("phone", phone.Number))
	return at, nil
}

// Logout 注销（撤销访问令牌）
func (u *AuthUseCase) Logout(ctx context.Context, accessJTI string) error {
	if err := u.accessRepo.Revoke(ctx, accessJTI); err != nil {
		zlog.FromContext(ctx).Error("撤销访问令牌失败", zlog.Error(err))
		return err
	}
	_ = u.eventBus.Publish(ctx, "user.logged_out", map[string]string{"jti": accessJTI})
	zlog.FromContext(ctx).Info("用户登出并发布事件", zlog.String("jti", accessJTI))
	return nil
}
