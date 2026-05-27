package asset

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type PurgeCandidate struct {
	ID                 string
	TenantID           string
	Kind               string
	ObjectKey          string
	ThumbnailObjectKey *string
	DeletedAt          time.Time
}

func NewRepository(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (r Repository) withDB(db *gorm.DB) Repository {
	return Repository{db: db}
}

func (r Repository) base(ctx context.Context, scope tenant.Scope) (*gorm.DB, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if !scope.Valid() {
		return nil, tenant.ErrMissingTenantID
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return r.db.WithContext(ctx), nil
}

func (r Repository) ListAssets(ctx context.Context, scope tenant.Scope, projectID string, options ListOptions) ([]database.ImageAsset, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, 0, ErrValidation
	}

	query := db.Model(&database.ImageAsset{}).
		Where("tenant_id = ? AND project_id = ? AND deleted_at IS NULL", scope.ID(), projectID)
	if options.Kind != "" {
		query = query.Where("kind = ?", options.Kind)
	}
	if options.Category != "" {
		query = query.Where("category = ?", options.Category)
	}
	if options.Favorite != nil {
		query = query.Where("is_favorite = ?", *options.Favorite)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []database.ImageAsset
	offset := (options.PageNum - 1) * options.PageSize
	if err := query.
		Order("created_at DESC, id DESC").
		Limit(options.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

func (r Repository) FindAsset(ctx context.Context, scope tenant.Scope, assetID string) (database.ImageAsset, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.ImageAsset{}, err
	}
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return database.ImageAsset{}, ErrValidation
	}

	var record database.ImageAsset
	err = db.Model(&database.ImageAsset{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), assetID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.ImageAsset{}, ErrNotFound
	}
	return record, err
}

func (r Repository) CreateAsset(ctx context.Context, scope tenant.Scope, record *database.ImageAsset) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	return db.Create(record).Error
}

func (r Repository) UpdateAsset(ctx context.Context, scope tenant.Scope, assetID string, updates map[string]any) (database.ImageAsset, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.ImageAsset{}, err
	}
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return database.ImageAsset{}, ErrValidation
	}

	result := db.Model(&database.ImageAsset{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), assetID).
		Updates(updates)
	if result.Error != nil {
		return database.ImageAsset{}, result.Error
	}
	if result.RowsAffected == 0 {
		return database.ImageAsset{}, ErrNotFound
	}

	return r.FindAsset(ctx, scope, assetID)
}

func (r Repository) SoftDeleteAsset(ctx context.Context, scope tenant.Scope, assetID string, deletedAt time.Time) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return ErrValidation
	}

	result := db.Model(&database.ImageAsset{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), assetID).
		Update("deleted_at", deletedAt.UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r Repository) ListPurgeCandidates(ctx context.Context, scope tenant.Scope, cutoff time.Time, limit int) ([]PurgeCandidate, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}
	if cutoff.IsZero() || limit <= 0 {
		return nil, ErrValidation
	}

	var records []PurgeCandidate
	err = db.Table("image_assets").
		Select("id, tenant_id, kind, object_key, thumbnail_object_key, deleted_at").
		Where("tenant_id = ? AND deleted_at IS NOT NULL AND deleted_at < ? AND purged_at IS NULL", scope.ID(), cutoff.UTC()).
		Order("deleted_at ASC, id ASC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

func (r Repository) ObjectKeyReferenced(ctx context.Context, scope tenant.Scope, objectKey string) (bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return false, err
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return false, ErrValidation
	}

	var count int64
	if err := db.Model(&database.ImageAsset{}).
		Where("tenant_id = ? AND (object_key = ? OR thumbnail_object_key = ?)", scope.ID(), objectKey, objectKey).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r Repository) MarkAssetPurged(ctx context.Context, scope tenant.Scope, assetID string, purgedAt time.Time) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	assetID = strings.TrimSpace(assetID)
	if assetID == "" || purgedAt.IsZero() {
		return ErrValidation
	}

	result := db.Table("image_assets").
		Where("tenant_id = ? AND id = ? AND deleted_at IS NOT NULL AND purged_at IS NULL", scope.ID(), assetID).
		Updates(map[string]any{
			"purged_at":  purgedAt.UTC(),
			"updated_at": purgedAt.UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	return nil
}
