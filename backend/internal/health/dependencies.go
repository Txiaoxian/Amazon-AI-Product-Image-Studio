package health

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

var ErrDependencyCheckerUnavailable = errors.New("dependency checker is unavailable")

type redisPinger interface {
	Ping(context.Context) *redis.StatusCmd
}

type RedisChecker struct {
	client redisPinger
}

func NewRedisChecker(queueConfig config.QueueConfig) RedisChecker {
	return RedisChecker{
		client: redis.NewClient(&redis.Options{
			Addr:     queueConfig.RedisAddr,
			Password: queueConfig.RedisPassword,
			DB:       queueConfig.RedisDB,
		}),
	}
}

func NewRedisCheckerWithClient(client redisPinger) RedisChecker {
	return RedisChecker{client: client}
}

func (c RedisChecker) Name() string {
	return "redis"
}

func (c RedisChecker) Check(ctx context.Context) error {
	if c.client == nil {
		return ErrDependencyCheckerUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.client.Ping(ctx).Err()
}

type minioBucketChecker interface {
	BucketExists(context.Context, string) (bool, error)
}

type MinIOChecker struct {
	client  minioBucketChecker
	buckets []string
}

func NewMinIOChecker(storageConfig config.StorageConfig) (MinIOChecker, error) {
	storageConfig = config.NormalizeStorageConfig(storageConfig)
	endpoint, secure, err := minioEndpoint(storageConfig.Endpoint)
	if err != nil {
		return MinIOChecker{}, err
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(storageConfig.AccessKey, storageConfig.SecretKey, ""),
		Secure: secure,
		Region: storageConfig.Region,
	})
	if err != nil {
		return MinIOChecker{}, fmt.Errorf("create MinIO health client: %w", err)
	}

	return NewMinIOCheckerWithClient(client, storageConfig.BucketOriginals, storageConfig.BucketGenerated, storageConfig.BucketThumbnails), nil
}

func NewMinIOCheckerWithClient(client minioBucketChecker, buckets ...string) MinIOChecker {
	return MinIOChecker{client: client, buckets: uniqueBucketNames(buckets)}
}

func (c MinIOChecker) Name() string {
	return "minio"
}

func (c MinIOChecker) Check(ctx context.Context) error {
	if c.client == nil {
		return ErrDependencyCheckerUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for _, bucket := range c.buckets {
		exists, err := c.client.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("check MinIO bucket %q: %w", bucket, err)
		}
		if !exists {
			return fmt.Errorf("MinIO bucket %q is missing", bucket)
		}
	}

	return nil
}

func minioEndpoint(raw string) (string, bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
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

func uniqueBucketNames(buckets []string) []string {
	seen := make(map[string]struct{}, len(buckets))
	unique := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		bucket = strings.TrimSpace(bucket)
		if bucket == "" {
			continue
		}
		if _, ok := seen[bucket]; ok {
			continue
		}
		seen[bucket] = struct{}{}
		unique = append(unique, bucket)
	}
	return unique
}
