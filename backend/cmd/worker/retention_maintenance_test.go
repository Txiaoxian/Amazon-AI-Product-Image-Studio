package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestRetentionMaintenanceRunOnceProcessesOnlyValidTenantsAndContinuesAfterFailure(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-c", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-disabled", "DISABLED", now)
	seedRetentionSetting(t, db, "tenant-a", `{"deletedAssetRetentionDays":30}`, now)
	seedRetentionSetting(t, db, "tenant-b", `{"deletedAssetRetentionDays":0}`, now)
	seedRetentionSetting(t, db, "tenant-c", `{"deletedAssetRetentionDays":7}`, now)
	seedRetentionSetting(t, db, "tenant-disabled", `{"deletedAssetRetentionDays":1}`, now)
	cleaner := &fakeRetentionCleaner{errByTenant: map[string]error{"tenant-a": errors.New("temporary cleanup failure with object key")}}

	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), cleaner, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:   time.Hour,
		BatchLimit: 17,
		Now:        func() time.Time { return now },
	})

	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if len(cleaner.calls) != 2 {
		t.Fatalf("cleanup calls = %#v, want tenant-a and tenant-c only", cleaner.calls)
	}
	assertRetentionCleanupCall(t, cleaner.calls[0], "tenant-a", now.Add(-30*24*time.Hour), 17)
	assertRetentionCleanupCall(t, cleaner.calls[1], "tenant-c", now.Add(-7*24*time.Hour), 17)
}

func TestRetentionMaintenanceLoopStopsOnContextCancel(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionSetting(t, db, "tenant-a", `{"deletedAssetRetentionDays":1}`, now)
	cleaner := &fakeRetentionCleaner{}
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), cleaner, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:   10 * time.Millisecond,
		BatchLimit: 3,
		Now:        func() time.Time { return now },
	})

	ctx, cancel := context.WithCancel(context.Background())
	loop := startRetentionMaintenanceLoop(ctx, runner)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := loop.Stop(shutdownCtx); err != nil {
		t.Fatalf("retention maintenance Stop returned error: %v", err)
	}
}

func TestRetentionMaintenanceRunOnceStopsBeforeStartingNewTenantAfterCancel(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)
	seedRetentionSetting(t, db, "tenant-a", `{"deletedAssetRetentionDays":2}`, now)
	seedRetentionSetting(t, db, "tenant-b", `{"deletedAssetRetentionDays":2}`, now)
	ctx, cancel := context.WithCancel(context.Background())
	cleaner := &fakeRetentionCleaner{afterCall: cancel}
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), cleaner, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:   time.Hour,
		BatchLimit: 5,
		Now:        func() time.Time { return now },
	})

	if err := runner.runOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("runOnce returned error = %v, want nil or context.Canceled", err)
	}
	if len(cleaner.calls) != 1 {
		t.Fatalf("cleanup calls after cancellation = %#v, want only first tenant", cleaner.calls)
	}
}

type fakeRetentionCleaner struct {
	calls       []retentionCleanupCall
	errByTenant map[string]error
	afterCall   func()
}

type retentionCleanupCall struct {
	tenantID   string
	cutoff     time.Time
	batchLimit int
}

func (c *fakeRetentionCleaner) PurgeDeletedAssets(ctx context.Context, tenantID string, cutoff time.Time, options asset.PurgeOptions) (asset.PurgeResult, error) {
	c.calls = append(c.calls, retentionCleanupCall{tenantID: tenantID, cutoff: cutoff, batchLimit: options.BatchLimit})
	if c.afterCall != nil {
		c.afterCall()
	}
	if err := c.errByTenant[tenantID]; err != nil {
		return asset.PurgeResult{Processed: 2, Purged: 1, Failed: 1}, err
	}
	return asset.PurgeResult{Processed: 1, Purged: 1}, ctx.Err()
}

func assertRetentionCleanupCall(t *testing.T, got retentionCleanupCall, tenantID string, cutoff time.Time, batchLimit int) {
	t.Helper()
	if got.tenantID != tenantID || !got.cutoff.Equal(cutoff) || got.batchLimit != batchLimit {
		t.Fatalf("cleanup call = %#v, want tenant=%s cutoff=%s batch=%d", got, tenantID, cutoff, batchLimit)
	}
}

func newRetentionMaintenanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open retention sqlite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access retention sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&database.Tenant{}, &database.SystemSetting{}); err != nil {
		t.Fatalf("migrate retention test schema: %v", err)
	}
	return db
}

func seedRetentionTenant(t *testing.T, db *gorm.DB, tenantID string, status string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.Tenant{
		ID:        tenantID,
		Name:      tenantID,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed retention tenant %s: %v", tenantID, err)
	}
}

func seedRetentionSetting(t *testing.T, db *gorm.DB, tenantID string, valueJSON string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.SystemSetting{
		ID:        "setting-" + tenantID,
		TenantID:  tenantID,
		Key:       settings.KeyStorageRetention,
		ValueJSON: valueJSON,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed retention setting %s: %v", tenantID, err)
	}
}
