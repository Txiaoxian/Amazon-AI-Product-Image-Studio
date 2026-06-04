package provider

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
	return r.findProvider(ctx, scope, providerID, false)
}

func (r Repository) LockProvider(ctx context.Context, scope tenant.Scope, providerID string) (database.AIProvider, error) {
	return r.findProvider(ctx, scope, providerID, true)
}

func (r Repository) findProvider(ctx context.Context, scope tenant.Scope, providerID string, lock bool) (database.AIProvider, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.AIProvider{}, err
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return database.AIProvider{}, ErrValidation
	}

	var record database.AIProvider
	query := db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.ID(), providerID).
		Limit(1)
	if lock && db.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err = query.First(&record).Error
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
		Updates(map[string]any{
			"encrypted_api_key":  "",
			"api_key_hint":       "",
			"api_key_updated_at": nil,
			"deleted_at":         deletedAt.UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r Repository) CountLinkedModels(ctx context.Context, scope tenant.Scope, providerID string) (int64, error) {
	return r.countLinkedModels(ctx, scope, providerID, "")
}

func (r Repository) CountLinkedModelsByStatus(ctx context.Context, scope tenant.Scope, providerID string, status string) (int64, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		return 0, ErrValidation
	}
	return r.countLinkedModels(ctx, scope, providerID, status)
}

func (r Repository) countLinkedModels(ctx context.Context, scope tenant.Scope, providerID string, status string) (int64, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return 0, err
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return 0, ErrValidation
	}

	var count int64
	query := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND provider_id = ? AND deleted_at IS NULL", scope.ID(), providerID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
