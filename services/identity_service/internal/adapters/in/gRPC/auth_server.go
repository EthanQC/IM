package grpc

import (
	"context"
	"strconv"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/identity_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/in"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// AuthServer implements the shared IdentityService proto for MVP.
type AuthServer struct {
	imv1.UnimplementedIdentityServiceServer
	AuthUC in.AuthUseCase
	UserUC in.UserUseCase
	SMSUC  in.SMSUseCase
}

func NewAuthServer(authUC in.AuthUseCase, userUC in.UserUseCase, smsUC in.SMSUseCase) *AuthServer {
	return &AuthServer{AuthUC: authUC, UserUC: userUC, SMSUC: smsUC}
}

func (s *AuthServer) Register(ctx context.Context, req *imv1.RegisterRequest) (*imv1.AuthResponse, error) {
	// 调用 UserUseCase 注册
	user, err := s.UserUC.Register(ctx, req.Username, req.Password, req.DisplayName, nil, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register failed: %v", err)
	}

	// 注册成功后自动登录获取 token
	_, token, err := s.UserUC.Login(ctx, req.Username, req.Password)
	if err != nil {
		// 注册成功但登录失败，返回用户信息但没有 token
		return &imv1.AuthResponse{
			Profile: &imv1.UserProfile{
				User: &imv1.UserBrief{
					Id:          int64(user.ID),
					Username:    user.Username,
					DisplayName: user.DisplayName,
				},
				Status: "active",
			},
		}, nil
	}

	return &imv1.AuthResponse{
		AccessToken:  token,
		RefreshToken: "",  // MVP 简化
		ExpiresIn:    900, // 15分钟
		Profile: &imv1.UserProfile{
			User: &imv1.UserBrief{
				Id:          int64(user.ID),
				Username:    user.Username,
				DisplayName: user.DisplayName,
			},
			Status: "active",
		},
	}, nil
}

func (s *AuthServer) Login(ctx context.Context, req *imv1.LoginRequest) (*imv1.AuthResponse, error) {
	// 使用 UserUseCase.Login（更简洁的实现）
	user, token, err := s.UserUC.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "login failed: %v", err)
	}

	return &imv1.AuthResponse{
		AccessToken:  token,
		RefreshToken: "",  // MVP 简化
		ExpiresIn:    900, // 15分钟
		Profile: &imv1.UserProfile{
			User: &imv1.UserBrief{
				Id:          int64(user.ID),
				Username:    user.Username,
				DisplayName: user.DisplayName,
			},
			Status: "active",
		},
	}, nil
}

func (s *AuthServer) Refresh(ctx context.Context, req *imv1.RefreshRequest) (*imv1.AuthResponse, error) {
	at, err := s.AuthUC.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "refresh failed: %v", err)
	}
	return s.toAuthResp(at), nil
}

func (s *AuthServer) GetProfile(ctx context.Context, req *imv1.GetProfileRequest) (*imv1.UserProfile, error) {
	uid := req.UserId
	if uid == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id required")
	}

	// 使用 UserUseCase 获取真实用户数据
	user, err := s.UserUC.GetProfile(ctx, uint64(uid))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get profile failed: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}

	// 转换用户状态为可读字符串
	statusStr := "unknown"
	switch user.Status {
	case entity.UserStatusNormal:
		statusStr = "active"
	case entity.UserStatusDisabled:
		statusStr = "disabled"
	case entity.UserStatusFrozen:
		statusStr = "frozen"
	}

	return &imv1.UserProfile{
		User: &imv1.UserBrief{
			Id:          int64(user.ID),
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarUrl:   avatarURL,
		},
		Status: statusStr,
	}, nil
}

func (s *AuthServer) UpdateProfile(ctx context.Context, req *imv1.UpdateProfileRequest) (*imv1.UserProfile, error) {
	// 从 gRPC metadata 获取 user_id
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "missing metadata")
	}

	userIDStrs := md.Get("user_id")
	if len(userIDStrs) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id required")
	}

	userID, err := strconv.ParseUint(userIDStrs[0], 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}

	// 处理 avatar_url（转换为指针）
	var avatarURL *string
	if req.AvatarUrl != "" {
		avatarURL = &req.AvatarUrl
	}

	// 调用 UserUseCase 更新资料
	user, err := s.UserUC.UpdateProfile(ctx, userID, req.DisplayName, avatarURL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update profile failed: %v", err)
	}

	avatarURLStr := ""
	if user.AvatarURL != nil {
		avatarURLStr = *user.AvatarURL
	}

	// 转换用户状态为可读字符串
	statusStr := "unknown"
	switch user.Status {
	case entity.UserStatusNormal:
		statusStr = "active"
	case entity.UserStatusDisabled:
		statusStr = "disabled"
	case entity.UserStatusFrozen:
		statusStr = "frozen"
	}

	return &imv1.UserProfile{
		User: &imv1.UserBrief{
			Id:          int64(user.ID),
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarUrl:   avatarURLStr,
		},
		Status: statusStr,
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
