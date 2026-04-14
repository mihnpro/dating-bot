package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dating-bot/user-profile-service/internal/domain/entity"
)

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *userRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	query := `
		INSERT INTO users (telegram_id, username, first_name, last_name, registered_at, last_active, referral_by, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	return r.db.QueryRowContext(ctx, query,
		user.TelegramID,
		user.Username,
		user.FirstName,
		user.LastName,
		user.RegisteredAt,
		user.LastActive,
		user.ReferralBy,
		string(user.Status),
	).Scan(&user.ID)
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*entity.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, registered_at, last_active, referral_by, status
		FROM users WHERE id = $1
	`
	user := &entity.User{}
	var referralBy sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.RegisteredAt,
		&user.LastActive,
		&referralBy,
		&user.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	if referralBy.Valid {
		user.ReferralBy = &referralBy.Int64
	}
	return user, nil
}

func (r *userRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*entity.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, registered_at, last_active, referral_by, status
		FROM users WHERE telegram_id = $1
	`
	user := &entity.User{}
	var referralBy sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, telegramID).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.RegisteredAt,
		&user.LastActive,
		&referralBy,
		&user.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by telegram id: %w", err)
	}
	if referralBy.Valid {
		user.ReferralBy = &referralBy.Int64
	}
	return user, nil
}

func (r *userRepository) Update(ctx context.Context, user *entity.User) error {
	query := `
		UPDATE users SET username = $1, first_name = $2, last_name = $3, last_active = $4, referral_by = $5, status = $6
		WHERE id = $7
	`
	_, err := r.db.ExecContext(ctx, query,
		user.Username,
		user.FirstName,
		user.LastName,
		user.LastActive,
		user.ReferralBy,
		string(user.Status),
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id int64) error {
	query := `UPDATE users SET status = 'deactivated' WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *userRepository) List(ctx context.Context, page, pageSize int32, status *entity.UserStatus) ([]*entity.User, int32, error) {
	var total int32
	countQuery := `SELECT COUNT(*) FROM users`
	args := []any{}
	idx := 1

	if status != nil {
		countQuery += fmt.Sprintf(" WHERE status = $%d", idx)
		args = append(args, string(*status))
		idx++
	}
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `
		SELECT id, telegram_id, username, first_name, last_name, registered_at, last_active, referral_by, status
		FROM users
	`
	if status != nil {
		query += fmt.Sprintf(" WHERE status = $%d", idx)
		idx++
	}
	query += fmt.Sprintf(" ORDER BY id LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, int(pageSize), int(offset))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*entity.User
	for rows.Next() {
		u := &entity.User{}
		var referralBy sql.NullInt64
		if err := rows.Scan(
			&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LastName,
			&u.RegisteredAt, &u.LastActive, &referralBy, &u.Status,
		); err != nil {
			return nil, 0, err
		}
		if referralBy.Valid {
			u.ReferralBy = &referralBy.Int64
		}
		users = append(users, u)
	}

	return users, total, nil
}
