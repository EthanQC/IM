package grpc

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/file_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/file_service/internal/ports/in"
)

// FileServer gRPC文件服务
type FileServer struct {
	pb.UnimplementedFileServiceServer
	fileUseCase in.FileUseCase
	messageClient pb.MessageServiceClient
}

// NewFileServer 创建文件服务
func NewFileServer(fileUseCase in.FileUseCase, messageClient pb.MessageServiceClient) *FileServer {
	return &FileServer{fileUseCase: fileUseCase, messageClient: messageClient}
}

// RegisterFileServiceServer 注册服务
func RegisterFileServiceServer(s *grpc.Server, srv *FileServer) {
	pb.RegisterFileServiceServer(s, srv)
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

// CreateUpload 创建上传
func (s *FileServer) CreateUpload(ctx context.Context, req *pb.CreateUploadRequest) (*pb.CreateUploadResponse, error) {
	// 从 metadata 获取 userID
	userID, err := getUserIDFromMetadata(ctx)
	if err != nil {
		return nil, err
	}

	kind := entity.FileKind(req.Kind)
	if kind == "" {
		kind = entity.FileKindFile
	}

	token, err := s.fileUseCase.CreateUpload(ctx, &in.CreateUploadInput{
		UserID:      userID,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		Kind:        kind,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CreateUploadResponse{
		ObjectKey:   token.ObjectKey,
		UploadUrl:   token.UploadURL,
		CallbackUrl: token.CallbackURL,
	}, nil
}

// CompleteUpload 完成上传
func (s *FileServer) CompleteUpload(ctx context.Context, req *pb.CompleteUploadRequest) (*pb.CompleteUploadResponse, error) {
	userID, err := getUserIDFromMetadata(ctx)
	if err != nil {
		return nil, err
	}

	if req.Media == nil {
		return nil, status.Error(codes.InvalidArgument, "media is required")
	}
	if req.ConversationId == 0 || req.ClientMsgId == "" {
		return nil, status.Error(codes.InvalidArgument, "conversation_id and client_msg_id are required")
	}

	file, err := s.fileUseCase.CompleteUpload(ctx, &in.CompleteUploadInput{
		UserID:         userID,
		ObjectKey:      req.Media.ObjectKey,
		ConversationID: uint64(req.ConversationId),
		ClientMsgID:    req.ClientMsgId,
		Width:          0, // proto MediaRef 中无此字段
		Height:         0, // proto MediaRef 中无此字段
		Duration:       req.Media.DurationSec,
		Thumbnail:      req.Media.ThumbnailKey,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if s.messageClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "message service not configured")
	}

	contentType, body := buildMessageBody(file)
	msgCtx := metadata.AppendToOutgoingContext(ctx, "user_id", strconv.FormatUint(userID, 10))
	msgResp, err := s.messageClient.SendMessage(msgCtx, &pb.SendMessageRequest{
		ConversationId: req.ConversationId,
		ClientMsgId:    req.ClientMsgId,
		ContentType:    contentType,
		Body:           body,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CompleteUploadResponse{
		Message: msgResp.Message,
	}, nil
}

func buildMessageBody(file *entity.FileUpload) (pb.MessageContentType, *pb.MessageBody) {
	media := &pb.MediaRef{
		ObjectKey:    file.ObjectKey,
		Filename:     file.FileName,
		ContentType:  file.ContentType,
		SizeBytes:    file.SizeBytes,
		DurationSec:  file.Duration,
		ThumbnailKey: file.Thumbnail,
	}

	switch file.Kind {
	case entity.FileKindImage:
		return pb.MessageContentType_MESSAGE_CONTENT_TYPE_IMAGE, &pb.MessageBody{Body: &pb.MessageBody_Image{Image: media}}
	case entity.FileKindAudio:
		return pb.MessageContentType_MESSAGE_CONTENT_TYPE_AUDIO, &pb.MessageBody{Body: &pb.MessageBody_Audio{Audio: media}}
	case entity.FileKindVideo:
		return pb.MessageContentType_MESSAGE_CONTENT_TYPE_VIDEO, &pb.MessageBody{Body: &pb.MessageBody_Video{Video: media}}
	default:
		return pb.MessageContentType_MESSAGE_CONTENT_TYPE_FILE, &pb.MessageBody{Body: &pb.MessageBody_File{File: media}}
	}
}
