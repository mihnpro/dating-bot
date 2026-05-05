package repository

import (
	"context"

	"github.com/dating-bot/user-profile-service/internal/domain/entity"
)

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, id int64) (*entity.User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, page, pageSize int32, status *entity.UserStatus) ([]*entity.User, int32, error)
}

type ProfileRepository interface {
	Create(ctx context.Context, profile *entity.Profile) error
	GetByUserID(ctx context.Context, userID int64) (*entity.Profile, error)
	Update(ctx context.Context, profile *entity.Profile) error
	Delete(ctx context.Context, userID int64) error
	List(ctx context.Context, page, pageSize int32, gender *entity.Gender, city *string, minAge, maxAge *int32) ([]*entity.Profile, int32, error)
	// IncrementPhotosCount atomically increments photos_count and returns the updated profile.
	IncrementPhotosCount(ctx context.Context, userID int64) (*entity.Profile, error)
}
