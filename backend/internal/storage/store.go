package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	ErrNotFound    = errors.New("storage object not found")
	ErrUnavailable = errors.New("storage is unavailable")
)

type Object struct {
	Body        io.ReadCloser
	Size        int64
	ContentType string
}

type ObjectStore interface {
	PutObject(ctx context.Context, bucket string, key string, body io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, bucket string, key string) (Object, error)
	RemoveObject(ctx context.Context, bucket string, key string) error
}

type MinIOStore struct {
	client *minio.Client
}

func NewMinIOStore(storageConfig config.StorageConfig) (*MinIOStore, error) {
	storageConfig = config.NormalizeStorageConfig(storageConfig)
	endpoint, secure, err := parseEndpoint(storageConfig.Endpoint)
	if err != nil {
		return nil, err
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(storageConfig.AccessKey, storageConfig.SecretKey, ""),
		Secure: secure,
		Region: storageConfig.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}

	return &MinIOStore{client: client}, nil
}

func (s *MinIOStore) PutObject(ctx context.Context, bucket string, key string, body io.Reader, size int64, contentType string) error {
	if s == nil || s.client == nil {
		return ErrUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	_, err := s.client.PutObject(ctx, bucket, key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return mapError(err)
	}
	return nil
}

func (s *MinIOStore) GetObject(ctx context.Context, bucket string, key string) (Object, error) {
	if s == nil || s.client == nil {
		return Object{}, ErrUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	object, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return Object{}, mapError(err)
	}

	stat, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return Object{}, mapError(err)
	}

	return Object{
		Body:        object,
		Size:        stat.Size,
		ContentType: stat.ContentType,
	}, nil
}

func (s *MinIOStore) RemoveObject(ctx context.Context, bucket string, key string) error {
	if s == nil || s.client == nil {
		return ErrUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return mapError(err)
	}
	return nil
}

func parseEndpoint(raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, fmt.Errorf("MINIO_ENDPOINT is required")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("invalid MINIO_ENDPOINT: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false, fmt.Errorf("invalid MINIO_ENDPOINT: scheme must be http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", false, fmt.Errorf("invalid MINIO_ENDPOINT")
	}

	return parsed.Host, parsed.Scheme == "https", nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	response := minio.ToErrorResponse(err)
	switch response.Code {
	case "NoSuchBucket", "NoSuchKey", "NotFound":
		return ErrNotFound
	default:
		return err
	}
}
