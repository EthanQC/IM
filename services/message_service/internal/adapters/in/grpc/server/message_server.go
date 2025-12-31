package server

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
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

// getUserIDFromMetadata 从 gRPC metadata 获取 user_id
func getUserIDFromMetadata(ctx context.Context) (uint64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, "missing metadata")
	}

	userIDStrs := md.Get("user_id")
	if len(userIDStrs) == 0 {
		return 0, status.Error(codes.Unauthenticated, "user_id not found in metadata")
	}

	userID, err := strconv.ParseUint(userIDStrs[0], 10, 64)
	if err != nil {
		return 0, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	return userID, nil
}

// SendMessage 发送消息
func (s *MessageServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	// 从 metadata 获取 userID
	userID, err := getUserIDFromMetadata(ctx)
	if err != nil {
		return nil, err
	}

	if req.ConversationId == 0 || req.ClientMsgId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
	}

	// 从 proto MessageBody 转换为 domain MessageContent
	content := s.bodyToContent(req.Body)

	msg, err := s.messageUseCase.SendMessage(ctx, &in.SendMessageRequest{
		ConversationID: uint64(req.ConversationId),
		SenderID:       userID,
		ClientMsgID:    req.ClientMsgId,
		ContentType:    entity.MessageContentType(req.ContentType),
		Content:        content,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.SendMessageResponse{
		Message: s.entityToMessageItem(msg),
	}, nil
}

// GetHistory 获取历史消息
func (s *MessageServer) GetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.GetHistoryResponse, error) {
	if req.ConversationId == 0 {
		return nil, status.Error(codes.InvalidArgument, "conversation_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	messages, err := s.messageUseCase.GetHistory(ctx, uint64(req.ConversationId), uint64(req.AfterSeq), limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	items := make([]*pb.MessageItem, len(messages))
	for i, msg := range messages {
		items[i] = s.entityToMessageItem(msg)
	}

	return &pb.GetHistoryResponse{
		Items: items,
	}, nil
}

// UpdateRead 更新已读状态
func (s *MessageServer) UpdateRead(ctx context.Context, req *pb.UpdateReadRequest) (*emptypb.Empty, error) {
	userID, err := getUserIDFromMetadata(ctx)
	if err != nil {
		return nil, err
	}

	if req.ConversationId == 0 || req.ReadSeq == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
	}

	err = s.messageUseCase.UpdateRead(ctx, userID, uint64(req.ConversationId), uint64(req.ReadSeq))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// bodyToContent 将 proto MessageBody 转换为 domain MessageContent
func (s *MessageServer) bodyToContent(body *pb.MessageBody) entity.MessageContent {
	content := entity.MessageContent{}
	if body == nil {
		return content
	}

	switch b := body.Body.(type) {
	case *pb.MessageBody_Text:
		if b.Text != nil {
			content.Text = &entity.TextContent{
				Text: b.Text.Text,
			}
		}
	case *pb.MessageBody_Image:
		if b.Image != nil {
			content.Image = &entity.MediaContent{
				ObjectKey:   b.Image.ObjectKey,
				Filename:    b.Image.Filename,
				ContentType: b.Image.ContentType,
				SizeBytes:   b.Image.SizeBytes,
			}
		}
	case *pb.MessageBody_File:
		if b.File != nil {
			content.File = &entity.MediaContent{
				ObjectKey:   b.File.ObjectKey,
				Filename:    b.File.Filename,
				ContentType: b.File.ContentType,
				SizeBytes:   b.File.SizeBytes,
			}
		}
	case *pb.MessageBody_Audio:
		if b.Audio != nil {
			content.Audio = &entity.MediaContent{
				ObjectKey:   b.Audio.ObjectKey,
				Filename:    b.Audio.Filename,
				ContentType: b.Audio.ContentType,
				SizeBytes:   b.Audio.SizeBytes,
				DurationSec: int(b.Audio.DurationSec),
			}
		}
	case *pb.MessageBody_Video:
		if b.Video != nil {
			content.Video = &entity.MediaContent{
				ObjectKey:    b.Video.ObjectKey,
				Filename:     b.Video.Filename,
				ContentType:  b.Video.ContentType,
				SizeBytes:    b.Video.SizeBytes,
				DurationSec:  int(b.Video.DurationSec),
				ThumbnailKey: b.Video.ThumbnailKey,
			}
		}
	}

	return content
}

// entityToMessageItem 将 domain Message 转换为 proto MessageItem
func (s *MessageServer) entityToMessageItem(msg *entity.Message) *pb.MessageItem {
	if msg == nil {
		return nil
	}

	item := &pb.MessageItem{
		Id:             int64(msg.ID),
		ConversationId: int64(msg.ConversationID),
		SenderId:       int64(msg.SenderID),
		Seq:            int64(msg.Seq),
		ContentType:    pb.MessageContentType(msg.ContentType),
		CreateTime:     timestamppb.New(msg.CreatedAt),
	}

	// 构建 MessageBody
	item.Body = s.contentToBody(msg.Content)

	return item
}

// contentToBody 将 domain MessageContent 转换为 proto MessageBody
func (s *MessageServer) contentToBody(content entity.MessageContent) *pb.MessageBody {
	body := &pb.MessageBody{}

	if content.Text != nil {
		body.Body = &pb.MessageBody_Text{
			Text: &pb.TextBody{
				Text: content.Text.Text,
			},
		}
	} else if content.Image != nil {
		body.Body = &pb.MessageBody_Image{
			Image: &pb.MediaRef{
				ObjectKey:   content.Image.ObjectKey,
				Filename:    content.Image.Filename,
				ContentType: content.Image.ContentType,
				SizeBytes:   content.Image.SizeBytes,
			},
		}
	} else if content.Audio != nil {
		body.Body = &pb.MessageBody_Audio{
			Audio: &pb.MediaRef{
				ObjectKey:   content.Audio.ObjectKey,
				Filename:    content.Audio.Filename,
				ContentType: content.Audio.ContentType,
				SizeBytes:   content.Audio.SizeBytes,
				DurationSec: int32(content.Audio.DurationSec),
			},
		}
	} else if content.Video != nil {
		body.Body = &pb.MessageBody_Video{
			Video: &pb.MediaRef{
				ObjectKey:    content.Video.ObjectKey,
				Filename:     content.Video.Filename,
				ContentType:  content.Video.ContentType,
				SizeBytes:    content.Video.SizeBytes,
				DurationSec:  int32(content.Video.DurationSec),
				ThumbnailKey: content.Video.ThumbnailKey,
			},
		}
	} else if content.File != nil {
		body.Body = &pb.MessageBody_File{
			File: &pb.MediaRef{
				ObjectKey:   content.File.ObjectKey,
				Filename:    content.File.Filename,
				ContentType: content.File.ContentType,
				SizeBytes:   content.File.SizeBytes,
			},
		}
	}

	return body
}
