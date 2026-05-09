package repository

import (
	"context"

	"github.com/dating-bot/chat-service/internal/domain/entity"
	"github.com/dating-bot/chat-service/internal/domain/event"
)

type ConversationRepository interface {
	Create(ctx context.Context, conv *entity.Conversation) error
	GetByID(ctx context.Context, id string) (*entity.Conversation, error)
	GetByMatchID(ctx context.Context, matchID int64) (*entity.Conversation, error)
	GetByUserID(ctx context.Context, userID int64) ([]*entity.Conversation, error)
}

type MessageRepository interface {
	Create(ctx context.Context, msg *entity.Message) error
	GetByConversationID(ctx context.Context, convID string, limit, offset int) ([]*entity.Message, error)
	MarkAsRead(ctx context.Context, convID string, userID int64) error
	GetUnreadCount(ctx context.Context, convID string, userID int64) (int, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, evt *event.DomainEvent) error
	Close()
}

type EventSubscriber interface {
	Subscribe(exchange, queue string, handler func([]byte) error) error
	Close()
}
