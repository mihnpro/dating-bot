package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	Bucket     string
	UseSSL     bool
	PublicHost string
}

type MinioStorage struct {
	client     *minio.Client
	bucket     string
	publicHost string
}

func NewMinioStorage(ctx context.Context, cfg Config) (*MinioStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	s := &MinioStorage{
		client:     client,
		bucket:     cfg.Bucket,
		publicHost: cfg.PublicHost,
	}

	if err := s.ensureBucket(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *MinioStorage) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket existence: %w", err)
	}
	if exists {
		return nil
	}

	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}

	// Public read policy so Gateway can display images directly via URL.
	policy := fmt.Sprintf(
		`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`,
		s.bucket,
	)
	if err := s.client.SetBucketPolicy(ctx, s.bucket, policy); err != nil {
		return fmt.Errorf("set bucket policy: %w", err)
	}

	return nil
}

func (s *MinioStorage) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}
	return nil
}

func (s *MinioStorage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *MinioStorage) GetPublicURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", s.publicHost, s.bucket, key)
}
