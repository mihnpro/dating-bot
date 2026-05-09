package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dating-bot/chat-service/internal/domain/entity"
)

type ConversationRepository struct {
	db *sql.DB
}

func NewConversationRepository(db *sql.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) Create(ctx context.Context, conv *entity.Conversation) error {
	query := `
		INSERT INTO conversations (match_id, user1_id, user2_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`
	return r.db.QueryRowContext(ctx, query,
		conv.MatchID, conv.User1ID, conv.User2ID,
		string(conv.Status), conv.CreatedAt, conv.UpdatedAt,
	).Scan(&conv.ID)
}

func (r *ConversationRepository) GetByID(ctx context.Context, id string) (*entity.Conversation, error) {
	query := `
		SELECT id, match_id, user1_id, user2_id, status, created_at, updated_at
		FROM conversations WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanConversation(row)
}

func (r *ConversationRepository) GetByMatchID(ctx context.Context, matchID int64) (*entity.Conversation, error) {
	query := `
		SELECT id, match_id, user1_id, user2_id, status, created_at, updated_at
		FROM conversations WHERE match_id = $1`
	row := r.db.QueryRowContext(ctx, query, matchID)
	return scanConversation(row)
}

func (r *ConversationRepository) GetByUserID(ctx context.Context, userID int64) ([]*entity.Conversation, error) {
	query := `
		SELECT id, match_id, user1_id, user2_id, status, created_at, updated_at
		FROM conversations
		WHERE (user1_id = $1 OR user2_id = $1) AND status = 'active'
		ORDER BY updated_at DESC`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query conversations by user: %w", err)
	}
	defer rows.Close()

	var convs []*entity.Conversation
	for rows.Next() {
		conv, err := scanConversationRow(rows)
		if err != nil {
			return nil, err
		}
		convs = append(convs, conv)
	}
	return convs, rows.Err()
}

func scanConversation(row *sql.Row) (*entity.Conversation, error) {
	c := &entity.Conversation{}
	var status string
	err := row.Scan(&c.ID, &c.MatchID, &c.User1ID, &c.User2ID, &status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan conversation: %w", err)
	}
	c.Status = entity.ConversationStatus(status)
	return c, nil
}

func scanConversationRow(rows *sql.Rows) (*entity.Conversation, error) {
	c := &entity.Conversation{}
	var status string
	err := rows.Scan(&c.ID, &c.MatchID, &c.User1ID, &c.User2ID, &status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan conversation row: %w", err)
	}
	c.Status = entity.ConversationStatus(status)
	return c, nil
}
