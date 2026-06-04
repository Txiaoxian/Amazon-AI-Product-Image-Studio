package main

import (
	"context"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

const actionStorageQuotaReconcile = "storage_quota.reconcile"

type quotaReconciler interface {
	ReconcileStorageQuota(ctx context.Context, tenantID string) (quotaReconciliationResult, error)
}

type quotaReconciliationResult struct {
	UsedBytes          int64
	ReservedBytes      int64
	ReleasedStaleCount int64
}

type databaseQuotaReconciler struct {
	db   *gorm.DB
	repo settings.Repository
}

func newDatabaseQuotaReconciler(db *gorm.DB) *databaseQuotaReconciler {
	return &databaseQuotaReconciler{
		db:   db,
		repo: settings.NewRepository(db),
	}
}

func (r *databaseQuotaReconciler) ReconcileStorageQuota(ctx context.Context, tenantID string) (quotaReconciliationResult, error) {
	if r == nil || r.db == nil {
		return quotaReconciliationResult{}, database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	scope, err := tenant.NewScope(tenantID)
	if err != nil {
		return quotaReconciliationResult{}, err
	}
	if _, err := settings.LoadStorageQuota(ctx, r.repo, scope); err != nil {
		return quotaReconciliationResult{}, err
	}
	result, err := settings.ReconcileStorageQuotaCounterWithResult(ctx, r.repo, scope)
	if err != nil {
		return quotaReconciliationResult{}, err
	}
	maintenanceResult := quotaReconciliationResult{
		UsedBytes:          result.UsedBytes,
		ReservedBytes:      result.ReservedBytes,
		ReleasedStaleCount: result.ReleasedStaleCount,
	}
	if err := audit.NewRecorder(r.db).Record(ctx, audit.Event{
		TenantID:     scope.ID(),
		Action:       actionStorageQuotaReconcile,
		ResourceType: "system_settings",
		ResourceID:   settings.KeyStorageQuota,
		Metadata: map[string]any{
			"key":                settings.KeyStorageQuota,
			"status":             "succeeded",
			"usedBytes":          maintenanceResult.UsedBytes,
			"reservedBytes":      maintenanceResult.ReservedBytes,
			"releasedStaleCount": maintenanceResult.ReleasedStaleCount,
		},
	}); err != nil {
		return maintenanceResult, err
	}
	return maintenanceResult, ctx.Err()
}
