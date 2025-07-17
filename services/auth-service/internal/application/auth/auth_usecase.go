package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/model"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/out"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrTokenExpired       = errors.New("token expired")
)

type AuthUseCase struct {
	userRepo   out.UserRepository
	tokenRepo  out.TokenRepository
	cryptoUtil out.CryptoUtil
}

func NewAuthUseCase(userRepo out.UserRepository, tokenRepo out.TokenRepository, cryptoUtil out.CryptoUtil) *AuthUseCase {
	return &AuthUseCase{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		cryptoUtil: cryptoUtil,
	}
}

// Login handles user login with phone and password
func (uc *AuthUseCase) Login(ctx context.Context, phone, password string) (*model.TokenPair, error) {
	user, err := uc.userRepo.FindByPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	if !uc.cryptoUtil.ComparePasswords(user.Password, password) {
		return nil, ErrInvalidCredentials
	}

	// Generate token pair
	accessToken, err := uc.generateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := uc.generateRefreshToken(user)
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	return &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Register handles new user registration
func (uc *AuthUseCase) Register(ctx context.Context, phone, password string) error {
	exists, err := uc.userRepo.ExistsByPhone(ctx, phone)
	if err != nil {
		return fmt.Errorf("checking user existence: %w", err)
	}

	if exists {
		return errors.New("phone number already registered")
	}

	hashedPassword, err := uc.cryptoUtil.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user := &model.User{
		Phone:     phone,
		Password:  hashedPassword,
		CreatedAt: time.Now(),
	}

	if err := uc.userRepo.Save(ctx, user); err != nil {
		return fmt.Errorf("saving user: %w", err)
	}

	return nil
}

// ValidateToken validates the given access token
func (uc *AuthUseCase) ValidateToken(ctx context.Context, token string) (*model.User, error) {
	claims, err := uc.cryptoUtil.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("validating token: %w", err)
	}

	user, err := uc.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	return user, nil
}

// RefreshToken refreshes the access token using a valid refresh token
func (uc *AuthUseCase) RefreshToken(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	claims, err := uc.cryptoUtil.ValidateToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("validating refresh token: %w", err)
	}

	user, err := uc.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	newAccessToken, err := uc.generateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("generating new access token: %w", err)
	}

	newRefreshToken, err := uc.generateRefreshToken(user)
	if err != nil {
		return nil, fmt.Errorf("generating new refresh token: %w", err)
	}

	return &model.TokenPair{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (uc *AuthUseCase) generateAccessToken(user *model.User) (string, error) {
	return uc.cryptoUtil.GenerateToken(user.ID, 24*time.Hour) // 24 hour expiry
}

func (uc *AuthUseCase) generateRefreshToken(user *model.User) (string, error) {
	return uc.cryptoUtil.GenerateToken(user.ID, 7*24*time.Hour) // 7 days expiry
}
