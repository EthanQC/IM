package grpc

import (
	"context"
	"strconv"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/conversation_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/conversation_service/internal/ports/in"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ConversationServer gRPC服务实现
type ConversationServer struct {
	imv1.UnimplementedConversationServiceServer
	convUC in.ConversationUseCase
}

func NewConversationServer(convUC in.ConversationUseCase) *ConversationServer {
	return &ConversationServer{convUC: convUC}
}

func (s *ConversationServer) CreateConversation(ctx context.Context, req *imv1.CreateConversationRequest) (*imv1.ConversationBrief, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	var convType entity.ConversationType
	switch req.Type {
	case imv1.ConversationType_CONVERSATION_TYPE_SINGLE:
		convType = entity.ConversationTypeSingle
	case imv1.ConversationType_CONVERSATION_TYPE_GROUP:
		convType = entity.ConversationTypeGroup
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid conversation type")
	}

	memberIDs := make([]uint64, len(req.MemberIds))
	for i, id := range req.MemberIds {
		memberIDs[i] = uint64(id)
	}

	conv, err := s.convUC.CreateConversation(ctx, userID, convType, req.Title, memberIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create conversation failed: %v", err)
	}

	return toConversationBrief(conv), nil
}

func (s *ConversationServer) UpdateConversation(ctx context.Context, req *imv1.UpdateConversationRequest) (*imv1.ConversationBrief, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	var title *string
	if req.Title != "" {
		title = &req.Title
	}

	conv, err := s.convUC.UpdateConversation(ctx, userID, uint64(req.ConversationId), title, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update conversation failed: %v", err)
	}

	return toConversationBrief(conv), nil
}

func (s *ConversationServer) AddMembers(ctx context.Context, req *imv1.AddMembersRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	memberIDs := make([]uint64, len(req.UserIds))
	for i, id := range req.UserIds {
		memberIDs[i] = uint64(id)
	}

	if err := s.convUC.AddMembers(ctx, userID, uint64(req.ConversationId), memberIDs); err != nil {
		return nil, status.Errorf(codes.Internal, "add members failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *ConversationServer) RemoveMembers(ctx context.Context, req *imv1.RemoveMembersRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
	}

	memberIDs := make([]uint64, len(req.UserIds))
	for i, id := range req.UserIds {
		memberIDs[i] = uint64(id)
	}

	if err := s.convUC.RemoveMembers(ctx, userID, uint64(req.ConversationId), memberIDs); err != nil {
		return nil, status.Errorf(codes.Internal, "remove members failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *ConversationServer) GetMembers(ctx context.Context, req *imv1.GetMembersRequest) (*imv1.GetMembersResponse, error) {
	members, err := s.convUC.GetMembers(ctx, uint64(req.ConversationId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get members failed: %v", err)
	}

	var userBriefs []*imv1.UserBrief
	for _, m := range members {
		userBriefs = append(userBriefs, &imv1.UserBrief{
			Id: int64(m.UserID),
			// 其他字段需要从用户服务获取
		})
	}

	return &imv1.GetMembersResponse{Members: userBriefs}, nil
}

func (s *ConversationServer) ListMyConversations(ctx context.Context, req *imv1.ListMyConversationsRequest) (*imv1.ListMyConversationsResponse, error) {
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

	conversations, total, err := s.convUC.ListMyConversations(ctx, userID, page, pageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list conversations failed: %v", err)
	}

	var items []*imv1.ConversationBrief
	for _, c := range conversations {
		items = append(items, toConversationBrief(c))
	}

	return &imv1.ListMyConversationsResponse{
		Items: items,
		Total: int32(total),
	}, nil
}

// RegisterServer 注册gRPC服务
func (s *ConversationServer) RegisterServer(gs *grpc.Server) {
	imv1.RegisterConversationServiceServer(gs, s)
}

// 辅助函数
func toConversationBrief(conv *entity.Conversation) *imv1.ConversationBrief {
	title := ""
	if conv.Title != nil {
		title = *conv.Title
	}

	var convType imv1.ConversationType
	switch conv.Type {
	case entity.ConversationTypeSingle:
		convType = imv1.ConversationType_CONVERSATION_TYPE_SINGLE
	case entity.ConversationTypeGroup:
		convType = imv1.ConversationType_CONVERSATION_TYPE_GROUP
	}

	return &imv1.ConversationBrief{
		Id:    int64(conv.ID),
		Type:  convType,
		Title: title,
	}
}

func getUserIDFromContext(ctx context.Context) (uint64, error) {
	// 从 gRPC metadata 中获取 user_id
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	userIDStrs := md.Get("user_id")
	if len(userIDStrs) == 0 {
		return 0, status.Errorf(codes.Unauthenticated, "user_id not found in metadata")
	}

	userID, err := strconv.ParseUint(userIDStrs[0], 10, 64)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	return userID, nil
}
