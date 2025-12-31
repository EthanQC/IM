package server

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/message_service/internal/ports/in"
)

// MessageServer gRPC消息服务
type MessageServer struct {
	pb.UnimplementedMessageServiceServer
	messageUseCase in.MessageUseCase
}

// NewMessageServer 创建消息服务
func NewMessageServer(messageUseCase in.MessageUseCase) *MessageServer {
	return &MessageServer{messageUseCase: messageUseCase}
}

// RegisterMessageServiceServer 注册服务
func RegisterMessageServiceServer(s *grpc.Server, srv *MessageServer) {
	pb.RegisterMessageServiceServer(s, srv)
}

// SendMessage 发送消息
func (s *MessageServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	if req.ConversationId == 0 || req.SenderId == 0 || req.ClientMsgId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
	}

	content := entity.MessageContent{}
	if req.Content != nil {
		if req.Content.Text != nil {
			content.Text = &entity.TextContent{
				Content:   req.Content.Text.Content,
				Mentions:  make([]uint64, len(req.Content.Text.Mentions)),
				MentionAll: req.Content.Text.MentionAll,
			}
			copy(content.Text.Mentions, req.Content.Text.Mentions)
		}
		if req.Content.Media != nil {
			content.Media = &entity.MediaContent{
				URL:       req.Content.Media.Url,
				Thumbnail: req.Content.Media.Thumbnail,
				FileName:  req.Content.Media.FileName,
				FileSize:  req.Content.Media.FileSize,
				Duration:  req.Content.Media.Duration,
				Width:     req.Content.Media.Width,
				Height:    req.Content.Media.Height,
			}
		}
		if req.Content.Location != nil {
			content.Location = &entity.LocationContent{
				Latitude:  req.Content.Location.Latitude,
				Longitude: req.Content.Location.Longitude,
				Address:   req.Content.Location.Address,
				Name:      req.Content.Location.Name,
			}
		}
	}

	var replyToMsgID *uint64
	if req.ReplyToMsgId != 0 {
		replyToMsgID = &req.ReplyToMsgId
	}

	msg, err := s.messageUseCase.SendMessage(ctx, &in.SendMessageInput{
		ConversationID: req.ConversationId,
		SenderID:       req.SenderId,
		ClientMsgID:    req.ClientMsgId,
		ContentType:    entity.MessageContentType(req.ContentType),
		Content:        content,
		ReplyToMsgID:   replyToMsgID,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.SendMessageResponse{
		MessageId: msg.ID,
		Seq:       msg.Seq,
		CreatedAt: timestamppb.New(msg.CreatedAt),
	}, nil
}

// GetHistory 获取历史消息
func (s *MessageServer) GetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.GetHistoryResponse, error) {
	if req.ConversationId == 0 || req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	messages, err := s.messageUseCase.GetHistory(ctx, &in.GetHistoryInput{
		ConversationID: req.ConversationId,
		UserID:         req.UserId,
		AfterSeq:       req.AfterSeq,
		BeforeSeq:      req.BeforeSeq,
		Limit:          limit,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbMessages := make([]*pb.Message, len(messages))
	for i, msg := range messages {
		pbMessages[i] = entityToProto(msg)
	}

	return &pb.GetHistoryResponse{
		Messages: pbMessages,
	}, nil
}

// UpdateRead 更新已读状态
func (s *MessageServer) UpdateRead(ctx context.Context, req *pb.UpdateReadRequest) (*pb.UpdateReadResponse, error) {
	if req.ConversationId == 0 || req.UserId == 0 || req.ReadSeq == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
	}

	err := s.messageUseCase.UpdateRead(ctx, &in.UpdateReadInput{
		ConversationID: req.ConversationId,
		UserID:         req.UserId,
		ReadSeq:        req.ReadSeq,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.UpdateReadResponse{
		Success: true,
	}, nil
}

// RevokeMessage 撤回消息
func (s *MessageServer) RevokeMessage(ctx context.Context, req *pb.RevokeMessageRequest) (*pb.RevokeMessageResponse, error) {
	if req.MessageId == 0 || req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
	}

	err := s.messageUseCase.RevokeMessage(ctx, req.UserId, req.MessageId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.RevokeMessageResponse{
		Success: true,
	}, nil
}

// DeleteMessage 删除消息
func (s *MessageServer) DeleteMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*pb.DeleteMessageResponse, error) {
	if req.MessageId == 0 || req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
	}

	err := s.messageUseCase.DeleteMessage(ctx, req.UserId, req.MessageId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.DeleteMessageResponse{
		Success: true,
	}, nil
}

// GetUnreadCount 获取未读数
func (s *MessageServer) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
	if req.ConversationId == 0 || req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
	}

	count, err := s.messageUseCase.GetUnreadCount(ctx, req.UserId, req.ConversationId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.GetUnreadCountResponse{
		Count: int32(count),
	}, nil
}

// entityToProto 将实体转换为protobuf消息
func entityToProto(msg *entity.Message) *pb.Message {
	pbMsg := &pb.Message{
		Id:             msg.ID,
		ConversationId: msg.ConversationID,
		SenderId:       msg.SenderID,
		ClientMsgId:    msg.ClientMsgID,
		Seq:            msg.Seq,
		ContentType:    pb.MessageContentType(msg.ContentType),
		Status:         pb.MessageStatus(msg.Status),
		CreatedAt:      timestamppb.New(msg.CreatedAt),
		UpdatedAt:      timestamppb.New(msg.UpdatedAt),
	}

	if msg.ReplyToMsgID != nil {
		pbMsg.ReplyToMsgId = *msg.ReplyToMsgID
	}

	pbMsg.Content = &pb.MessageContent{}
	if msg.Content.Text != nil {
		pbMsg.Content.Text = &pb.TextContent{
			Content:    msg.Content.Text.Content,
			Mentions:   msg.Content.Text.Mentions,
			MentionAll: msg.Content.Text.MentionAll,
		}
	}
	if msg.Content.Media != nil {
		pbMsg.Content.Media = &pb.MediaContent{
			Url:       msg.Content.Media.URL,
			Thumbnail: msg.Content.Media.Thumbnail,
			FileName:  msg.Content.Media.FileName,
			FileSize:  msg.Content.Media.FileSize,
			Duration:  msg.Content.Media.Duration,
			Width:     msg.Content.Media.Width,
			Height:    msg.Content.Media.Height,
		}
	}
	if msg.Content.Location != nil {
		pbMsg.Content.Location = &pb.LocationContent{
			Latitude:  msg.Content.Location.Latitude,
			Longitude: msg.Content.Location.Longitude,
			Address:   msg.Content.Location.Address,
			Name:      msg.Content.Location.Name,
		}
	}

	return pbMsg
}
