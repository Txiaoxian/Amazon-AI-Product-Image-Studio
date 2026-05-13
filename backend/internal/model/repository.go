package model

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

func (r Repository) ListModels(ctx context.Context, scope tenant.Scope, options ListOptions) ([]database.AIModel, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}

	query := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND deleted_at IS NULL", scope.ID())
	if options.ProviderID != "" {
		query = query.Where("provider_id = ?", options.ProviderID)
	}
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	switch options.Capability {
	case capabilityGenerateFilter:
		query = query.Where("supports_generate = ?", true)
	case capabilityEditFilter:
		query = query.Where("supports_edit = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []database.AIModel
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

func (r Repository) FindModel(ctx context.Context, scope tenant.Scope, modelID string) (database.AIModel, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.AIModel{}, err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return database.AIModel{}, ErrValidation
	}

	var record database.AIModel
	err = db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), modelID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.AIModel{}, ErrNotFound
	}
	return record, err
}

func (r Repository) CreateModel(ctx context.Context, scope tenant.Scope, record *database.AIModel) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	record.TenantID = scope.ID()
	return db.Create(record).Error
}

func (r Repository) UpdateModel(ctx context.Context, scope tenant.Scope, modelID string, updates map[string]any) (database.AIModel, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.AIModel{}, err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return database.AIModel{}, ErrValidation
	}

	result := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), modelID).
		Updates(updates)
	if result.Error != nil {
		return database.AIModel{}, result.Error
	}
	if result.RowsAffected == 0 {
		return database.AIModel{}, ErrNotFound
	}

	return r.FindModel(ctx, scope, modelID)
}

func (r Repository) SoftDeleteModel(ctx context.Context, scope tenant.Scope, modelID string, deletedAt time.Time) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ErrValidation
	}

	result := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), modelID).
		Update("deleted_at", deletedAt.UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r Repository) ProviderNames(ctx context.Context, scope tenant.Scope, providerIDs []string) (map[string]string, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, err
	}

	uniqueIDs := make([]string, 0, len(providerIDs))
	seen := map[string]bool{}
	for _, providerID := range providerIDs {
		providerID = strings.TrimSpace(providerID)
		if providerID == "" || seen[providerID] {
			continue
		}
		seen[providerID] = true
		uniqueIDs = append(uniqueIDs, providerID)
	}
	if len(uniqueIDs) == 0 {
		return map[string]string{}, nil
	}

	var providers []database.AIProvider
	if err := db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", scope.ID(), uniqueIDs).
		Find(&providers).Error; err != nil {
		return nil, err
	}

	names := make(map[string]string, len(providers))
	for _, provider := range providers {
		names[provider.ID] = provider.Name
	}
	return names, nil
}
