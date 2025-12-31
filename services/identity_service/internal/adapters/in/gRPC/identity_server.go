package grpc

import (
	"context"
	"strconv"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/identity_service/internal/ports/in"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// IdentityServer 身份服务gRPC实现
type IdentityServer struct {
	imv1.UnimplementedIdentityServiceServer
	userUC    in.UserUseCase
	contactUC in.ContactUseCase
	authUC    in.AuthUseCase
	smsUC     in.SMSUseCase
}

func NewIdentityServer(
	userUC in.UserUseCase,
	contactUC in.ContactUseCase,
	authUC in.AuthUseCase,
	smsUC in.SMSUseCase,
) *IdentityServer {
	return &IdentityServer{
		userUC:    userUC,
		contactUC: contactUC,
		authUC:    authUC,
		smsUC:     smsUC,
	}
}

func (s *IdentityServer) Register(ctx context.Context, req *imv1.RegisterRequest) (*imv1.AuthResponse, error) {
	// 参数校验
	if req.Username == "" || req.Password == "" || req.DisplayName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username, password and display_name are required")
	}

	// 调用用例注册
	user, err := s.userUC.Register(ctx, req.Username, req.Password, req.DisplayName, nil, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register failed: %v", err)
	}

	// 再次登录获取token
	_, token, err := s.userUC.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "login after register failed: %v", err)
	}

	return &imv1.AuthResponse{
		AccessToken: token,
		ExpiresIn:   3600, // 1小时
		Profile: &imv1.UserProfile{
			User: &imv1.UserBrief{
				Id:          int64(user.ID),
				Username:    user.Username,
				DisplayName: user.DisplayName,
				AvatarUrl:   ptrToStr(user.AvatarURL),
			},
			Status: "active",
		},
	}, nil
}

func (s *IdentityServer) Login(ctx context.Context, req *imv1.LoginRequest) (*imv1.AuthResponse, error) {
	// 参数校验
	if req.Username == "" || req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username and password are required")
	}

	// 调用用例登录
	user, token, err := s.userUC.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "login failed: %v", err)
	}

	return &imv1.AuthResponse{
		AccessToken: token,
		ExpiresIn:   3600,
		Profile: &imv1.UserProfile{
			User: &imv1.UserBrief{
				Id:          int64(user.ID),
				Username:    user.Username,
				DisplayName: user.DisplayName,
				AvatarUrl:   ptrToStr(user.AvatarURL),
			},
			Status: "active",
		},
	}, nil
}

func (s *IdentityServer) Refresh(ctx context.Context, req *imv1.RefreshRequest) (*imv1.AuthResponse, error) {
	if s.authUC == nil {
		return nil, status.Errorf(codes.Unimplemented, "refresh not implemented")
	}
	at, err := s.authUC.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "refresh failed: %v", err)
	}
	return &imv1.AuthResponse{
		AccessToken:  at.AccessToken,
		RefreshToken: at.RefreshToken,
		ExpiresIn:    int64(at.RefreshExpiresAt.Sub(at.CreatedAt).Seconds()),
		Profile:      &imv1.UserProfile{},
	}, nil
}

func (s *IdentityServer) GetProfile(ctx context.Context, req *imv1.GetProfileRequest) (*imv1.UserProfile, error) {
	if req.UserId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "user_id required")
	}

	user, err := s.userUC.GetProfile(ctx, uint64(req.UserId))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return &imv1.UserProfile{
		User: &imv1.UserBrief{
			Id:          int64(user.ID),
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarUrl:   ptrToStr(user.AvatarURL),
		},
		Status: userStatusToString(user.Status),
	}, nil
}

func (s *IdentityServer) UpdateProfile(ctx context.Context, req *imv1.UpdateProfileRequest) (*imv1.UserProfile, error) {
	// 从context中获取当前用户ID（通常由JWT中间件设置）
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	var avatarURL *string
	if req.AvatarUrl != "" {
		avatarURL = &req.AvatarUrl
	}

	user, err := s.userUC.UpdateProfile(ctx, userID, req.DisplayName, avatarURL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update profile failed: %v", err)
	}

	return &imv1.UserProfile{
		User: &imv1.UserBrief{
			Id:          int64(user.ID),
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarUrl:   ptrToStr(user.AvatarURL),
		},
		Status: userStatusToString(user.Status),
	}, nil
}

// 联系人相关接口
func (s *IdentityServer) ApplyContact(ctx context.Context, req *imv1.ApplyContactRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	var message *string
	if req.Remark != "" {
		message = &req.Remark
	}

	if err := s.contactUC.ApplyContact(ctx, userID, uint64(req.TargetUserId), message); err != nil {
		return nil, status.Errorf(codes.Internal, "apply contact failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *IdentityServer) RespondContact(ctx context.Context, req *imv1.RespondContactRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	// 注意：这里假设target_user_id作为申请ID使用，实际应该是申请ID
	if err := s.contactUC.RespondContact(ctx, uint64(req.TargetUserId), userID, req.Accept); err != nil {
		return nil, status.Errorf(codes.Internal, "respond contact failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *IdentityServer) RemoveContact(ctx context.Context, req *imv1.RemoveContactRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	if err := s.contactUC.RemoveContact(ctx, userID, uint64(req.TargetUserId)); err != nil {
		return nil, status.Errorf(codes.Internal, "remove contact failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *IdentityServer) AddToBlacklist(ctx context.Context, req *imv1.BlacklistRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	if err := s.contactUC.AddToBlacklist(ctx, userID, uint64(req.UserId)); err != nil {
		return nil, status.Errorf(codes.Internal, "add to blacklist failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *IdentityServer) RemoveFromBlacklist(ctx context.Context, req *imv1.BlacklistRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	if err := s.contactUC.RemoveFromBlacklist(ctx, userID, uint64(req.UserId)); err != nil {
		return nil, status.Errorf(codes.Internal, "remove from blacklist failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *IdentityServer) ListContacts(ctx context.Context, req *imv1.ListContactsRequest) (*imv1.ListContactsResponse, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	page := int(req.Page)
	pageSize := int(req.PageSize)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	contacts, total, err := s.contactUC.ListContacts(ctx, userID, page, pageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list contacts failed: %v", err)
	}

	var contactList []*imv1.UserBrief
	for _, c := range contacts {
		contactList = append(contactList, &imv1.UserBrief{
			Id: int64(c.FriendID),
			// 这里应该关联查询用户信息
		})
	}

	return &imv1.ListContactsResponse{
		Contacts: contactList,
		Total:    int32(total),
	}, nil
}

// RegisterServer 注册gRPC服务
func (s *IdentityServer) RegisterServer(gs *grpc.Server) {
	imv1.RegisterIdentityServiceServer(gs, s)
}

// 辅助函数
func ptrToStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func userStatusToString(status interface{}) string {
	// 根据实际状态类型进行转换
	return "active"
}

func getUserIDFromContext(ctx context.Context) (uint64, error) {
	// 从context中获取用户ID，通常由认证中间件设置
	// 这里是简化实现，实际应该从metadata中获取
	userIDStr, ok := ctx.Value("user_id").(string)
	if !ok {
		return 0, status.Errorf(codes.Unauthenticated, "user_id not found in context")
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return userID, nil
}
