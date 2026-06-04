package settings

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
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

func (r Repository) FindByKey(ctx context.Context, scope tenant.Scope, key string) (database.SystemSetting, bool, error) {
	db, err := r.base(ctx, scope)
	if err != nil {
		return database.SystemSetting{}, false, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return database.SystemSetting{}, false, ErrValidation
	}

	var record database.SystemSetting
	err = db.Select("id, tenant_id, `key`, value_json").
		Where("tenant_id = ? AND `key` = ?", scope.ID(), key).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.SystemSetting{}, false, nil
	}
	if err != nil {
		return database.SystemSetting{}, false, err
	}
	return record, true, nil
}

func (r Repository) StorageUsedBytes(ctx context.Context, scope tenant.Scope) (int64, error) {
	return storageQuotaCounterUsedBytes(ctx, r, scope)
}

func (r Repository) ListActiveTenantIDs(ctx context.Context, limit int) ([]string, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if limit <= 0 {
		return nil, ErrValidation
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var tenantIDs []string
	err := r.db.WithContext(ctx).
		Model(&database.Tenant{}).
		Select("id").
		Where("status = ?", "ACTIVE").
		Order("id ASC").
		Limit(limit).
		Pluck("id", &tenantIDs).Error
	return tenantIDs, err
}

func (r Repository) ListByKeyForActiveTenants(ctx context.Context, key string) ([]database.SystemSetting, error) {
	if r.db == nil {
		return nil, database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrValidation
	}

	var records []database.SystemSetting
	err := r.db.WithContext(ctx).
		Model(&database.SystemSetting{}).
		Select("system_settings.id, system_settings.tenant_id, system_settings.`key`, system_settings.value_json").
		Joins("JOIN tenants ON tenants.id = system_settings.tenant_id AND tenants.status = ?", "ACTIVE").
		Where("system_settings.`key` = ?", key).
		Order("system_settings.tenant_id ASC").
		Find(&records).Error
	return records, err
}

func (r Repository) Upsert(ctx context.Context, scope tenant.Scope, key string, valueJSON string, now time.Time) error {
	db, err := r.base(ctx, scope)
	if err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	valueJSON = strings.TrimSpace(valueJSON)
	if key == "" || valueJSON == "" {
		return ErrValidation
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	record := database.SystemSetting{
		ID:        idgen.New(),
		TenantID:  scope.ID(),
		Key:       key,
		ValueJSON: valueJSON,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value_json": valueJSON,
			"updated_at": now,
		}),
	}).Create(&record).Error
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
		return database.AIProvider{}, ErrValidation
	}
	return record, err
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
		return database.AIModel{}, ErrValidation
	}
	return record, err
}
