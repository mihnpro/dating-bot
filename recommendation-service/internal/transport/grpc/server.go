package grpc

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	recommendationv1 "github.com/mihnpro/DatingBotProtos/gen/go/recommendation/v1"

	"github.com/dating-bot/recommendation-service/internal/domain/entity"
	"github.com/dating-bot/recommendation-service/internal/service"
)

// Server implements recommendationv1.RecommendationServiceServer.
type Server struct {
	recommendationv1.UnimplementedRecommendationServiceServer
	svc *service.RecommendationService
}

func NewServer(svc *service.RecommendationService) *Server {
	return &Server{svc: svc}
}

// ── GetNextProfile ────────────────────────────────────────────────────────────

func (s *Server) GetNextProfile(
	ctx context.Context,
	req *recommendationv1.GetNextProfileRequest,
) (*recommendationv1.GetNextProfileResponse, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	profile, hasProfile, err := s.svc.GetNextProfile(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get next profile: %v", err)
	}

	resp := &recommendationv1.GetNextProfileResponse{
		HasProfile: hasProfile,
	}
	if hasProfile && profile != nil {
		resp.Profile = toProtoProfile(profile)
	}
	return resp, nil
}

// ── GetRecommendations ────────────────────────────────────────────────────────

func (s *Server) GetRecommendations(
	ctx context.Context,
	req *recommendationv1.GetRecommendationsRequest,
) (*recommendationv1.GetRecommendationsResponse, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	profiles, err := s.svc.GetRecommendations(ctx, req.UserId, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get recommendations: %v", err)
	}

	protoProfiles := make([]*recommendationv1.RecommendedProfile, 0, len(profiles))
	for _, p := range profiles {
		if p != nil {
			protoProfiles = append(protoProfiles, toProtoProfile(p))
		}
	}

	return &recommendationv1.GetRecommendationsResponse{
		Profiles: protoProfiles,
	}, nil
}

// ── GetRating ─────────────────────────────────────────────────────────────────

func (s *Server) GetRating(
	ctx context.Context,
	req *recommendationv1.GetRatingRequest,
) (*recommendationv1.GetRatingResponse, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	rating, err := s.svc.GetRating(ctx, req.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "rating not found for user %d", req.UserId)
		}
		return nil, status.Errorf(codes.Internal, "get rating: %v", err)
	}

	return &recommendationv1.GetRatingResponse{
		Rating: toProtoRating(rating),
	}, nil
}

// ── UpdateRating ──────────────────────────────────────────────────────────────

func (s *Server) UpdateRating(
	ctx context.Context,
	req *recommendationv1.UpdateRatingRequest,
) (*recommendationv1.UpdateRatingResponse, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	var primary, behavioral *float64
	if req.PrimaryRating != nil {
		v := *req.PrimaryRating
		primary = &v
	}
	if req.BehavioralRating != nil {
		v := *req.BehavioralRating
		behavioral = &v
	}

	rating, err := s.svc.UpdateRating(ctx, req.UserId, primary, behavioral)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update rating: %v", err)
	}

	return &recommendationv1.UpdateRatingResponse{
		Rating: toProtoRating(rating),
	}, nil
}

// ── TriggerRecalculation ──────────────────────────────────────────────────────

func (s *Server) TriggerRecalculation(
	ctx context.Context,
	req *recommendationv1.TriggerRecalculationRequest,
) (*emptypb.Empty, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	if err := s.svc.TriggerRecalculation(ctx, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "trigger recalculation: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ── Mappers ───────────────────────────────────────────────────────────────────

func toProtoProfile(p *entity.RecommendedProfile) *recommendationv1.RecommendedProfile {
	return &recommendationv1.RecommendedProfile{
		UserId:          p.UserID,
		Age:             p.Age,
		Gender:          p.Gender,
		City:            p.City,
		Interests:       p.Interests,
		PhotosCount:     p.PhotosCount,
		FullnessPercent: p.FullnessPercent,
		Score:           p.Score,
	}
}

func toProtoRating(r *entity.Rating) *recommendationv1.Rating {
	return &recommendationv1.Rating{
		UserId:           r.UserID,
		PrimaryRating:    r.PrimaryRating,
		BehavioralRating: r.BehavioralRating,
		CombinedRating:   r.CombinedRating,
		CalculatedAt:     timestamppb.New(r.CalculatedAt),
	}
}
