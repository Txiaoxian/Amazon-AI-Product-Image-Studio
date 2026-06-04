package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
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

func TestRetentionMaintenanceRunOnceProcessesLogRetentionOnlyForValidActiveTenants(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-valid", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-null", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-invalid", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-inactive", "DISABLED", now)
	seedLogRetentionSetting(t, db, "tenant-valid", `{"operationLogRetentionDays":30,"apiCallLogRetentionDays":null,"taskEventRetentionDays":7}`, now)
	seedLogRetentionSetting(t, db, "tenant-null", `{"operationLogRetentionDays":null,"apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`, now)
	seedLogRetentionSetting(t, db, "tenant-invalid", `{"operationLogRetentionDays":0,"apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`, now)
	seedLogRetentionSetting(t, db, "tenant-inactive", `{"operationLogRetentionDays":1,"apiCallLogRetentionDays":1,"taskEventRetentionDays":1}`, now)
	cleaner := &fakeLogRetentionCleaner{}

	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:   time.Hour,
		BatchLimit: 19,
		Now:        func() time.Time { return now },
		LogCleaner: cleaner,
	})

	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if len(cleaner.calls) != 1 {
		t.Fatalf("log cleanup calls = %#v, want tenant-valid only", cleaner.calls)
	}
	call := cleaner.calls[0]
	if call.tenantID != "tenant-valid" || call.now != now || call.batchLimit != 19 {
		t.Fatalf("log cleanup call = %#v, want tenant-valid now batch=19", call)
	}
	if call.operationDays == nil || *call.operationDays != 30 || call.apiCallDays != nil || call.taskEventDays == nil || *call.taskEventDays != 7 {
		t.Fatalf("log cleanup config = %#v, want operation=30 api=nil task=7", call)
	}
}

func TestRetentionMaintenanceRunOnceContinuesAfterLogCleanupFailureWithSanitizedWarning(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)
	seedLogRetentionSetting(t, db, "tenant-a", `{"operationLogRetentionDays":30,"apiCallLogRetentionDays":14,"taskEventRetentionDays":7}`, now)
	seedLogRetentionSetting(t, db, "tenant-b", `{"operationLogRetentionDays":30,"apiCallLogRetentionDays":14,"taskEventRetentionDays":7}`, now)

	var logs bytes.Buffer
	cleaner := &fakeLogRetentionCleaner{
		errByTenant: map[string]error{
			"tenant-a": fmt.Errorf("bucket product-originals object_key tenant-a/raw.png api_key super-secret: %w", errLogRetentionCleanupFailed),
		},
		summaryByTenant: map[string]logRetentionCleanupSummary{
			"tenant-a": {
				OperationLogs: logRetentionCategorySummary{Processed: 3, Deleted: 2, Failed: 1},
				APICallLogs:   logRetentionCategorySummary{Processed: 4, Deleted: 3, Failed: 1},
				TaskEvents:    logRetentionCategorySummary{Processed: 5, Deleted: 4, Failed: 1},
			},
			"tenant-b": {
				OperationLogs: logRetentionCategorySummary{Processed: 1, Deleted: 1},
				APICallLogs:   logRetentionCategorySummary{Processed: 1, Deleted: 1},
				TaskEvents:    logRetentionCategorySummary{Processed: 1, Deleted: 1},
			},
		},
	}
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(&logs, nil)), retentionMaintenanceOptions{
		Interval:   time.Hour,
		BatchLimit: 11,
		Now:        func() time.Time { return now },
		LogCleaner: cleaner,
	})

	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if len(cleaner.calls) != 2 {
		t.Fatalf("log cleanup calls = %#v, want tenant-a failure then tenant-b success", cleaner.calls)
	}
	if cleaner.calls[0].tenantID != "tenant-a" || cleaner.calls[1].tenantID != "tenant-b" {
		t.Fatalf("log cleanup call order = %#v, want tenant-a then tenant-b", cleaner.calls)
	}

	logOutput := strings.ToLower(logs.String())
	for _, required := range []string{"log retention cleanup failed", "tenant-a", "cleanup_failed", "operation_processed=3", "api_call_processed=4", "task_event_processed=5"} {
		if !strings.Contains(logOutput, required) {
			t.Fatalf("warning log missing %q: %s", required, logs.String())
		}
	}
	for _, forbidden := range []string{"product-originals", "object_key", "raw.png", "api_key", "super-secret", "bucket"} {
		if strings.Contains(logOutput, forbidden) {
			t.Fatalf("warning log leaked %q: %s", forbidden, logs.String())
		}
	}
}

