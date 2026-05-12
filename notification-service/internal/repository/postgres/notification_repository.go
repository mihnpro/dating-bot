package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dating-bot/notification-service/internal/domain/entity"
	"github.com/dating-bot/notification-service/internal/domain/repository"
)

type notificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) repository.NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(ctx context.Context, n *entity.Notification) error {
	const q = `
		INSERT INTO notifications (user_id, type, message, is_sent, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	return r.db.QueryRowContext(ctx, q,
		n.UserID, string(n.Type), n.Message, n.IsSent, n.IsRead, n.CreatedAt,
	).Scan(&n.ID)
}

func (r *notificationRepository) MarkSent(ctx context.Context, id int64) error {
	now := time.Now()
	const q = `UPDATE notifications SET is_sent = true, sent_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, q, now, id)
	return err
}

func (r *notificationRepository) MarkRead(ctx context.Context, id int64) error {
	const q = `UPDATE notifications SET is_read = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}

func (r *notificationRepository) GetByUserID(
	ctx context.Context, userID int64, limit, offset int,
) ([]*entity.Notification, error) {
	const q = `
		SELECT id, user_id, type, message, is_sent, is_read, created_at, sent_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	var out []*entity.Notification
	for rows.Next() {
		n := &entity.Notification{}
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.Type, &n.Message,
			&n.IsSent, &n.IsRead, &n.CreatedAt, &n.SentAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *notificationRepository) CountUnread(ctx context.Context, userID int64) (int, error) {
	const q = `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`
	var count int
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&count)
	return count, err
}
