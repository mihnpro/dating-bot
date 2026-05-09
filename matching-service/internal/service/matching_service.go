package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dating-bot/matching-service/internal/domain/entity"
	"github.com/dating-bot/matching-service/internal/domain/event"
	"github.com/dating-bot/matching-service/internal/domain/repository"
)

type MatchingService struct {
	matchRepo       repository.MatchRepository
	interactionRepo repository.InteractionRepository
	publisher       repository.EventPublisher
}

func NewMatchingService(
	matchRepo repository.MatchRepository,
	interactionRepo repository.InteractionRepository,
	publisher repository.EventPublisher,
) *MatchingService {
	return &MatchingService{
		matchRepo:       matchRepo,
		interactionRepo: interactionRepo,
		publisher:       publisher,
	}
}

func (s *MatchingService) Like(ctx context.Context, fromUserID, toUserID int64) (*entity.Interaction, bool, *entity.Match, error) {
	// Check if already interacted
	existing, _ := s.interactionRepo.GetByUserPair(ctx, fromUserID, toUserID)
	if existing != nil {
		return existing, false, nil, nil
	}

	// Record the like
	interaction := entity.NewInteraction(fromUserID, toUserID, entity.InteractionTypeLike)
	if err := s.interactionRepo.Create(ctx, interaction); err != nil {
		return nil, false, nil, err
	}

	// Publish event
	evt, _ := event.NewInteractionLikedEvent(fromUserID, toUserID)
	if evt != nil {
		_ = s.publisher.Publish(ctx, "interaction.liked", evt)
	}

	// Check if target user already liked us → mutual match
	reverse, _ := s.interactionRepo.GetByUserPair(ctx, toUserID, fromUserID)
	if reverse != nil && reverse.Type == entity.InteractionTypeLike {
		// Create match
		match := entity.NewMatch(fromUserID, toUserID)
		if err := s.matchRepo.Create(ctx, match); err != nil {
			return interaction, true, nil, err
		}

		matchEvt, _ := event.NewMatchCreatedEvent(match)
		if matchEvt != nil {
			_ = s.publisher.Publish(ctx, "match.created", matchEvt)
		}

		return interaction, true, match, nil
	}

	return interaction, false, nil, nil
}

func (s *MatchingService) Pass(ctx context.Context, fromUserID, toUserID int64) (*entity.Interaction, error) {
	existing, _ := s.interactionRepo.GetByUserPair(ctx, fromUserID, toUserID)
	if existing != nil {
		return existing, nil
	}

	interaction := entity.NewInteraction(fromUserID, toUserID, entity.InteractionTypePass)
	if err := s.interactionRepo.Create(ctx, interaction); err != nil {
		return nil, err
	}

	return interaction, nil
}

func (s *MatchingService) UndoLike(ctx context.Context, fromUserID, toUserID int64) error {
	return s.interactionRepo.Delete(ctx, fromUserID, toUserID, entity.InteractionTypeLike)
}

func (s *MatchingService) GetMatch(ctx context.Context, matchID int64) (*entity.Match, error) {
	return s.matchRepo.GetByID(ctx, matchID)
}

func (s *MatchingService) GetUserMatches(ctx context.Context, userID int64, page, pageSize int32, status *entity.MatchStatus) ([]*entity.Match, int32, error) {
	return s.matchRepo.GetByUserID(ctx, userID, page, pageSize, status)
}

func (s *MatchingService) HasMatched(ctx context.Context, user1ID, user2ID int64) (bool, int64, error) {
	match, err := s.matchRepo.GetByUserIDs(ctx, user1ID, user2ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, nil
		}
		return false, 0, err
	}
	return true, match.ID, nil
}

func (s *MatchingService) GetInteractionHistory(ctx context.Context, userID int64, page, pageSize int32, t *entity.InteractionType) ([]*entity.Interaction, int32, error) {
	return s.interactionRepo.GetByUserID(ctx, userID, page, pageSize, t)
}

func (s *MatchingService) GetWhoLikedMe(ctx context.Context, toUserID int64, page, pageSize int32) ([]int64, int32, error) {
	return s.interactionRepo.GetWhoLikedMe(ctx, toUserID, page, pageSize)
}

func (s *MatchingService) MarkConversationStarted(ctx context.Context, matchID int64) error {
	match, err := s.matchRepo.GetByID(ctx, matchID)
	if err != nil {
		return err
	}
	match.StartConversation()
	return s.matchRepo.Update(ctx, match)
}
