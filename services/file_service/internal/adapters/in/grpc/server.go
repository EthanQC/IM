package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/EthanQC/IM/api/gen/im/v1"
	"github.com/EthanQC/IM/services/file_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/file_service/internal/ports/in"
)

// FileServer gRPC文件服务
type FileServer struct {
	pb.UnimplementedFileServiceServer
	fileUseCase in.FileUseCase
}

// NewFileServer 创建文件服务
func NewFileServer(fileUseCase in.FileUseCase) *FileServer {
	return &FileServer{fileUseCase: fileUseCase}
}

// RegisterFileServiceServer 注册服务
func RegisterFileServiceServer(s *grpc.Server, srv *FileServer) {
	pb.RegisterFileServiceServer(s, srv)
}

// CreateUpload 创建上传
func (s *FileServer) CreateUpload(ctx context.Context, req *pb.CreateUploadRequest) (*pb.CreateUploadResponse, error) {
	// 从context获取userID（需要在拦截器中设置）
	userID := ctx.Value("user_id").(uint64)
	if userID == 0 {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
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
	userID := ctx.Value("user_id").(uint64)
	if userID == 0 {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}

	if req.Media == nil {
		return nil, status.Error(codes.InvalidArgument, "media is required")
	}

	file, err := s.fileUseCase.CompleteUpload(ctx, &in.CompleteUploadInput{
		UserID:         userID,
		ObjectKey:      req.Media.ObjectKey,
		ConversationID: uint64(req.ConversationId),
		ClientMsgID:    req.ClientMsgId,
		Width:          req.Media.Width,
		Height:         req.Media.Height,
		Duration:       req.Media.Duration,
		Thumbnail:      req.Media.Thumbnail,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.CompleteUploadResponse{
		Message: &pb.MessageItem{
			MediaRef: &pb.MediaRef{
				ObjectKey:   file.ObjectKey,
				Url:         file.URL,
				Mime:        file.ContentType,
				SizeBytes:   file.SizeBytes,
				Width:       file.Width,
				Height:      file.Height,
				Duration:    file.Duration,
				Thumbnail:   file.Thumbnail,
			},
		},
	}, nil
}
