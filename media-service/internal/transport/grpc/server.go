package grpc

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	mediav1 "github.com/mihnpro/DatingBotProtos/gen/go/media/v1"

	"github.com/dating-bot/media-service/internal/service"
)

type Server struct {
	mediav1.UnimplementedMediaServiceServer
	svc *service.MediaService
}

func NewServer(svc *service.MediaService) *Server {
	return &Server{svc: svc}
}

func (s *Server) GetMedia(ctx context.Context, req *mediav1.GetMediaRequest) (*mediav1.GetMediaResponse, error) {
	m, err := s.svc.GetByID(ctx, req.MediaId)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			return nil, status.Error(codes.NotFound, "media not found")
		}
		return nil, status.Errorf(codes.Internal, "get media: %v", err)
	}
	return &mediav1.GetMediaResponse{Media: toProto(m)}, nil
}

func (s *Server) GetUserMedia(ctx context.Context, req *mediav1.GetUserMediaRequest) (*mediav1.GetUserMediaResponse, error) {
	items, err := s.svc.GetByUserID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user media: %v", err)
	}

	proto := make([]*mediav1.MediaItem, len(items))
	for i, m := range items {
		proto[i] = toProto(m)
	}
	return &mediav1.GetUserMediaResponse{Items: proto}, nil
}

func (s *Server) DeleteMedia(ctx context.Context, req *mediav1.DeleteMediaRequest) (*emptypb.Empty, error) {
	err := s.svc.Delete(ctx, req.MediaId, req.CallerUserId)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaNotFound):
			return nil, status.Error(codes.NotFound, "media not found")
		case errors.Is(err, service.ErrForbidden):
			return nil, status.Error(codes.PermissionDenied, "access denied")
		default:
			return nil, status.Errorf(codes.Internal, "delete media: %v", err)
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) SetMainPhoto(ctx context.Context, req *mediav1.SetMainPhotoRequest) (*emptypb.Empty, error) {
	err := s.svc.SetMain(ctx, req.MediaId, req.CallerUserId)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaNotFound):
			return nil, status.Error(codes.NotFound, "media not found")
		case errors.Is(err, service.ErrForbidden):
			return nil, status.Error(codes.PermissionDenied, "access denied")
		case errors.Is(err, sql.ErrNoRows):
			return nil, status.Error(codes.NotFound, "media not found")
		default:
			return nil, status.Errorf(codes.Internal, "set main photo: %v", err)
		}
	}
	return &emptypb.Empty{}, nil
}

func toProto(m *service.MediaWithURL) *mediav1.MediaItem {
	return &mediav1.MediaItem{
		MediaId:          m.ID,
		UserId:           m.UserID,
		Url:              m.URL,
		OriginalFilename: m.OriginalFilename,
		MimeType:         m.MimeType,
		FileSize:         m.FileSize,
		IsMain:           m.IsMain,
		UploadedAt:       timestamppb.New(m.UploadedAt),
	}
}
