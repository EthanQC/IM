package grpc

import (
	"context"

	"github.com/EthanQC/IM/services/auth-service/internal/domain/vo"
	"github.com/EthanQC/IM/services/auth-service/internal/ports/in"
	authpb "github.com/EthanQC/IM/services/auth-service/internal/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AuthServer struct {
	authpb.UnimplementedAuthServiceServer
	AuthUC in.AuthUseCase
	SMSUC  in.SMSUseCase
}

func NewAuthServer(authUC in.AuthUseCase, smsUC in.SMSUseCase) *AuthServer {
	return &AuthServer{AuthUC: authUC, SMSUC: smsUC}
}

func (s *AuthServer) SendCode(ctx context.Context, req *authpb.SendCodeRequest) (*authpb.SendCodeResponse, error) {
	phoneVO, err := vo.NewPhone(req.Phone)
	if err != nil {
		return &authpb.SendCodeResponse{Success: false, Error: err.Error()}, nil
	}
	if err := s.SMSUC.SendSMSCode(ctx, *phoneVO, req.Ip); err != nil {
		return &authpb.SendCodeResponse{Success: false, Error: err.Error()}, nil
	}
	return &authpb.SendCodeResponse{Success: true}, nil
}

func (s *AuthServer) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	pwdVO, err := vo.NewPassword(req.Password)
	if err != nil {
		return &authpb.LoginResponse{Error: err.Error()}, nil
	}
	at, err := s.AuthUC.LoginByPassword(ctx, req.Identifier, *pwdVO)
	if err != nil {
		return &authpb.LoginResponse{Error: err.Error()}, nil
	}
	return &authpb.LoginResponse{
		AccessToken:  at.AccessToken,
		RefreshToken: at.RefreshToken,
	}, nil
}

func (s *AuthServer) LoginBySMS(ctx context.Context, req *authpb.SMSLoginRequest) (*authpb.SMSLoginResponse, error) {
	phoneVO, err := vo.NewPhone(req.Phone)
	if err != nil {
		return &authpb.SMSLoginResponse{Error: err.Error()}, nil
	}
	at, err := s.AuthUC.LoginBySMS(ctx, *phoneVO, req.Code)
	if err != nil {
		return &authpb.SMSLoginResponse{Error: err.Error()}, nil
	}
	return &authpb.SMSLoginResponse{
		AccessToken:  at.AccessToken,
		RefreshToken: at.RefreshToken,
	}, nil
}

func (s *AuthServer) RefreshToken(ctx context.Context, req *authpb.RefreshRequest) (*authpb.RefreshResponse, error) {
	at, err := s.AuthUC.RefreshToken(ctx, req.RefreshJti)
	if err != nil {
		return &authpb.RefreshResponse{Error: err.Error()}, nil
	}
	return &authpb.RefreshResponse{
		AccessToken:  at.AccessToken,
		RefreshToken: at.RefreshToken,
	}, nil
}

func (s *AuthServer) Logout(ctx context.Context, req *authpb.LogoutRequest) (*emptypb.Empty, error) {
	_ = s.AuthUC.Logout(ctx, req.AccessJti)
	return &emptypb.Empty{}, nil
}
