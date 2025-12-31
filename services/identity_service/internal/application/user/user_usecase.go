package user

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/in"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/out"
	"github.com/EthanQC/IM/services/identity_service/pkg/jwt"
)

var (
	ErrUsernameTaken     = errors.New("username already taken")
	ErrPhoneTaken        = errors.New("phone already taken")
	ErrEmailTaken        = errors.New("email already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserInactive      = errors.New("user is inactive")
)

type UserUseCaseImpl struct {
	userRepo    out.UserRepository
	jwtManager  *jwt.Manager
	eventPub    out.EventPublisher
}

var _ in.UserUseCase = (*UserUseCaseImpl)(nil)

func NewUserUseCaseImpl(
	userRepo out.UserRepository,
	jwtManager *jwt.Manager,
	eventPub out.EventPublisher,
) *UserUseCaseImpl {
	return &UserUseCaseImpl{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		eventPub:   eventPub,
	}
}

func (uc *UserUseCaseImpl) Register(ctx context.Context, username, password, displayName string, phone, email *string) (*entity.User, error) {
	// 检查用户名是否已存在
	exists, err := uc.userRepo.ExistsByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("check username exists: %w", err)
	}
	if exists {
		return nil, ErrUsernameTaken
	}

	// 检查手机号是否已存在
	if phone != nil && *phone != "" {
		exists, err := uc.userRepo.ExistsByPhone(ctx, *phone)
		if err != nil {
			return nil, fmt.Errorf("check phone exists: %w", err)
		}
		if exists {
			return nil, ErrPhoneTaken
		}
	}

	// 检查邮箱是否已存在
	if email != nil && *email != "" {
		exists, err := uc.userRepo.ExistsByEmail(ctx, *email)
		if err != nil {
			return nil, fmt.Errorf("check email exists: %w", err)
		}
		if exists {
			return nil, ErrEmailTaken
		}
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// 创建用户
	user := &entity.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Phone:        phone,
		Email:        email,
		DisplayName:  displayName,
		Status:       entity.UserStatusNormal,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// 发布用户注册事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":      "user.registered",
			"user_id":   user.ID,
			"username":  user.Username,
			"timestamp": user.CreatedAt,
		}
		_ = uc.eventPub.Publish(ctx, "user-events", event)
	}

	return user, nil
}

func (uc *UserUseCaseImpl) Login(ctx context.Context, username, password string) (*entity.User, string, error) {
	// 获取用户
	user, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, "", fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, "", ErrInvalidCredentials
	}

	// 检查用户状态
	if !user.CanLogin() {
		return nil, "", ErrUserInactive
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// 生成JWT Token
	token, err := uc.jwtManager.GenerateAccessToken(fmt.Sprintf("%d", user.ID), user.Username)
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	// 发布登录事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":      "user.logged_in",
			"user_id":   user.ID,
			"username":  user.Username,
			"timestamp": user.CreatedAt,
		}
		_ = uc.eventPub.Publish(ctx, "user-events", event)
	}

	return user, token, nil
}

func (uc *UserUseCaseImpl) GetProfile(ctx context.Context, userID uint64) (*entity.User, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (uc *UserUseCaseImpl) UpdateProfile(ctx context.Context, userID uint64, displayName string, avatarURL *string) (*entity.User, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	user.Update(displayName, avatarURL)

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	// 发布用户更新事件
	if uc.eventPub != nil {
		event := map[string]interface{}{
			"type":         "user.updated",
			"user_id":      user.ID,
			"display_name": user.DisplayName,
			"timestamp":    user.UpdatedAt,
		}
		_ = uc.eventPub.Publish(ctx, "user-events", event)
	}

	return user, nil
}