func TestRetentionMaintenanceRunOnceStopsBeforeLogTenantAfterCancel(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)
	seedLogRetentionSetting(t, db, "tenant-a", `{"operationLogRetentionDays":2,"apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`, now)
	seedLogRetentionSetting(t, db, "tenant-b", `{"operationLogRetentionDays":2,"apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`, now)
	ctx, cancel := context.WithCancel(context.Background())
	cleaner := &fakeLogRetentionCleaner{afterCall: cancel}
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:   time.Hour,
		BatchLimit: 5,
		Now:        func() time.Time { return now },
		LogCleaner: cleaner,
	})

	if err := runner.runOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("runOnce returned error = %v, want nil or context.Canceled", err)
	}
	if len(cleaner.calls) != 1 {
		t.Fatalf("log cleanup calls after cancellation = %#v, want only first tenant", cleaner.calls)
	}
}

func TestQuotaReconciliationMaintenanceProcessesOnlyActiveTenantsWithinBatchLimit(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "DISABLED", now)
	seedRetentionTenant(t, db, "tenant-c", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-d", "ACTIVE", now)
	reconciler := &fakeQuotaReconciler{}
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:        time.Hour,
		BatchLimit:      2,
		Now:             func() time.Time { return now },
		QuotaReconciler: reconciler,
	})

	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if len(reconciler.calls) != 2 {
		t.Fatalf("quota reconciliation calls = %#v, want first two active tenants", reconciler.calls)
	}
	if reconciler.calls[0].tenantID != "tenant-a" || reconciler.calls[1].tenantID != "tenant-c" {
		t.Fatalf("quota reconciliation call order = %#v, want tenant-a then tenant-c", reconciler.calls)
	}
}

func TestQuotaReconciliationMaintenanceRotatesPastBatchLimitAcrossRuns(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-c", "ACTIVE", now)
	reconciler := &fakeQuotaReconciler{}
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:        time.Hour,
		BatchLimit:      2,
		Now:             func() time.Time { return now },
		QuotaReconciler: reconciler,
	})

	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("first runOnce returned error: %v", err)
	}
	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("second runOnce returned error: %v", err)
	}
	if len(reconciler.calls) != 3 {
		t.Fatalf("quota reconciliation calls = %#v, want tenant-a tenant-b tenant-c across two runs", reconciler.calls)
	}
	if reconciler.calls[0].tenantID != "tenant-a" || reconciler.calls[1].tenantID != "tenant-b" || reconciler.calls[2].tenantID != "tenant-c" {
		t.Fatalf("quota reconciliation call order = %#v, want tenant-a tenant-b tenant-c", reconciler.calls)
	}
}

func TestQuotaReconciliationMaintenanceStopsBeforeTenantAfterCancel(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)
	ctx, cancel := context.WithCancel(context.Background())
	reconciler := &fakeQuotaReconciler{afterCall: cancel}
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:        time.Hour,
		BatchLimit:      10,
		Now:             func() time.Time { return now },
		QuotaReconciler: reconciler,
	})

	if err := runner.runOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("runOnce returned error = %v, want nil or context.Canceled", err)
	}
	if len(reconciler.calls) != 1 {
		t.Fatalf("quota reconciliation calls after cancellation = %#v, want only first tenant", reconciler.calls)
	}
}

func TestQuotaReconciliationMaintenanceContinuesAfterFailureWithSanitizedWarning(t *testing.T) {
	db := newRetentionMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)
	var logs bytes.Buffer
	reconciler := &fakeQuotaReconciler{
		errByTenant: map[string]error{
			"tenant-a": fmt.Errorf("bucket product-originals object_key tenants/tenant-a/assets/raw.png reservation reservation-secret jwt cookie provider_key base64 data:image/png: %w", settings.ErrStorageQuotaCounterInvalid),
		},
	}
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(&logs, nil)), retentionMaintenanceOptions{
		Interval:        time.Hour,
		BatchLimit:      10,
		Now:             func() time.Time { return now },
		QuotaReconciler: reconciler,
	})

	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	if len(reconciler.calls) != 2 {
		t.Fatalf("quota reconciliation calls = %#v, want tenant-a failure then tenant-b success", reconciler.calls)
	}
	logOutput := strings.ToLower(logs.String())
	for _, required := range []string{"storage quota reconciliation failed", "tenant-a", "invalid_counter"} {
		if !strings.Contains(logOutput, required) {
			t.Fatalf("warning log missing %q: %s", required, logs.String())
		}
	}
	for _, forbidden := range []string{"product-originals", "object_key", "raw.png", "reservation-secret", "reservation ", "jwt", "cookie", "provider_key", "base64", "data:image", "bucket"} {
		if strings.Contains(logOutput, forbidden) {
			t.Fatalf("warning log leaked %q: %s", forbidden, logs.String())
		}
	}
}

