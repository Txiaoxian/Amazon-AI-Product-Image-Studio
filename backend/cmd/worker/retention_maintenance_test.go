package main

import (
	"context"
	"errors"
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
	calls     []logRetentionCleanupCall
	afterCall func()
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
	return logRetentionCleanupSummary{}, ctx.Err()
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
