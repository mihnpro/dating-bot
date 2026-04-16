package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dating-bot/user-profile-service/internal/domain/entity"
	"github.com/dating-bot/user-profile-service/internal/domain/event"
	"github.com/dating-bot/user-profile-service/internal/domain/repository"
)

type UserService struct {
	userRepo    repository.UserRepository
	profileRepo repository.ProfileRepository
	publisher   repository.EventPublisher
}

func NewUserService(
	userRepo repository.UserRepository,
	profileRepo repository.ProfileRepository,
	publisher repository.EventPublisher,
) *UserService {
	return &UserService{
		userRepo:    userRepo,
		profileRepo: profileRepo,
		publisher:   publisher,
	}
}

func (s *UserService) RegisterUser(ctx context.Context, telegramID int64, username, firstName, lastName string, referralBy *int64) (*entity.User, error) {
	// Idempotent: return the existing user if the telegram_id is already registered.
	existing, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	user := entity.NewUser(telegramID, username, firstName, lastName, referralBy)
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	evt, _ := event.NewUserRegisteredEvent(user)
	if evt != nil {
		_ = s.publisher.Publish(ctx, "user.registered", evt)
	}

	return user, nil
}

func (s *UserService) GetUser(ctx context.Context, id int64) (*entity.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

func (s *UserService) GetUserByTelegramID(ctx context.Context, telegramID int64) (*entity.User, error) {
	return s.userRepo.GetByTelegramID(ctx, telegramID)
}

func (s *UserService) UpdateUser(ctx context.Context, id int64, username, firstName, lastName *string, status *entity.UserStatus) (*entity.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if username != nil {
		user.Username = *username
	}
	if firstName != nil {
		user.FirstName = *firstName
	}
	if lastName != nil {
		user.LastName = *lastName
	}
	if status != nil {
		user.Status = *status
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	evt, _ := event.NewUserUpdatedEvent(user)
	if evt != nil {
		_ = s.publisher.Publish(ctx, "user.updated", evt)
	}

	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id int64) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	user.Deactivate()
	return s.userRepo.Update(ctx, user)
}

func (s *UserService) ListUsers(ctx context.Context, page, pageSize int32, status *entity.UserStatus) ([]*entity.User, int32, error) {
	return s.userRepo.List(ctx, page, pageSize, status)
}

func (s *UserService) CreateProfile(ctx context.Context, userID int64, age int, gender entity.Gender, city string, interests []string) (*entity.Profile, error) {
	profile := entity.NewProfile(userID, age, gender, city, interests)
	profile.CalculateFullness()
	if err := s.profileRepo.Create(ctx, profile); err != nil {
		return nil, err
	}

	evt, _ := event.NewProfileUpdatedEvent(profile)
	if evt != nil {
		_ = s.publisher.Publish(ctx, "profile.updated", evt)
	}

	return profile, nil
}

func (s *UserService) GetProfile(ctx context.Context, userID int64) (*entity.Profile, error) {
	return s.profileRepo.GetByUserID(ctx, userID)
}

func (s *UserService) UpdateProfile(ctx context.Context, userID int64, age *int32, gender *entity.Gender, city *string, interests []string, photosCount *int32) (*entity.Profile, error) {
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if age != nil {
		profile.Age = int(*age)
	}
	if gender != nil {
		profile.Gender = *gender
	}
	if city != nil {
		profile.City = *city
	}
	if interests != nil {
		profile.Interests = interests
	}
	if photosCount != nil {
		profile.PhotosCount = int(*photosCount)
	}

	profile.CalculateFullness()
	if err := s.profileRepo.Update(ctx, profile); err != nil {
		return nil, err
	}

	evt, _ := event.NewProfileUpdatedEvent(profile)
	if evt != nil {
		_ = s.publisher.Publish(ctx, "profile.updated", evt)
	}

	return profile, nil
}

func (s *UserService) DeleteProfile(ctx context.Context, userID int64) error {
	return s.profileRepo.Delete(ctx, userID)
}

func (s *UserService) ListProfiles(ctx context.Context, page, pageSize int32, gender *entity.Gender, city *string, minAge, maxAge *int32) ([]*entity.Profile, int32, error) {
	return s.profileRepo.List(ctx, page, pageSize, gender, city, minAge, maxAge)
}
