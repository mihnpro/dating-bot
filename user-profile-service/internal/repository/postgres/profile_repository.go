package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/dating-bot/user-profile-service/internal/domain/entity"
	"github.com/lib/pq"
)

type profileRepository struct {
	db *sql.DB
}

func NewProfileRepository(db *sql.DB) *profileRepository {
	return &profileRepository{db: db}
}

func (r *profileRepository) Create(ctx context.Context, profile *entity.Profile) error {
	query := `
		INSERT INTO profiles (user_id, age, gender, city, interests, photos_count, fullness_percent, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	return r.db.QueryRowContext(ctx, query,
		profile.UserID,
		profile.Age,
		string(profile.Gender),
		profile.City,
		pq.Array(profile.Interests),
		profile.PhotosCount,
		profile.FullnessPercent,
		profile.UpdatedAt,
	).Scan(&profile.ID)
}

func (r *profileRepository) GetByUserID(ctx context.Context, userID int64) (*entity.Profile, error) {
	query := `
		SELECT id, user_id, age, gender, city, interests, photos_count, fullness_percent, updated_at
		FROM profiles WHERE user_id = $1
	`
	p := &entity.Profile{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&p.ID, &p.UserID, &p.Age, &p.Gender, &p.City,
		pq.Array(&p.Interests), &p.PhotosCount, &p.FullnessPercent, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get profile by user id: %w", err)
	}
	p.Gender = entity.Gender(p.Gender)
	return p, nil
}

func (r *profileRepository) Update(ctx context.Context, profile *entity.Profile) error {
	query := `
		UPDATE profiles SET age = $1, gender = $2, city = $3, interests = $4,
		photos_count = $5, fullness_percent = $6, updated_at = $7
		WHERE user_id = $8
	`
	_, err := r.db.ExecContext(ctx, query,
		profile.Age,
		string(profile.Gender),
		profile.City,
		pq.Array(profile.Interests),
		profile.PhotosCount,
		profile.FullnessPercent,
		profile.UpdatedAt,
		profile.UserID,
	)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

func (r *profileRepository) Delete(ctx context.Context, userID int64) error {
	query := `DELETE FROM profiles WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *profileRepository) List(ctx context.Context, page, pageSize int32, gender *entity.Gender, city *string, minAge, maxAge *int32) ([]*entity.Profile, int32, error) {
	conditions := []string{}
	args := []any{}
	idx := 1

	if gender != nil {
		conditions = append(conditions, fmt.Sprintf("gender = $%d", idx))
		args = append(args, string(*gender))
		idx++
	}
	if city != nil {
		conditions = append(conditions, fmt.Sprintf("city = $%d", idx))
		args = append(args, *city)
		idx++
	}
	if minAge != nil {
		conditions = append(conditions, fmt.Sprintf("age >= $%d", idx))
		args = append(args, int(*minAge))
		idx++
	}
	if maxAge != nil {
		conditions = append(conditions, fmt.Sprintf("age <= $%d", idx))
		args = append(args, int(*maxAge))
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int32
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query := `
		SELECT id, user_id, age, gender, city, interests, photos_count, fullness_percent, updated_at
		FROM profiles
	` + where + fmt.Sprintf(" ORDER BY id LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, int(pageSize), int(offset))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var profiles []*entity.Profile
	for rows.Next() {
		p := &entity.Profile{}
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Age, &p.Gender, &p.City,
			pq.Array(&p.Interests), &p.PhotosCount, &p.FullnessPercent, &p.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		p.Gender = entity.Gender(p.Gender)
		profiles = append(profiles, p)
	}

	return profiles, total, nil
}
