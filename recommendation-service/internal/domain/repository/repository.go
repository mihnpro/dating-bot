package repository

import (
	"context"

	"github.com/dating-bot/recommendation-service/internal/domain/entity"
)

// RatingRepository defines persistence operations for user ratings.
type RatingRepository interface {
	// Upsert inserts or updates the full rating record for a user.
	Upsert(ctx context.Context, rating *entity.Rating) error

	// GetByUserID returns the current rating for a user.
	// Returns sql.ErrNoRows when the user has no rating yet.
	GetByUserID(ctx context.Context, userID int64) (*entity.Rating, error)

	// GetCandidates returns up to limit ratings ordered by combined_rating DESC,
	// filtered by gender and excluding the given userIDs.
	// Used to build the recommendation feed for a viewer.
	GetCandidates(ctx context.Context, gender string, excludeUserIDs []int64, limit int) ([]*entity.Rating, error)

	// GetAll returns every rating record.
	// Used by the periodic full-recalculation worker.
	GetAll(ctx context.Context) ([]*entity.Rating, error)

	// LogChange appends a rating_log row recording why the combined score changed.
	LogChange(ctx context.Context, userID int64, oldCombined, newCombined float64, reason string) error
}

// EventPublisher is the write-side of the domain event bus.
type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, evt *entity.DomainEvent) error
	Close() error
}

// EventSubscriber is the read-side of the domain event bus.
type EventSubscriber interface {
	// Subscribe binds queue to exchange and starts consuming.
	// handler is called in a dedicated goroutine for every delivered message.
	Subscribe(queue, exchange string, handler func(evt *entity.DomainEvent) error) error
	Close() error
}
