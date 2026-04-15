package repository

import (
	"context"

	"github.com/dating-bot/matching-service/internal/domain/event"
)

type EventPublisher interface {
	Publish(ctx context.Context, topic string, evt *event.DomainEvent) error
	Close() error
}
