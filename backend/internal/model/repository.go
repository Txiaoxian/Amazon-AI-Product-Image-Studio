package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r Repository) ListModels(ctx context.Context, scope tenant.Scope, options ListOptions, accessibleUserID string) ([]database.AIModel, int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return nil, 0, err
	}

	query := db.Model(&database.AIModel{}).
		Where("ai_models.tenant_id = ? AND ai_models.deleted_at IS NULL", scope.ID())
	accessibleUserID = strings.TrimSpace(accessibleUserID)
	if accessibleUserID != "" {
		query = query.Joins(
			"JOIN user_model_access_grants ON user_model_access_grants.tenant_id = ai_models.tenant_id AND user_model_access_grants.model_id = ai_models.id AND user_model_access_grants.user_id = ?",
			accessibleUserID,
		)
	}
	if options.ProviderID != "" {
		query = query.Where("ai_models.provider_id = ?", options.ProviderID)
	}
	if options.Status != "" {
		query = query.Where("ai_models.status = ?", options.Status)
	}
	switch options.Capability {
	case capabilityGenerateFilter:
		query = query.Where("ai_models.supports_generate = ?", true)
	case capabilityEditFilter:
		query = query.Where("ai_models.supports_edit = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []database.AIModel
	offset := (options.PageNum - 1) * options.PageSize
	if err := query.
		Order("ai_models.created_at DESC, ai_models.id DESC").
		Limit(options.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

func (r Repository) UserCanAccessModel(ctx context.Context, scope tenant.Scope, userID string, modelID string) (bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return false, err
	}
	userID = strings.TrimSpace(userID)
	modelID = strings.TrimSpace(modelID)
	if userID == "" || modelID == "" {
		return false, ErrValidation
	}
	var count int64
	if err := db.Model(&database.UserModelAccessGrant{}).
		Where("tenant_id = ? AND user_id = ? AND model_id = ?", scope.ID(), userID, modelID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r Repository) FindModel(ctx context.Context, scope tenant.Scope, modelID string) (database.AIModel, error) {
	return r.findModel(ctx, scope, modelID, false)
}

func (r Repository) LockModel(ctx context.Context, scope tenant.Scope, modelID string) (database.AIModel, error) {
	return r.findModel(ctx, scope, modelID, true)
}

func (r Repository) findModel(ctx context.Context, scope tenant.Scope, modelID string, lock bool) (database.AIModel, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.AIModel{}, err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return database.AIModel{}, ErrValidation
	}

	var record database.AIModel
	query := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), modelID).
		Limit(1)
	if lock && db.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err = query.First(&record).Error
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

func (r Repository) ActiveModelNameExists(ctx context.Context, scope tenant.Scope, providerID string, modelName string, excludeModelID string) (bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return false, err
	}
	providerID = strings.TrimSpace(providerID)
	modelName = strings.TrimSpace(modelName)
	excludeModelID = strings.TrimSpace(excludeModelID)
	if providerID == "" || modelName == "" {
		return false, ErrValidation
	}

	var count int64
	query := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND provider_id = ? AND model_name = ? AND deleted_at IS NULL", scope.ID(), providerID, modelName)
	if excludeModelID != "" {
		query = query.Where("id <> ?", excludeModelID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

type ProviderSummary struct {
	Name string
	Type string
}

func (r Repository) ProviderSummaries(ctx context.Context, scope tenant.Scope, providerIDs []string) (map[string]ProviderSummary, error) {
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
		return map[string]ProviderSummary{}, nil
	}

	var providers []database.AIProvider
	if err := db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", scope.ID(), uniqueIDs).
		Find(&providers).Error; err != nil {
		return nil, err
	}

	summaries := make(map[string]ProviderSummary, len(providers))
	for _, provider := range providers {
		summaries[provider.ID] = ProviderSummary{Name: provider.Name, Type: provider.Type}
	}
	return summaries, nil
}
