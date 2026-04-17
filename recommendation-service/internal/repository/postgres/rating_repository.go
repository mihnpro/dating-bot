package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/dating-bot/recommendation-service/internal/domain/entity"
)

type ratingRepository struct {
	db *sql.DB
}

func NewRatingRepository(db *sql.DB) *ratingRepository {
	return &ratingRepository{db: db}
}

// Upsert inserts or updates the full rating record for a user.
func (r *ratingRepository) Upsert(ctx context.Context, rating *entity.Rating) error {
	query := `
		INSERT INTO ratings (user_id, gender, age, city, primary_rating, behavioral_rating, combined_rating, calculated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			gender            = EXCLUDED.gender,
			age               = EXCLUDED.age,
			city              = EXCLUDED.city,
			primary_rating    = EXCLUDED.primary_rating,
			behavioral_rating = EXCLUDED.behavioral_rating,
			combined_rating   = EXCLUDED.combined_rating,
			calculated_at     = EXCLUDED.calculated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		rating.UserID,
		rating.Gender,
		rating.Age,
		rating.City,
		rating.PrimaryRating,
		rating.BehavioralRating,
		rating.CombinedRating,
		rating.CalculatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert rating user_id=%d: %w", rating.UserID, err)
	}
	return nil
}

// GetByUserID returns the stored rating for a single user.
func (r *ratingRepository) GetByUserID(ctx context.Context, userID int64) (*entity.Rating, error) {
	query := `
		SELECT user_id, gender, age, city,
		       primary_rating, behavioral_rating, combined_rating, calculated_at
		FROM ratings
		WHERE user_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, userID)
	return scanRating(row)
}

// GetCandidates returns up to limit ratings ordered by combined_rating DESC,
// filtering by the target gender and excluding already-seen user IDs.
func (r *ratingRepository) GetCandidates(
	ctx context.Context,
	gender string,
	excludeUserIDs []int64,
	limit int,
) ([]*entity.Rating, error) {
	args := []any{gender}
	idx := 2

	// Build the NOT IN clause only when there are IDs to exclude.
	excludeClause := ""
	if len(excludeUserIDs) > 0 {
		placeholders := make([]string, len(excludeUserIDs))
		for i, id := range excludeUserIDs {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, id)
			idx++
		}
		excludeClause = "AND user_id NOT IN (" + strings.Join(placeholders, ", ") + ")"
	}

	query := fmt.Sprintf(`
		SELECT user_id, gender, age, city,
		       primary_rating, behavioral_rating, combined_rating, calculated_at
		FROM ratings
		WHERE gender = $1
		%s
		ORDER BY combined_rating DESC
		LIMIT $%d
	`, excludeClause, idx)

	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get candidates: %w", err)
	}
	defer rows.Close()

	var result []*entity.Rating
	for rows.Next() {
		rt, err := scanRatingRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, rt)
	}
	return result, rows.Err()
}

// GetAll returns every rating row — used by the periodic recalculation worker.
func (r *ratingRepository) GetAll(ctx context.Context) ([]*entity.Rating, error) {
	query := `
		SELECT user_id, gender, age, city,
		       primary_rating, behavioral_rating, combined_rating, calculated_at
		FROM ratings
		ORDER BY user_id
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all ratings: %w", err)
	}
	defer rows.Close()

	var result []*entity.Rating
	for rows.Next() {
		rt, err := scanRatingRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, rt)
	}
	return result, rows.Err()
}

// LogChange appends a rating_log row.
func (r *ratingRepository) LogChange(
	ctx context.Context,
	userID int64,
	oldCombined, newCombined float64,
	reason string,
) error {
	query := `
		INSERT INTO rating_log (user_id, old_combined, new_combined, reason, changed_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	_, err := r.db.ExecContext(ctx, query, userID, oldCombined, newCombined, reason)
	if err != nil {
		return fmt.Errorf("log rating change user_id=%d: %w", userID, err)
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scanRating(row *sql.Row) (*entity.Rating, error) {
	rt := &entity.Rating{}
	err := row.Scan(
		&rt.UserID, &rt.Gender, &rt.Age, &rt.City,
		&rt.PrimaryRating, &rt.BehavioralRating, &rt.CombinedRating, &rt.CalculatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan rating: %w", err)
	}
	return rt, nil
}

func scanRatingRow(rows *sql.Rows) (*entity.Rating, error) {
	rt := &entity.Rating{}
	err := rows.Scan(
		&rt.UserID, &rt.Gender, &rt.Age, &rt.City,
		&rt.PrimaryRating, &rt.BehavioralRating, &rt.CombinedRating, &rt.CalculatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan rating row: %w", err)
	}
	return rt, nil
}
