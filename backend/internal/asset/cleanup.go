package asset

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

const (
	defaultPurgeBatchLimit = 100
	maxPurgeBatchLimit     = 1000
)

type CleanupService struct {
	db            *gorm.DB
	repo          Repository
	store         storage.ObjectStore
	storageConfig config.StorageConfig
	log           *slog.Logger
	now           func() time.Time
}

type PurgeOptions struct {
	BatchLimit int
}

type PurgeResult struct {
	Processed int
	Purged    int
	Failed    int
}

func NewCleanupService(db *gorm.DB, log *slog.Logger, storageConfig config.StorageConfig, store storage.ObjectStore) *CleanupService {
	if log == nil {
		log = slog.Default()
	}
	return &CleanupService{
		db:            db,
		repo:          NewRepository(db),
		store:         store,
		storageConfig: config.NormalizeStorageConfig(storageConfig),
		log:           log,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *CleanupService) PurgeDeletedAssets(ctx context.Context, tenantID string, cutoff time.Time, options PurgeOptions) (PurgeResult, error) {
	if s.db == nil {
		return PurgeResult{}, database.ErrNilDB
	}
	if s.store == nil {
		return PurgeResult{}, ErrStorageUnavailable
	}
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		return PurgeResult{}, err
	}
	if cutoff.IsZero() {
		return PurgeResult{}, ErrValidation
	}
	limit := normalizePurgeBatchLimit(options.BatchLimit)
	records, err := s.repo.ListPurgeCandidates(ctx, scope, cutoff, limit)
	if err != nil {
		return PurgeResult{}, err
	}

	result := PurgeResult{Processed: len(records)}
	var failed bool
	for _, record := range records {
		if err := s.purgeCandidate(ctx, scope, record); err != nil {
			failed = true
			result.Failed++
			s.log.Warn("asset purge failed", slog.String("tenant_id", scope.ID()), slog.String("asset_id", record.ID), slog.String("error_kind", safeCleanupErrorKind(err)))
			continue
		}
		result.Purged++
	}
	if failed {
		return result, ErrCleanupFailed
	}
	return result, nil
}

func (s *CleanupService) purgeCandidate(ctx context.Context, scope tenant.Scope, record PurgeCandidate) error {
	if err := removeObjectIfPresent(ctx, s.store, bucketForKind(s.storageConfig, record.Kind), record.ObjectKey); err != nil {
		return err
	}
	if record.ThumbnailObjectKey != nil && *record.ThumbnailObjectKey != "" {
		if err := removeObjectIfPresent(ctx, s.store, s.storageConfig.BucketThumbnails, *record.ThumbnailObjectKey); err != nil {
			return err
		}
	}
	return s.repo.MarkAssetPurged(ctx, scope, record.ID, s.now())
}

func removeObjectIfPresent(ctx context.Context, store storage.ObjectStore, bucket string, key string) error {
	err := store.RemoveObject(ctx, bucket, key)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	return err
}

func normalizePurgeBatchLimit(limit int) int {
	if limit <= 0 {
		return defaultPurgeBatchLimit
	}
	if limit > maxPurgeBatchLimit {
		return maxPurgeBatchLimit
	}
	return limit
}

func safeCleanupErrorKind(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline_exceeded"
	case errors.Is(err, storage.ErrNotFound):
		return "storage_not_found"
	case errors.Is(err, storage.ErrUnavailable):
		return "storage_unavailable"
	default:
		return "internal_error"
	}
}
