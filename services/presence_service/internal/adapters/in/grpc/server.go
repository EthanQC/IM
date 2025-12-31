package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/presence_service/internal/ports/in"
)

// PresenceServer gRPC在线状态服务
type PresenceServer struct {
	pb.UnimplementedPresenceServiceServer
	presenceUseCase in.PresenceUseCase
}

// NewPresenceServer 创建在线状态服务
func NewPresenceServer(presenceUseCase in.PresenceUseCase) *PresenceServer {
	return &PresenceServer{presenceUseCase: presenceUseCase}
}

// RegisterPresenceServiceServer 注册服务
func RegisterPresenceServiceServer(s *grpc.Server, srv *PresenceServer) {
	pb.RegisterPresenceServiceServer(s, srv)
}

// ReportOnline 上报上线
func (s *PresenceServer) ReportOnline(ctx context.Context, req *pb.ReportOnlineRequest) (*emptypb.Empty, error) {
	err := s.presenceUseCase.ReportOnline(ctx, uint64(req.UserId), req.NodeId, "")
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ReportOffline 上报下线
func (s *PresenceServer) ReportOffline(ctx context.Context, req *pb.ReportOfflineRequest) (*emptypb.Empty, error) {
	err := s.presenceUseCase.ReportOffline(ctx, uint64(req.UserId), req.NodeId)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// GetOnline 获取在线状态
func (s *PresenceServer) GetOnline(ctx context.Context, req *pb.GetOnlineRequest) (*pb.GetOnlineResponse, error) {
	userIDs := make([]uint64, len(req.UserIds))
	for i, id := range req.UserIds {
		userIDs[i] = uint64(id)
	}

	presences, err := s.presenceUseCase.GetPresences(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	items := make([]*pb.GetOnlineResponse_Item, 0, len(presences))
	for userID, presence := range presences {
		item := &pb.GetOnlineResponse_Item{
			UserId:       int64(userID),
			Online:       presence.Online,
			NodeId:       presence.NodeID,
			LastSeenUnix: presence.LastSeenAt.Unix(),
		}
		if presence.LastSeenAt.IsZero() {
			item.LastSeenUnix = time.Now().Unix()
		}
		items = append(items, item)
	}

	return &pb.GetOnlineResponse{Items: items}, nil
}
