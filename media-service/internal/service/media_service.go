package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/dating-bot/media-service/internal/domain/entity"
	"github.com/dating-bot/media-service/internal/domain/repository"
)

var (
	ErrMediaNotFound   = errors.New("media not found")
	ErrTooManyPhotos   = errors.New("photo limit reached: maximum 6 photos per user")
	ErrInvalidMimeType = errors.New("invalid file type: only JPEG, PNG and WebP are allowed")
	ErrForbidden       = errors.New("access denied")
)

type MediaService struct {
	repo      repository.MediaRepository
	storage   repository.StoragePort
	publisher repository.EventPublisher
	log       *logrus.Logger
}

func NewMediaService(
	repo repository.MediaRepository,
	storage repository.StoragePort,
	publisher repository.EventPublisher,
	log *logrus.Logger,
) *MediaService {
	return &MediaService{
		repo:      repo,
		storage:   storage,
		publisher: publisher,
		log:       log,
	}
}

// UploadInput carries the data needed to upload a photo.
type UploadInput struct {
	UserID           int64
	OriginalFilename string
	MimeType         string
	FileSize         int64
	Content          io.Reader
}

// MediaWithURL enriches Media with a publicly accessible URL.
type MediaWithURL struct {
	*entity.Media
	URL string
}

func (s *MediaService) Upload(ctx context.Context, input UploadInput) (*MediaWithURL, error) {
	if !entity.AllowedMimeTypes[input.MimeType] {
		return nil, ErrInvalidMimeType
	}

	existing, err := s.repo.GetByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user media: %w", err)
	}
	if len(existing) >= entity.MaxPhotosPerUser {
		return nil, ErrTooManyPhotos
	}

	s3Key := fmt.Sprintf("%d/%s%s", input.UserID, uuid.New().String(), mimeToExt(input.MimeType))

	if err := s.storage.Upload(ctx, s3Key, input.Content, input.FileSize, input.MimeType); err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	isMain := len(existing) == 0
	media := entity.NewMedia(input.UserID, s3Key, input.OriginalFilename, input.MimeType, input.FileSize, isMain)

	if err := s.repo.Save(ctx, media); err != nil {
		_ = s.storage.Delete(ctx, s3Key)
		return nil, fmt.Errorf("save media metadata: %w", err)
	}

	if err := s.publisher.PublishMediaUploaded(ctx, input.UserID, media.ID); err != nil {
		s.log.WithError(err).Warn("failed to publish media.uploaded event")
	}

	return &MediaWithURL{
		Media: media,
		URL:   s.storage.GetPublicURL(s3Key),
	}, nil
}

func (s *MediaService) GetByID(ctx context.Context, id int64) (*MediaWithURL, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMediaNotFound
		}
		return nil, fmt.Errorf("get media: %w", err)
	}
	return &MediaWithURL{Media: m, URL: s.storage.GetPublicURL(m.S3Key)}, nil
}

func (s *MediaService) GetByUserID(ctx context.Context, userID int64) ([]*MediaWithURL, error) {
	medias, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user media: %w", err)
	}

	result := make([]*MediaWithURL, len(medias))
	for i, m := range medias {
		result[i] = &MediaWithURL{Media: m, URL: s.storage.GetPublicURL(m.S3Key)}
	}
	return result, nil
}

func (s *MediaService) Delete(ctx context.Context, mediaID, callerUserID int64) error {
	m, err := s.repo.GetByID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMediaNotFound
		}
		return fmt.Errorf("get media: %w", err)
	}

	if m.UserID != callerUserID {
		return ErrForbidden
	}

	deleted, err := s.repo.Delete(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("delete media: %w", err)
	}

	if err := s.storage.Delete(ctx, deleted.S3Key); err != nil {
		s.log.WithError(err).WithField("s3_key", deleted.S3Key).Warn("failed to delete object from storage")
	}

	return nil
}

func (s *MediaService) SetMain(ctx context.Context, mediaID, callerUserID int64) error {
	m, err := s.repo.GetByID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMediaNotFound
		}
		return fmt.Errorf("get media: %w", err)
	}

	if m.UserID != callerUserID {
		return ErrForbidden
	}

	return s.repo.SetMain(ctx, mediaID, callerUserID)
}

func mimeToExt(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}