func TestDatabaseQuotaReconcilerReconcilesTenantScopedMetadataAndWritesSanitizedAudit(t *testing.T) {
	db := newQuotaReconciliationMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	purgedAt := now
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)
	seedStorageQuotaSetting(t, db, "tenant-a", `{"maxBytes":1000}`, now)
	seedQuotaMaintenanceAsset(t, db, "tenant-a", "asset-active", "tenants/tenant-a/projects/project-a/assets/asset-active/original.png", 100, nil, nil, now)
	seedQuotaMaintenanceAsset(t, db, "tenant-a", "asset-soft", "tenants/tenant-a/projects/project-a/assets/asset-soft/original.png", 50, &now, nil, now)
	seedQuotaMaintenanceAsset(t, db, "tenant-a", "asset-purged", "tenants/tenant-a/projects/project-a/assets/asset-purged/original.png", 200, &now, &purgedAt, now)
	seedQuotaMaintenanceAsset(t, db, "tenant-b", "asset-cross", "tenants/tenant-b/projects/project-b/assets/asset-cross/original.png", 900, nil, nil, now)
	seedQuotaMaintenanceCounter(t, db, "tenant-a", 1, 90, now)
	seedQuotaMaintenanceReservation(t, db, "reservation-secret-active", "tenant-a", 40, "RESERVED", time.Now().UTC().Add(time.Hour), now)
	seedQuotaMaintenanceReservation(t, db, "reservation-secret-stale", "tenant-a", 30, "RESERVED", time.Now().UTC().Add(-time.Hour), now)

	result, err := newDatabaseQuotaReconciler(db).ReconcileStorageQuota(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("ReconcileStorageQuota returned error: %v", err)
	}
	if result.UsedBytes != 150 || result.ReservedBytes != 40 || result.ReleasedStaleCount != 1 {
		t.Fatalf("quota reconciliation result = %#v, want used=150 reserved=40 released stale=1", result)
	}
	assertQuotaMaintenanceCounter(t, db, "tenant-a", 150, 40)
	assertQuotaMaintenanceCounterMissing(t, db, "tenant-b")

	var metadataJSON string
	if err := db.Model(&database.OperationLog{}).
		Where("tenant_id = ? AND action = ? AND resource_id = ?", "tenant-a", actionStorageQuotaReconcile, settings.KeyStorageQuota).
		Pluck("metadata_json", &metadataJSON).Error; err != nil {
		t.Fatalf("load quota reconciliation audit log: %v", err)
	}
	metadata := strings.ToLower(metadataJSON)
	for _, required := range []string{"storage_quota", "succeeded", "usedbytes", "reservedbytes", "releasedstalecount"} {
		if !strings.Contains(metadata, required) {
			t.Fatalf("audit metadata missing %q: %s", required, metadataJSON)
		}
	}
	for _, forbidden := range []string{"tenants/", "object_key", "objectkey", "product-originals", "bucket", "minio", "http://", "reservation-secret", "reservation", "authorization", "cookie", "jwt", "provider", "api_key", "apikey", "base64", "data:image"} {
		if strings.Contains(metadata, forbidden) {
			t.Fatalf("audit metadata contains %q: %s", forbidden, metadataJSON)
		}
	}
}

func TestQuotaReconciliationMaintenanceSkipsMalformedStorageQuotaFailClosed(t *testing.T) {
	db := newQuotaReconciliationMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedStorageQuotaSetting(t, db, "tenant-a", `{"maxBytes":1000,"usedBytes":1}`, now)
	seedQuotaMaintenanceAsset(t, db, "tenant-a", "asset-active", "tenants/tenant-a/projects/project-a/assets/asset-active/original.png", 100, nil, nil, now)
	seedQuotaMaintenanceCounter(t, db, "tenant-a", 1, 0, now)
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:        time.Hour,
		BatchLimit:      10,
		Now:             func() time.Time { return now },
		QuotaReconciler: newDatabaseQuotaReconciler(db),
	})

	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	assertQuotaMaintenanceCounter(t, db, "tenant-a", 1, 0)
	assertQuotaMaintenanceAuditCount(t, db, "tenant-a", 0)
}

