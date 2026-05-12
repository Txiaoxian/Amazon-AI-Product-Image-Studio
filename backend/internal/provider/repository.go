package provider

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

func (r Repository) ListProviders(ctx context.Context, scope tenant.Scope, options ListOptions) ([]database.AIProvider, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}

	query := db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND deleted_at IS NULL", scope.ID())
	if options.Type != "" {
		query = query.Where("type = ?", options.Type)
	}
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []database.AIProvider
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

func (r Repository) FindProvider(ctx context.Context, scope tenant.Scope, providerID string) (database.AIProvider, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.AIProvider{}, err
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return database.AIProvider{}, ErrValidation
	}

	var record database.AIProvider
	err = db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), providerID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.AIProvider{}, ErrNotFound
	}
	return record, err
}

func (r Repository) CreateProvider(ctx context.Context, scope tenant.Scope, record *database.AIProvider) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	return db.Create(record).Error
}

func (r Repository) UpdateProvider(ctx context.Context, scope tenant.Scope, providerID string, updates map[string]any) (database.AIProvider, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.AIProvider{}, err
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return database.AIProvider{}, ErrValidation
	}

	result := db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), providerID).
		Updates(updates)
	if result.Error != nil {
		return database.AIProvider{}, result.Error
	}
	if result.RowsAffected == 0 {
		return database.AIProvider{}, ErrNotFound
	}

	return r.FindProvider(ctx, scope, providerID)
}

func (r Repository) SoftDeleteProvider(ctx context.Context, scope tenant.Scope, providerID string, deletedAt time.Time) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return ErrValidation
	}

	result := db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), providerID).
		Update("deleted_at", deletedAt.UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
