package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dating-bot/chat-service/internal/domain/entity"
)

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(ctx context.Context, msg *entity.Message) error {
	query := `
		INSERT INTO messages (conversation_id, sender_id, content, content_type, sent_at, is_read)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`
	return r.db.QueryRowContext(ctx, query,
		msg.ConversationID, msg.SenderID, msg.Content,
		string(msg.ContentType), msg.SentAt, msg.IsRead,
	).Scan(&msg.ID)
}

func (r *MessageRepository) GetByConversationID(ctx context.Context, convID string, limit, offset int) ([]*entity.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, content, content_type, sent_at, is_read
		FROM messages
		WHERE conversation_id = $1
		ORDER BY sent_at ASC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, convID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var msgs []*entity.Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *MessageRepository) MarkAsRead(ctx context.Context, convID string, userID int64) error {
	query := `
		UPDATE messages
		SET is_read = true
		WHERE conversation_id = $1 AND sender_id != $2 AND is_read = false`
	_, err := r.db.ExecContext(ctx, query, convID, userID)
	return err
}

func (r *MessageRepository) GetUnreadCount(ctx context.Context, convID string, userID int64) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM messages
		WHERE conversation_id = $1 AND sender_id != $2 AND is_read = false`
	err := r.db.QueryRowContext(ctx, query, convID, userID).Scan(&count)
	return count, err
}

func scanMessage(rows *sql.Rows) (*entity.Message, error) {
	m := &entity.Message{}
	var contentType string
	err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &contentType, &m.SentAt, &m.IsRead)
	if err != nil {
		return nil, fmt.Errorf("scan message: %w", err)
	}
	m.ContentType = entity.ContentType(contentType)
	return m, nil
}
