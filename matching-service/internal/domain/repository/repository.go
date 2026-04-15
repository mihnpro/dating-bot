package repository

import (
	"context"

	"github.com/dating-bot/matching-service/internal/domain/entity"
)

type MatchRepository interface {
	Create(ctx context.Context, match *entity.Match) error
	GetByID(ctx context.Context, id int64) (*entity.Match, error)
	GetByUserIDs(ctx context.Context, user1ID, user2ID int64) (*entity.Match, error)
	GetByUserID(ctx context.Context, userID int64, page, pageSize int32, status *entity.MatchStatus) ([]*entity.Match, int32, error)
	Update(ctx context.Context, match *entity.Match) error
}

type InteractionRepository interface {
	Create(ctx context.Context, interaction *entity.Interaction) error
	GetByUserPair(ctx context.Context, fromUserID, toUserID int64) (*entity.Interaction, error)
	GetByUserID(ctx context.Context, userID int64, page, pageSize int32, t *entity.InteractionType) ([]*entity.Interaction, int32, error)
	Delete(ctx context.Context, fromUserID, toUserID int64, t entity.InteractionType) error
}
