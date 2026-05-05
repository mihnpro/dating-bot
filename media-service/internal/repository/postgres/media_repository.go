package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"github.com/dating-bot/media-service/internal/domain/entity"
)

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

func (r *MediaRepository) Save(ctx context.Context, media *entity.Media) error {
	q := `
		INSERT INTO media (user_id, s3_key, original_filename, mime_type, file_size, is_main, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	return r.db.QueryRowContext(ctx, q,
		media.UserID,
		media.S3Key,
		media.OriginalFilename,
		media.MimeType,
		media.FileSize,
		media.IsMain,
		media.UploadedAt,
	).Scan(&media.ID)
}

func (r *MediaRepository) GetByID(ctx context.Context, id int64) (*entity.Media, error) {
	q := `
		SELECT id, user_id, s3_key, original_filename, mime_type, file_size, is_main, uploaded_at
		FROM media WHERE id = $1
	`
	m := &entity.Media{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.UserID, &m.S3Key, &m.OriginalFilename,
		&m.MimeType, &m.FileSize, &m.IsMain, &m.UploadedAt,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *MediaRepository) GetByUserID(ctx context.Context, userID int64) ([]*entity.Media, error) {
	q := `
		SELECT id, user_id, s3_key, original_filename, mime_type, file_size, is_main, uploaded_at
		FROM media WHERE user_id = $1 ORDER BY is_main DESC, uploaded_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*entity.Media
	for rows.Next() {
		m := &entity.Media{}
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.S3Key, &m.OriginalFilename,
			&m.MimeType, &m.FileSize, &m.IsMain, &m.UploadedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *MediaRepository) Delete(ctx context.Context, id int64) (*entity.Media, error) {
	q := `
		DELETE FROM media WHERE id = $1
		RETURNING id, user_id, s3_key, original_filename, mime_type, file_size, is_main, uploaded_at
	`
	m := &entity.Media{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.UserID, &m.S3Key, &m.OriginalFilename,
		&m.MimeType, &m.FileSize, &m.IsMain, &m.UploadedAt,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *MediaRepository) SetMain(ctx context.Context, mediaID, userID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE media SET is_main = false WHERE user_id = $1`, userID,
	); err != nil {
		return fmt.Errorf("unset main photos: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE media SET is_main = true WHERE id = $1 AND user_id = $2`, mediaID, userID,
	); err != nil {
		return fmt.Errorf("set main photo: %w", err)
	}

	return tx.Commit()
}
