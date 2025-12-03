package grpc

import (
	"context"
	"fmt"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/domain/vo"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/in"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// AuthServer implements the shared IdentityService proto for MVP.
type AuthServer struct {
	imv1.UnimplementedIdentityServiceServer
	AuthUC in.AuthUseCase
	SMSUC  in.SMSUseCase
}

func NewAuthServer(authUC in.AuthUseCase, smsUC in.SMSUseCase) *AuthServer {
	return &AuthServer{AuthUC: authUC, SMSUC: smsUC}
}

func (s *AuthServer) Register(ctx context.Context, _ *imv1.RegisterRequest) (*imv1.AuthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "register not implemented")
}

func (s *AuthServer) Login(ctx context.Context, req *imv1.LoginRequest) (*imv1.AuthResponse, error) {
	pw, err := vo.NewPassword(req.Password)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid password: %v", err)
	}
	at, err := s.AuthUC.LoginByPassword(ctx, req.Username, *pw)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "login failed: %v", err)
	}
	return s.toAuthResp(at), nil
}

func (s *AuthServer) Refresh(ctx context.Context, req *imv1.RefreshRequest) (*imv1.AuthResponse, error) {
	at, err := s.AuthUC.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "refresh failed: %v", err)
	}
	return s.toAuthResp(at), nil
}

func (s *AuthServer) GetProfile(ctx context.Context, req *imv1.GetProfileRequest) (*imv1.UserProfile, error) {
	// MVP stub: echo user id, status active
	uid := req.UserId
	if uid == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id required")
	}
	return &imv1.UserProfile{
		User:   &imv1.UserBrief{Id: uid, Username: fmt.Sprintf("user-%d", uid), DisplayName: fmt.Sprintf("User %d", uid)},
		Status: "active",
	}, nil
}

func (s *AuthServer) UpdateProfile(ctx context.Context, req *imv1.UpdateProfileRequest) (*imv1.UserProfile, error) {
	// MVP stub: no persistence
	return &imv1.UserProfile{
		User:   &imv1.UserBrief{DisplayName: req.DisplayName, AvatarUrl: req.AvatarUrl},
		Status: "active",
	}, nil
}

func (s *AuthServer) ApplyContact(context.Context, *imv1.ApplyContactRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, status.Errorf(codes.Unimplemented, "contacts not implemented")
}

func (s *AuthServer) RespondContact(context.Context, *imv1.RespondContactRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, status.Errorf(codes.Unimplemented, "contacts not implemented")
}

func (s *AuthServer) RemoveContact(context.Context, *imv1.RemoveContactRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, status.Errorf(codes.Unimplemented, "contacts not implemented")
}

func (s *AuthServer) AddToBlacklist(context.Context, *imv1.BlacklistRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, status.Errorf(codes.Unimplemented, "blacklist not implemented")
}

func (s *AuthServer) RemoveFromBlacklist(context.Context, *imv1.BlacklistRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, status.Errorf(codes.Unimplemented, "blacklist not implemented")
}

func (s *AuthServer) ListContacts(context.Context, *imv1.ListContactsRequest) (*imv1.ListContactsResponse, error) {
	return &imv1.ListContactsResponse{}, status.Errorf(codes.Unimplemented, "contacts not implemented")
}

// RegisterServer registers the gRPC server implementation.
func (s *AuthServer) RegisterServer(gs *grpc.Server) {
	imv1.RegisterIdentityServiceServer(gs, s)
}

func (s *AuthServer) toAuthResp(at *entity.AuthToken) *imv1.AuthResponse {
	return &imv1.AuthResponse{
		AccessToken:  at.AccessToken,
		RefreshToken: at.RefreshToken,
		ExpiresIn:    int64(at.RefreshExpiresAt.Sub(at.CreatedAt).Seconds()),
		Profile:      &imv1.UserProfile{},
	}
}
