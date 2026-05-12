package repository

import (
	"context"

	"github.com/dating-bot/notification-service/internal/domain/entity"
)

type NotificationRepository interface {
	Create(ctx context.Context, n *entity.Notification) error
	MarkSent(ctx context.Context, id int64) error
	MarkRead(ctx context.Context, id int64) error
	GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*entity.Notification, error)
	CountUnread(ctx context.Context, userID int64) (int, error)
}
