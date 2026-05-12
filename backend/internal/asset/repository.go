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