func TestQuotaReconciliationMaintenanceMalformedCounterFailsClosed(t *testing.T) {
	db := newQuotaReconciliationMaintenanceTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedStorageQuotaSetting(t, db, "tenant-a", `{"maxBytes":1000}`, now)
	seedQuotaMaintenanceAsset(t, db, "tenant-a", "asset-active", "tenants/tenant-a/projects/project-a/assets/asset-active/original.png", 100, nil, nil, now)
	seedQuotaMaintenanceCounter(t, db, "tenant-a", -1, 0, now)
	runner := newRetentionMaintenanceRunner(settings.NewRepository(db), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), retentionMaintenanceOptions{
		Interval:        time.Hour,
		BatchLimit:      10,
		Now:             func() time.Time { return now },
		QuotaReconciler: newDatabaseQuotaReconciler(db),
	})

	if err := runner.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce returned error: %v", err)
	}
	assertQuotaMaintenanceCounter(t, db, "tenant-a", -1, 0)
	assertQuotaMaintenanceAuditCount(t, db, "tenant-a", 0)
}

func TestDatabaseLogRetentionCleanerDeletesTenantScopedBatchesAndWritesAudit(t *testing.T) {
	db := newLogRetentionCleanerTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedRetentionTenant(t, db, "tenant-b", "ACTIVE", now)

	seedOperationLog(t, db, "tenant-a", "op-old-1", now.Add(-31*24*time.Hour))
	seedOperationLog(t, db, "tenant-a", "op-old-2", now.Add(-30*24*time.Hour-time.Second))
	seedOperationLog(t, db, "tenant-a", "op-old-3", now.Add(-40*24*time.Hour))
	seedOperationLog(t, db, "tenant-a", "op-recent", now.Add(-29*24*time.Hour))
	seedOperationLog(t, db, "tenant-b", "op-cross", now.Add(-40*24*time.Hour))

	seedAPICallLog(t, db, "tenant-a", "api-old-1", now.Add(-15*24*time.Hour))
	seedAPICallLog(t, db, "tenant-a", "api-old-2", now.Add(-20*24*time.Hour))
	seedAPICallLog(t, db, "tenant-a", "api-old-3", now.Add(-25*24*time.Hour))
	seedAPICallLog(t, db, "tenant-a", "api-recent", now.Add(-13*24*time.Hour))
	seedAPICallLog(t, db, "tenant-b", "api-cross", now.Add(-25*24*time.Hour))

	seedGenerationTask(t, db, "tenant-a", "task-terminal", task.StatusSucceeded, now)
	seedGenerationTask(t, db, "tenant-a", "task-running", task.StatusRunning, now)
	seedGenerationTask(t, db, "tenant-a", "task-retrying", task.StatusRetrying, now)
	seedGenerationTask(t, db, "tenant-b", "task-cross", task.StatusSucceeded, now)
	seedTaskEvent(t, db, "tenant-a", "task-terminal", "evt-old-1", now.Add(-8*24*time.Hour))
	seedTaskEvent(t, db, "tenant-a", "task-terminal", "evt-old-2", now.Add(-9*24*time.Hour))
	seedTaskEvent(t, db, "tenant-a", "task-terminal", "evt-old-3", now.Add(-10*24*time.Hour))
	seedTaskEvent(t, db, "tenant-a", "task-terminal", "evt-recent", now.Add(-6*24*time.Hour))
	seedTaskEvent(t, db, "tenant-a", "task-running", "evt-running-old", now.Add(-10*24*time.Hour))
	seedTaskEvent(t, db, "tenant-a", "task-retrying", "evt-retrying-old", now.Add(-10*24*time.Hour))
	seedTaskEvent(t, db, "tenant-b", "task-cross", "evt-cross", now.Add(-10*24*time.Hour))

	cleaner := newDatabaseLogRetentionCleaner(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	summary, err := cleaner.CleanupLogRetention(context.Background(), settings.EnabledLogRetention{
		TenantID:                  "tenant-a",
		OperationLogRetentionDays: ptrWorkerInt(30),
		APICallLogRetentionDays:   ptrWorkerInt(14),
		TaskEventRetentionDays:    ptrWorkerInt(7),
	}, now, 2)
	if err != nil {
		t.Fatalf("CleanupLogRetention returned error: %v", err)
	}
	if summary.OperationLogs.Processed != 2 || summary.OperationLogs.Deleted != 2 {
		t.Fatalf("operation summary = %#v, want processed/deleted 2", summary.OperationLogs)
	}
	if summary.APICallLogs.Processed != 2 || summary.APICallLogs.Deleted != 2 {
		t.Fatalf("api call summary = %#v, want processed/deleted 2", summary.APICallLogs)
	}
	if summary.TaskEvents.Processed != 2 || summary.TaskEvents.Deleted != 2 {
		t.Fatalf("task event summary = %#v, want processed/deleted 2", summary.TaskEvents)
	}

	assertRowMissing(t, db, &database.OperationLog{}, "id = ?", "op-old-1")
	assertRowExists(t, db, &database.OperationLog{}, "id = ?", "op-old-2")
	assertRowMissing(t, db, &database.OperationLog{}, "id = ?", "op-old-3")
	assertRowExists(t, db, &database.OperationLog{}, "id = ?", "op-recent")
	assertRowExists(t, db, &database.OperationLog{}, "id = ?", "op-cross")
	assertRowMissing(t, db, &database.APICallLog{}, "id = ?", "api-old-2")
	assertRowExists(t, db, &database.APICallLog{}, "id = ?", "api-old-1")
	assertRowExists(t, db, &database.APICallLog{}, "id = ?", "api-recent")
	assertRowExists(t, db, &database.APICallLog{}, "id = ?", "api-cross")
	assertRowExists(t, db, &database.TaskEvent{}, "id = ?", "evt-old-1")
	assertRowMissing(t, db, &database.TaskEvent{}, "id = ?", "evt-old-3")
	assertRowExists(t, db, &database.TaskEvent{}, "id = ?", "evt-recent")
	assertRowExists(t, db, &database.TaskEvent{}, "id = ?", "evt-running-old")
	assertRowExists(t, db, &database.TaskEvent{}, "id = ?", "evt-retrying-old")
	assertRowExists(t, db, &database.TaskEvent{}, "id = ?", "evt-cross")

	var metadataJSON string
	if err := db.Model(&database.OperationLog{}).
		Where("tenant_id = ? AND action = ? AND resource_id = ?", "tenant-a", actionLogRetentionCleanup, settings.KeyLogRetention).
		Pluck("metadata_json", &metadataJSON).Error; err != nil {
		t.Fatalf("load cleanup audit log: %v", err)
	}
	metadata := strings.ToLower(metadataJSON)
	for _, required := range []string{"log_retention", "operationlogs", "apicalllogs", "taskevents", "processed", "deleted", "failed"} {
		if !strings.Contains(metadata, required) {
			t.Fatalf("audit metadata missing %q: %s", required, metadataJSON)
		}
	}
	for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "jwt", "base64", "data:image", "object_key", "bucket", "minio", "prompt"} {
		if strings.Contains(metadata, forbidden) {
			t.Fatalf("audit metadata contains %q: %s", forbidden, metadataJSON)
		}
	}
}

