package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dating-bot/matching-service/internal/domain/entity"
)

type matchRepository struct {
	db *sql.DB
}

func NewMatchRepository(db *sql.DB) *matchRepository {
	return &matchRepository{db: db}
}

func (r *matchRepository) Create(ctx context.Context, match *entity.Match) error {
	query := `
		INSERT INTO matches (user1_id, user2_id, created_at, status, conversation_started, last_interaction_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	return r.db.QueryRowContext(ctx, query,
		match.User1ID,
		match.User2ID,
		match.CreatedAt,
		string(match.Status),
		match.ConversationStarted,
		match.LastInteractionAt,
	).Scan(&match.ID)
}

func (r *matchRepository) GetByID(ctx context.Context, id int64) (*entity.Match, error) {
	query := `
		SELECT id, user1_id, user2_id, created_at, status, conversation_started, last_interaction_at
		FROM matches WHERE id = $1
	`
	m := &entity.Match{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&m.ID, &m.User1ID, &m.User2ID, &m.CreatedAt,
		&m.Status, &m.ConversationStarted, &m.LastInteractionAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get match by id: %w", err)
	}
	m.Status = entity.MatchStatus(m.Status)
	return m, nil
}

func (r *matchRepository) GetByUserIDs(ctx context.Context, user1ID, user2ID int64) (*entity.Match, error) {
	query := `
		SELECT id, user1_id, user2_id, created_at, status, conversation_started, last_interaction_at
		FROM matches WHERE (user1_id = $1 AND user2_id = $2) OR (user1_id = $2 AND user2_id = $1)
	`
	m := &entity.Match{}
	err := r.db.QueryRowContext(ctx, query, user1ID, user2ID).Scan(
		&m.ID, &m.User1ID, &m.User2ID, &m.CreatedAt,
		&m.Status, &m.ConversationStarted, &m.LastInteractionAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get match by user ids: %w", err)
	}
	m.Status = entity.MatchStatus(m.Status)
	return m, nil
}

func (r *matchRepository) GetByUserID(ctx context.Context, userID int64, page, pageSize int32, status *entity.MatchStatus) ([]*entity.Match, int32, error) {
	var total int32
	countQuery := `SELECT COUNT(*) FROM matches WHERE user1_id = $1 OR user2_id = $1`
	args := []any{userID}
	idx := 2

	if status != nil {
		countQuery += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, string(*status))
		idx++
	}

	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `
		SELECT id, user1_id, user2_id, created_at, status, conversation_started, last_interaction_at
		FROM matches WHERE user1_id = $1 OR user2_id = $1
	`
	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", idx-1)
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, int(pageSize), int(offset))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var matches []*entity.Match
	for rows.Next() {
		m := &entity.Match{}
		if err := rows.Scan(
			&m.ID, &m.User1ID, &m.User2ID, &m.CreatedAt,
			&m.Status, &m.ConversationStarted, &m.LastInteractionAt,
		); err != nil {
			return nil, 0, err
		}
		m.Status = entity.MatchStatus(m.Status)
		matches = append(matches, m)
	}

	return matches, total, nil
}

func (r *matchRepository) Update(ctx context.Context, match *entity.Match) error {
	query := `
		UPDATE matches SET status = $1, conversation_started = $2, last_interaction_at = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query,
		string(match.Status),
		match.ConversationStarted,
		match.LastInteractionAt,
		match.ID,
	)
	return err
}
