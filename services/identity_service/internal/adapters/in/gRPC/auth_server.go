package grpc

import (
	"context"
	"strconv"
	"time"

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
	ContactUC in.ContactUseCase
	SMSUC  in.SMSUseCase
}

func NewAuthServer(authUC in.AuthUseCase, userUC in.UserUseCase, contactUC in.ContactUseCase, smsUC in.SMSUseCase) *AuthServer {
	return &AuthServer{AuthUC: authUC, UserUC: userUC, ContactUC: contactUC, SMSUC: smsUC}
}

func (s *AuthServer) Register(ctx context.Context, req *imv1.RegisterRequest) (*imv1.AuthResponse, error) {
	// 调用 UserUseCase 注册
	user, err := s.UserUC.Register(ctx, req.Username, req.Password, req.DisplayName, nil, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register failed: %v", err)
	}

	// 注册成功后自动登录获取 token
	at, err := s.AuthUC.LoginByPassword(ctx, req.Username, req.Password)
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
		AccessToken:  at.AccessToken,
		RefreshToken: at.RefreshToken,
		ExpiresIn:    expiresInSeconds(at),
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
	at, err := s.AuthUC.LoginByPassword(ctx, req.Username, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "login failed: %v", err)
	}

	userID, err := strconv.ParseUint(at.UserID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user id: %v", err)
	}

	user, err := s.UserUC.GetProfile(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get profile failed: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	return &imv1.AuthResponse{
		AccessToken:  at.AccessToken,
		RefreshToken: at.RefreshToken,
		ExpiresIn:    expiresInSeconds(at),
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
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
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

func (s *AuthServer) ApplyContact(ctx context.Context, req *imv1.ApplyContactRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var remark *string
	if req.Remark != "" {
		remark = &req.Remark
	}

	if err := s.ContactUC.ApplyContact(ctx, userID, uint64(req.TargetUserId), remark); err != nil {
		return nil, status.Errorf(codes.Internal, "apply contact failed: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *AuthServer) RespondContact(ctx context.Context, req *imv1.RespondContactRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.ContactUC.RespondContact(ctx, uint64(req.TargetUserId), userID, req.Accept); err != nil {
		return nil, status.Errorf(codes.Internal, "respond contact failed: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *AuthServer) RemoveContact(ctx context.Context, req *imv1.RemoveContactRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.ContactUC.RemoveContact(ctx, userID, uint64(req.TargetUserId)); err != nil {
		return nil, status.Errorf(codes.Internal, "remove contact failed: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *AuthServer) AddToBlacklist(ctx context.Context, req *imv1.BlacklistRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.ContactUC.AddToBlacklist(ctx, userID, uint64(req.UserId)); err != nil {
		return nil, status.Errorf(codes.Internal, "add to blacklist failed: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *AuthServer) RemoveFromBlacklist(ctx context.Context, req *imv1.BlacklistRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.ContactUC.RemoveFromBlacklist(ctx, userID, uint64(req.UserId)); err != nil {
		return nil, status.Errorf(codes.Internal, "remove from blacklist failed: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *AuthServer) ListContacts(ctx context.Context, req *imv1.ListContactsRequest) (*imv1.ListContactsResponse, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	page := int(req.Page)
	pageSize := int(req.PageSize)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	contacts, total, err := s.ContactUC.ListContacts(ctx, userID, page, pageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list contacts failed: %v", err)
	}

	resp := &imv1.ListContactsResponse{Total: int32(total)}
	for _, contact := range contacts {
		user, err := s.UserUC.GetProfile(ctx, contact.FriendID)
		if err != nil || user == nil {
			continue
		}

		avatarURL := ""
		if user.AvatarURL != nil {
			avatarURL = *user.AvatarURL
		}
		resp.Contacts = append(resp.Contacts, &imv1.UserBrief{
			Id:          int64(user.ID),
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarUrl:   avatarURL,
		})
	}
	return resp, nil
}

// RegisterServer registers the gRPC server implementation.
func (s *AuthServer) RegisterServer(gs *grpc.Server) {
	imv1.RegisterIdentityServiceServer(gs, s)
}

func (s *AuthServer) toAuthResp(at *entity.AuthToken) *imv1.AuthResponse {
	return &imv1.AuthResponse{
		AccessToken:  at.AccessToken,
		RefreshToken: at.RefreshToken,
		ExpiresIn:    expiresInSeconds(at),
		Profile:      &imv1.UserProfile{},
	}
}

func expiresInSeconds(at *entity.AuthToken) int64 {
	if at == nil {
		return 0
	}
	if at.ExpiresAt.IsZero() {
		return 0
	}
	ttl := time.Until(at.ExpiresAt).Seconds()
	if ttl < 0 {
		return 0
	}
	return int64(ttl)
}

func getUserIDFromContext(ctx context.Context) (uint64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, status.Errorf(codes.InvalidArgument, "missing metadata")
	}

	userIDStrs := md.Get("user_id")
	if len(userIDStrs) == 0 {
		return 0, status.Errorf(codes.InvalidArgument, "user_id required")
	}

	userID, err := strconv.ParseUint(userIDStrs[0], 10, 64)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	return userID, nil
}