func TestDatabaseLogRetentionCleanerIsRepeatableAndHonorsNullCategories(t *testing.T) {
	db := newLogRetentionCleanerTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedOperationLog(t, db, "tenant-a", "op-old", now.Add(-31*24*time.Hour))
	seedAPICallLog(t, db, "tenant-a", "api-old", now.Add(-31*24*time.Hour))
	cleaner := newDatabaseLogRetentionCleaner(db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for i := 0; i < 2; i++ {
		if _, err := cleaner.CleanupLogRetention(context.Background(), settings.EnabledLogRetention{
			TenantID:                  "tenant-a",
			OperationLogRetentionDays: ptrWorkerInt(30),
			APICallLogRetentionDays:   nil,
			TaskEventRetentionDays:    nil,
		}, now, 100); err != nil {
			t.Fatalf("CleanupLogRetention run %d returned error: %v", i+1, err)
		}
	}
	assertRowMissing(t, db, &database.OperationLog{}, "id = ?", "op-old")
	assertRowExists(t, db, &database.APICallLog{}, "id = ?", "api-old")
}

func TestDatabaseLogRetentionCleanerReturnsContextCancelWithoutDeleting(t *testing.T) {
	db := newLogRetentionCleanerTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedOperationLog(t, db, "tenant-a", "op-old", now.Add(-31*24*time.Hour))
	cleaner := newDatabaseLogRetentionCleaner(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := cleaner.CleanupLogRetention(ctx, settings.EnabledLogRetention{
		TenantID:                  "tenant-a",
		OperationLogRetentionDays: ptrWorkerInt(30),
	}, now, 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("CleanupLogRetention error = %v, want context.Canceled", err)
	}
	assertRowExists(t, db, &database.OperationLog{}, "id = ?", "op-old")
}

func TestDatabaseLogRetentionCleanerSQLFailureReturnsSanitizedErrorAndRollsBack(t *testing.T) {
	db := newLogRetentionCleanerTestDB(t)
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	seedRetentionTenant(t, db, "tenant-a", "ACTIVE", now)
	seedOperationLog(t, db, "tenant-a", "op-old", now.Add(-31*24*time.Hour))
	if err := db.Exec("DROP TABLE api_call_logs").Error; err != nil {
		t.Fatalf("drop api_call_logs to induce SQL failure: %v", err)
	}
	cleaner := newDatabaseLogRetentionCleaner(db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	summary, err := cleaner.CleanupLogRetention(context.Background(), settings.EnabledLogRetention{
		TenantID:                  "tenant-a",
		OperationLogRetentionDays: ptrWorkerInt(30),
		APICallLogRetentionDays:   ptrWorkerInt(30),
	}, now, 100)
	if !errors.Is(err, errLogRetentionCleanupFailed) {
		t.Fatalf("CleanupLogRetention error = %v, want errLogRetentionCleanupFailed", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "api_call_logs") {
		t.Fatalf("cleanup error leaked SQL table name: %v", err)
	}
	if summary.OperationLogs.Processed != 1 || summary.OperationLogs.Deleted != 1 {
		t.Fatalf("operation summary = %#v, want attempted processed/deleted before rollback", summary.OperationLogs)
	}
	assertRowExists(t, db, &database.OperationLog{}, "id = ?", "op-old")
	var auditRows int64
	if err := db.Model(&database.OperationLog{}).Where("tenant_id = ? AND action = ?", "tenant-a", actionLogRetentionCleanup).Count(&auditRows).Error; err != nil {
		t.Fatalf("count cleanup audit logs: %v", err)
	}
	if auditRows != 0 {
		t.Fatalf("cleanup audit rows after SQL failure = %d, want 0", auditRows)
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

type fakeLogRetentionCleaner struct {
	calls           []logRetentionCleanupCall
	errByTenant     map[string]error
	summaryByTenant map[string]logRetentionCleanupSummary
	afterCall       func()
}

type fakeQuotaReconciler struct {
	calls          []quotaReconciliationCall
	errByTenant    map[string]error
	resultByTenant map[string]quotaReconciliationResult
	afterCall      func()
}

type logRetentionCleanupCall struct {
	tenantID      string
	now           time.Time
	batchLimit    int
	operationDays *int
	apiCallDays   *int
	taskEventDays *int
}

func (c *fakeLogRetentionCleaner) CleanupLogRetention(ctx context.Context, config settings.EnabledLogRetention, now time.Time, batchLimit int) (logRetentionCleanupSummary, error) {
	c.calls = append(c.calls, logRetentionCleanupCall{
		tenantID:      config.TenantID,
		now:           now,
		batchLimit:    batchLimit,
		operationDays: config.OperationLogRetentionDays,
		apiCallDays:   config.APICallLogRetentionDays,
		taskEventDays: config.TaskEventRetentionDays,
	})
	if c.afterCall != nil {
		c.afterCall()
	}
	summary := c.summaryByTenant[config.TenantID]
	if err := c.errByTenant[config.TenantID]; err != nil {
		return summary, err
	}
	return summary, ctx.Err()
}

type quotaReconciliationCall struct {
	tenantID string
}

func (r *fakeQuotaReconciler) ReconcileStorageQuota(ctx context.Context, tenantID string) (quotaReconciliationResult, error) {
	r.calls = append(r.calls, quotaReconciliationCall{tenantID: tenantID})
	if r.afterCall != nil {
		r.afterCall()
	}
	result := r.resultByTenant[tenantID]
	if err := r.errByTenant[tenantID]; err != nil {
		return result, err
	}
	return result, ctx.Err()
}

func assertRetentionCleanupCall(t *testing.T, got retentionCleanupCall, tenantID string, cutoff time.Time, batchLimit int) {
	t.Helper()
	if got.tenantID != tenantID || !got.cutoff.Equal(cutoff) || got.batchLimit != batchLimit {
		t.Fatalf("cleanup call = %#v, want tenant=%s cutoff=%s batch=%d", got, tenantID, cutoff, batchLimit)
	}
}

func ptrWorkerInt(value int) *int {
	return &value
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

func newLogRetentionCleanerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open log retention sqlite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access log retention sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&database.Tenant{}, &database.OperationLog{}, &database.GenerationTask{}, &database.APICallLog{}, &database.TaskEvent{}); err != nil {
		t.Fatalf("migrate log retention test schema: %v", err)
	}
	return db
}

func newQuotaReconciliationMaintenanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open quota reconciliation sqlite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access quota reconciliation sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&database.Tenant{}, &database.SystemSetting{}, &database.StorageQuotaCounter{}, &database.StorageQuotaReservation{}, &database.ImageAsset{}, &database.OperationLog{}); err != nil {
		t.Fatalf("migrate quota reconciliation test schema: %v", err)
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

func seedLogRetentionSetting(t *testing.T, db *gorm.DB, tenantID string, valueJSON string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.SystemSetting{
		ID:        "log-setting-" + tenantID,
		TenantID:  tenantID,
		Key:       settings.KeyLogRetention,
		ValueJSON: valueJSON,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed log retention setting %s: %v", tenantID, err)
	}
}

func seedStorageQuotaSetting(t *testing.T, db *gorm.DB, tenantID string, valueJSON string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.SystemSetting{
		ID:        "quota-setting-" + tenantID,
		TenantID:  tenantID,
		Key:       settings.KeyStorageQuota,
		ValueJSON: valueJSON,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed storage quota setting %s: %v", tenantID, err)
	}
}

func seedQuotaMaintenanceAsset(t *testing.T, db *gorm.DB, tenantID string, assetID string, objectKey string, sizeBytes int64, deletedAt *time.Time, purgedAt *time.Time, now time.Time) {
	t.Helper()
	record := database.ImageAsset{
		ID:        assetID,
		TenantID:  tenantID,
		ProjectID: "project-" + tenantID,
		Kind:      "REFERENCE",
		Filename:  assetID + ".png",
		ObjectKey: objectKey,
		MimeType:  "image/png",
		SizeBytes: sizeBytes,
		Width:     1,
		Height:    1,
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CreatedBy: "user-" + tenantID,
		CreatedAt: now,
		UpdatedAt: now,
		PurgedAt:  purgedAt,
	}
	if deletedAt != nil {
		record.DeletedAt.Valid = true
		record.DeletedAt.Time = deletedAt.UTC()
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed quota maintenance asset %s/%s: %v", tenantID, assetID, err)
	}
}

func seedQuotaMaintenanceCounter(t *testing.T, db *gorm.DB, tenantID string, usedBytes int64, reservedBytes int64, now time.Time) {
	t.Helper()
	if err := db.Create(&database.StorageQuotaCounter{
		ID:            "quota-counter-" + tenantID,
		TenantID:      tenantID,
		UsedBytes:     usedBytes,
		ReservedBytes: reservedBytes,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("seed quota maintenance counter %s: %v", tenantID, err)
	}
}

func seedQuotaMaintenanceReservation(t *testing.T, db *gorm.DB, id string, tenantID string, bytes int64, status string, expiresAt time.Time, now time.Time) {
	t.Helper()
	if err := db.Create(&database.StorageQuotaReservation{
		ID:        id,
		TenantID:  tenantID,
		Bytes:     bytes,
		Status:    status,
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed quota maintenance reservation %s: %v", id, err)
	}
}

func seedOperationLog(t *testing.T, db *gorm.DB, tenantID string, id string, createdAt time.Time) {
	t.Helper()
	if err := db.Create(&database.OperationLog{
		ID:           id,
		TenantID:     tenantID,
		Action:       "test.action",
		ResourceType: "test",
		ResourceID:   id,
		MetadataJSON: `{}`,
		CreatedAt:    createdAt.UTC(),
	}).Error; err != nil {
		t.Fatalf("seed operation log %s/%s: %v", tenantID, id, err)
	}
}

func seedAPICallLog(t *testing.T, db *gorm.DB, tenantID string, id string, createdAt time.Time) {
	t.Helper()
	if err := db.Create(&database.APICallLog{
		ID:                  id,
		TenantID:            tenantID,
		TaskID:              "task-" + id,
		ProviderID:          "provider-" + id,
		ModelID:             "model-" + id,
		Status:              "SUCCESS",
		RedactedRequestJSON: `{}`,
		CreatedAt:           createdAt.UTC(),
	}).Error; err != nil {
		t.Fatalf("seed api call log %s/%s: %v", tenantID, id, err)
	}
}

func seedGenerationTask(t *testing.T, db *gorm.DB, tenantID string, taskID string, status string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.GenerationTask{
		ID:                taskID,
		TenantID:          tenantID,
		ProjectID:         "project-" + tenantID,
		Type:              task.TypeImageGeneration,
		ProviderID:        "provider-" + taskID,
		ModelID:           "model-" + taskID,
		Status:            status,
		Prompt:            "redacted test prompt",
		ParamsJSON:        `{}`,
		InputAssetIDsJSON: `[]`,
		Attempt:           1,
		MaxAttempts:       3,
		CreatedBy:         "user-" + tenantID,
		CreatedAt:         now.UTC(),
		UpdatedAt:         now.UTC(),
	}).Error; err != nil {
		t.Fatalf("seed generation task %s/%s: %v", tenantID, taskID, err)
	}
}

func seedTaskEvent(t *testing.T, db *gorm.DB, tenantID string, taskID string, eventID string, createdAt time.Time) {
	t.Helper()
	if err := db.Create(&database.TaskEvent{
		ID:               eventID,
		TenantID:         tenantID,
		TaskID:           taskID,
		ProjectID:        "project-" + tenantID,
		EventType:        task.EventTaskProgress,
		EventPayloadJSON: `{}`,
		CreatedAt:        createdAt.UTC(),
	}).Error; err != nil {
		t.Fatalf("seed task event %s/%s: %v", tenantID, eventID, err)
	}
}

func assertRowExists(t *testing.T, db *gorm.DB, model any, query string, args ...any) {
	t.Helper()
	var count int64
	if err := db.Model(model).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatalf("count row exists: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected row matching %s %v to exist", query, args)
	}
}

func assertRowMissing(t *testing.T, db *gorm.DB, model any, query string, args ...any) {
	t.Helper()
	var count int64
	if err := db.Model(model).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatalf("count row missing: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected row matching %s %v to be missing, found %d", query, args, count)
	}
}

func assertQuotaMaintenanceCounter(t *testing.T, db *gorm.DB, tenantID string, usedBytes int64, reservedBytes int64) {
	t.Helper()
	var counter database.StorageQuotaCounter
	if err := db.Model(&database.StorageQuotaCounter{}).
		Select("tenant_id, used_bytes, reserved_bytes").
		Where("tenant_id = ?", tenantID).
		First(&counter).Error; err != nil {
		t.Fatalf("load quota maintenance counter %s: %v", tenantID, err)
	}
	if counter.UsedBytes != usedBytes || counter.ReservedBytes != reservedBytes {
		t.Fatalf("quota maintenance counter %s used/reserved = %d/%d, want %d/%d", tenantID, counter.UsedBytes, counter.ReservedBytes, usedBytes, reservedBytes)
	}
}

func assertQuotaMaintenanceCounterMissing(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	var count int64
	if err := db.Model(&database.StorageQuotaCounter{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		t.Fatalf("count quota maintenance counter %s: %v", tenantID, err)
	}
	if count != 0 {
		t.Fatalf("quota maintenance counter %s exists, want missing", tenantID)
	}
}

func assertQuotaMaintenanceAuditCount(t *testing.T, db *gorm.DB, tenantID string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&database.OperationLog{}).Where("tenant_id = ? AND action = ?", tenantID, actionStorageQuotaReconcile).Count(&count).Error; err != nil {
		t.Fatalf("count quota maintenance audit logs: %v", err)
	}
	if count != want {
		t.Fatalf("quota maintenance audit rows for %s = %d, want %d", tenantID, count, want)
	}
}
