package repository

import (
	"context"
	"io"

	"github.com/dating-bot/media-service/internal/domain/entity"
)

type MediaRepository interface {
	Save(ctx context.Context, media *entity.Media) error
	GetByID(ctx context.Context, id int64) (*entity.Media, error)
	GetByUserID(ctx context.Context, userID int64) ([]*entity.Media, error)
	Delete(ctx context.Context, id int64) (*entity.Media, error)
	SetMain(ctx context.Context, mediaID, userID int64) error
}

type StoragePort interface {
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
	GetPublicURL(key string) string
}

type EventPublisher interface {
	PublishMediaUploaded(ctx context.Context, userID, mediaID int64) error
	Close() error
}
