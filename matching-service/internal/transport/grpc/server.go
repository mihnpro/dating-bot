package grpc

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	matchingv1 "github.com/mihnpro/DatingBotProtos/gen/go/matching/v1"

	"github.com/dating-bot/matching-service/internal/domain/entity"
	"github.com/dating-bot/matching-service/internal/service"
)

type Server struct {
	matchingv1.UnimplementedMatchingServiceServer
	svc *service.MatchingService
}

func NewServer(svc *service.MatchingService) *Server {
	return &Server{svc: svc}
}

func (s *Server) Like(ctx context.Context, req *matchingv1.LikeRequest) (*matchingv1.LikeResponse, error) {
	interaction, isMatch, match, err := s.svc.Like(ctx, req.FromUserId, req.ToUserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "like: %v", err)
	}

	resp := &matchingv1.LikeResponse{
		IsMatch:     isMatch,
		Interaction: toInteractionProto(interaction),
	}
	if match != nil {
		resp.Match = toMatchProto(match)
	}

	return resp, nil
}

func (s *Server) Pass(ctx context.Context, req *matchingv1.PassRequest) (*matchingv1.PassResponse, error) {
	interaction, err := s.svc.Pass(ctx, req.FromUserId, req.ToUserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pass: %v", err)
	}
	return &matchingv1.PassResponse{Interaction: toInteractionProto(interaction)}, nil
}

func (s *Server) UndoLike(ctx context.Context, req *matchingv1.UndoLikeRequest) (*emptypb.Empty, error) {
	if err := s.svc.UndoLike(ctx, req.FromUserId, req.ToUserId); err != nil {
		return nil, status.Errorf(codes.Internal, "undo like: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetMatch(ctx context.Context, req *matchingv1.GetMatchRequest) (*matchingv1.GetMatchResponse, error) {
	match, err := s.svc.GetMatch(ctx, req.MatchId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "match not found")
		}
		return nil, status.Errorf(codes.Internal, "get match: %v", err)
	}
	return &matchingv1.GetMatchResponse{Match: toMatchProto(match)}, nil
}

func (s *Server) GetUserMatches(ctx context.Context, req *matchingv1.GetUserMatchesRequest) (*matchingv1.GetUserMatchesResponse, error) {
	var matchStatus *entity.MatchStatus
	if req.Status != nil {
		st := entity.MatchStatus(*req.Status)
		matchStatus = &st
	}

	matches, total, err := s.svc.GetUserMatches(ctx, req.UserId, req.Page, req.PageSize, matchStatus)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user matches: %v", err)
	}

	var pbMatches []*matchingv1.Match
	for _, m := range matches {
		pbMatches = append(pbMatches, toMatchProto(m))
	}

	return &matchingv1.GetUserMatchesResponse{
		Matches:    pbMatches,
		TotalCount: total,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

func (s *Server) HasMatched(ctx context.Context, req *matchingv1.HasMatchedRequest) (*matchingv1.HasMatchedResponse, error) {
	matched, matchID, err := s.svc.HasMatched(ctx, req.User1Id, req.User2Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "has matched: %v", err)
	}

	resp := &matchingv1.HasMatchedResponse{Matched: wrapperspb.Bool(matched)}
	if matched {
		resp.MatchId = &matchID
	}
	return resp, nil
}

func (s *Server) GetInteractionHistory(ctx context.Context, req *matchingv1.GetInteractionHistoryRequest) (*matchingv1.GetInteractionHistoryResponse, error) {
	var interactionType *entity.InteractionType
	if req.Type != nil && *req.Type != "" && *req.Type != "all" {
		t := entity.InteractionType(*req.Type)
		interactionType = &t
	}

	interactions, total, err := s.svc.GetInteractionHistory(ctx, req.UserId, req.Page, req.PageSize, interactionType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get interaction history: %v", err)
	}

	var pbInteractions []*matchingv1.Interaction
	for _, i := range interactions {
		pbInteractions = append(pbInteractions, toInteractionProto(i))
	}

	return &matchingv1.GetInteractionHistoryResponse{
		Interactions: pbInteractions,
		TotalCount:   total,
		Page:         req.Page,
		PageSize:     req.PageSize,
	}, nil
}

func (s *Server) MarkConversationStarted(ctx context.Context, req *matchingv1.MarkConversationStartedRequest) (*emptypb.Empty, error) {
	if err := s.svc.MarkConversationStarted(ctx, req.MatchId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "match not found")
		}
		return nil, status.Errorf(codes.Internal, "mark conversation started: %v", err)
	}
	return &emptypb.Empty{}, nil
}

// --- Mappers ---

func toMatchProto(m *entity.Match) *matchingv1.Match {
	return &matchingv1.Match{
		Id:                  m.ID,
		User1Id:             m.User1ID,
		User2Id:             m.User2ID,
		CreatedAt:           timestamppb.New(m.CreatedAt),
		Status:              string(m.Status),
		ConversationStarted: m.ConversationStarted,
		LastInteractionAt:   timestamppb.New(m.LastInteractionAt),
	}
}

func toInteractionProto(i *entity.Interaction) *matchingv1.Interaction {
	return &matchingv1.Interaction{
		Id:         i.ID,
		FromUserId: i.FromUserID,
		ToUserId:   i.ToUserID,
		Type:       string(i.Type),
		CreatedAt:  timestamppb.New(i.CreatedAt),
		TimeOfDay:  int32(i.TimeOfDay),
	}
}
