package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dating-bot/matching-service/internal/domain/entity"
)

type interactionRepository struct {
	db *sql.DB
}

func NewInteractionRepository(db *sql.DB) *interactionRepository {
	return &interactionRepository{db: db}
}

func (r *interactionRepository) Create(ctx context.Context, interaction *entity.Interaction) error {
	query := `
		INSERT INTO interactions (from_user_id, to_user_id, type, created_at, time_of_day)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	return r.db.QueryRowContext(ctx, query,
		interaction.FromUserID,
		interaction.ToUserID,
		string(interaction.Type),
		interaction.CreatedAt,
		interaction.TimeOfDay,
	).Scan(&interaction.ID)
}

func (r *interactionRepository) GetByUserPair(ctx context.Context, fromUserID, toUserID int64) (*entity.Interaction, error) {
	query := `
		SELECT id, from_user_id, to_user_id, type, created_at, time_of_day
		FROM interactions WHERE from_user_id = $1 AND to_user_id = $2
		ORDER BY created_at DESC LIMIT 1
	`
	i := &entity.Interaction{}
	err := r.db.QueryRowContext(ctx, query, fromUserID, toUserID).Scan(
		&i.ID, &i.FromUserID, &i.ToUserID, &i.Type, &i.CreatedAt, &i.TimeOfDay,
	)
	if err != nil {
		return nil, fmt.Errorf("get interaction by user pair: %w", err)
	}
	i.Type = entity.InteractionType(i.Type)
	return i, nil
}

func (r *interactionRepository) GetByUserID(ctx context.Context, userID int64, page, pageSize int32, t *entity.InteractionType) ([]*entity.Interaction, int32, error) {
	var total int32
	countQuery := `SELECT COUNT(*) FROM interactions WHERE from_user_id = $1 OR to_user_id = $1`
	args := []any{userID}
	idx := 2

	if t != nil {
		countQuery += fmt.Sprintf(" AND type = $%d", idx)
		args = append(args, string(*t))
		idx++
	}

	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `
		SELECT id, from_user_id, to_user_id, type, created_at, time_of_day
		FROM interactions WHERE from_user_id = $1 OR to_user_id = $1
	`
	if t != nil {
		query += fmt.Sprintf(" AND type = $%d", idx-1)
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, int(pageSize), int(offset))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var interactions []*entity.Interaction
	for rows.Next() {
		i := &entity.Interaction{}
		if err := rows.Scan(
			&i.ID, &i.FromUserID, &i.ToUserID, &i.Type, &i.CreatedAt, &i.TimeOfDay,
		); err != nil {
			return nil, 0, err
		}
		i.Type = entity.InteractionType(i.Type)
		interactions = append(interactions, i)
	}

	return interactions, total, nil
}

func (r *interactionRepository) GetWhoLikedMe(ctx context.Context, toUserID int64, page, pageSize int32) ([]int64, int32, error) {
	var total int32
	countQuery := `
		SELECT COUNT(*) FROM interactions i
		WHERE i.to_user_id = $1 AND i.type = 'like'
		  AND NOT EXISTS (
		    SELECT 1 FROM interactions
		    WHERE from_user_id = $1 AND to_user_id = i.from_user_id
		  )
	`
	if err := r.db.QueryRowContext(ctx, countQuery, toUserID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `
		SELECT i.from_user_id FROM interactions i
		WHERE i.to_user_id = $1 AND i.type = 'like'
		  AND NOT EXISTS (
		    SELECT 1 FROM interactions
		    WHERE from_user_id = $1 AND to_user_id = i.from_user_id
		  )
		ORDER BY i.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, toUserID, int(pageSize), int(offset))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	return ids, total, nil
}

func (r *interactionRepository) Delete(ctx context.Context, fromUserID, toUserID int64, t entity.InteractionType) error {
	query := `DELETE FROM interactions WHERE from_user_id = $1 AND to_user_id = $2 AND type = $3`
	_, err := r.db.ExecContext(ctx, query, fromUserID, toUserID, string(t))
	return err
}
