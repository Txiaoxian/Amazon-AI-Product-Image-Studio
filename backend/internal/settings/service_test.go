package settings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestLoadStorageRetentionUsesNullableTenantScopedValue(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-b", "ACTIVE", now)
	seedSettingsRow(t, db, "tenant-b", KeyStorageRetention, `{"deletedAssetRetentionDays":45}`, now)

	tenantA, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant A scope: %v", err)
	}
	retention, err := LoadStorageRetention(context.Background(), repo, tenantA)
	if err != nil {
		t.Fatalf("load tenant A retention: %v", err)
	}
	if retention.DeletedAssetRetentionDays != nil {
		t.Fatalf("tenant A deletedAssetRetentionDays = %#v, want nil fallback", retention.DeletedAssetRetentionDays)
	}

	tenantB, err := tenant.NewScope("tenant-b")
	if err != nil {
		t.Fatalf("tenant B scope: %v", err)
	}
	retention, err = LoadStorageRetention(context.Background(), repo, tenantB)
	if err != nil {
		t.Fatalf("load tenant B retention: %v", err)
	}
	if retention.DeletedAssetRetentionDays == nil || *retention.DeletedAssetRetentionDays != 45 {
		t.Fatalf("tenant B deletedAssetRetentionDays = %#v, want 45", retention.DeletedAssetRetentionDays)
	}
}

func TestLoadStorageQuotaNullableUsageAndTenantIsolation(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	purgedAt := now
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-b", "ACTIVE", now)
	seedSettingsRow(t, db, "tenant-b", KeyStorageQuota, `{"maxBytes":1024}`, now)
	seedSettingsAsset(t, db, "tenant-a", "asset-active", 100, nil, nil, now)
	seedSettingsAsset(t, db, "tenant-a", "asset-soft", 200, &now, nil, now)
	seedSettingsAsset(t, db, "tenant-a", "asset-purged", 400, &now, &purgedAt, now)
	seedSettingsAsset(t, db, "tenant-b", "asset-cross", 800, nil, nil, now)

	tenantA, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant A scope: %v", err)
	}
	quota, err := LoadStorageQuota(context.Background(), repo, tenantA)
	if err != nil {
		t.Fatalf("load tenant A quota: %v", err)
	}
	if quota.MaxBytes != nil {
		t.Fatalf("tenant A maxBytes = %#v, want nil fallback", quota.MaxBytes)
	}
	used, err := repo.StorageUsedBytes(context.Background(), tenantA)
	if err != nil {
		t.Fatalf("tenant A used bytes: %v", err)
	}
	if used != 300 {
		t.Fatalf("tenant A used bytes = %d, want 300", used)
	}

	tenantB, err := tenant.NewScope("tenant-b")
	if err != nil {
		t.Fatalf("tenant B scope: %v", err)
	}
	quota, err = LoadStorageQuota(context.Background(), repo, tenantB)
	if err != nil {
		t.Fatalf("load tenant B quota: %v", err)
	}
	if quota.MaxBytes == nil || *quota.MaxBytes != 1024 {
		t.Fatalf("tenant B maxBytes = %#v, want 1024", quota.MaxBytes)
	}
}

func TestStorageQuotaRejectsMalformedStoredValuesAndFailClosed(t *testing.T) {
	tests := []struct {
		name      string
		valueJSON string
	}{
		{name: "invalid json", valueJSON: `{`},
		{name: "unknown field", valueJSON: `{"maxBytes":10,"usedBytes":1}`},
		{name: "zero", valueJSON: `{"maxBytes":0}`},
		{name: "negative", valueJSON: `{"maxBytes":-1}`},
		{name: "over range", valueJSON: `{"maxBytes":109951162777601}`},
		{name: "string", valueJSON: `{"maxBytes":"30"}`},
		{name: "float", valueJSON: `{"maxBytes":1.5}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newSettingsTestDB(t)
			repo := NewRepository(db)
			now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
			seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
			seedSettingsRow(t, db, "tenant-a", KeyStorageQuota, tc.valueJSON, now)
			scope, err := tenant.NewScope("tenant-a")
			if err != nil {
				t.Fatalf("tenant scope: %v", err)
			}

			if _, err = LoadStorageQuota(context.Background(), repo, scope); !errors.Is(err, ErrStoredStorageQuotaInvalid) {
				t.Fatalf("LoadStorageQuota error = %v, want ErrStoredStorageQuotaInvalid", err)
			}
			if err = CheckStorageQuota(context.Background(), repo, scope, 1); !errors.Is(err, ErrStoredStorageQuotaInvalid) {
				t.Fatalf("CheckStorageQuota error = %v, want ErrStoredStorageQuotaInvalid", err)
			}
		})
	}
}

func TestCheckStorageQuotaAllowsUnlimitedAndRejectsExceeded(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsAsset(t, db, "tenant-a", "asset-active", 100, nil, nil, now)
	scope, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant scope: %v", err)
	}

	if err := CheckStorageQuota(context.Background(), repo, scope, 1000); err != nil {
		t.Fatalf("unlimited CheckStorageQuota error = %v", err)
	}
	seedSettingsRow(t, db, "tenant-a", KeyStorageQuota, `{"maxBytes":150}`, now)
	if err := CheckStorageQuota(context.Background(), repo, scope, 50); err != nil {
		t.Fatalf("in-quota CheckStorageQuota error = %v", err)
	}
	if err := CheckStorageQuota(context.Background(), repo, scope, 51); !errors.Is(err, ErrStorageQuotaExceeded) {
		t.Fatalf("exceeded CheckStorageQuota error = %v, want ErrStorageQuotaExceeded", err)
	}
}

func TestLoadStorageRetentionRejectsMalformedStoredValues(t *testing.T) {
	tests := []struct {
		name      string
		valueJSON string
	}{
		{name: "invalid json", valueJSON: `{`},
		{name: "unknown field", valueJSON: `{"deletedAssetRetentionDays":30,"storageQuotaBytes":1}`},
		{name: "zero", valueJSON: `{"deletedAssetRetentionDays":0}`},
		{name: "negative", valueJSON: `{"deletedAssetRetentionDays":-1}`},
		{name: "over range", valueJSON: `{"deletedAssetRetentionDays":3651}`},
		{name: "string", valueJSON: `{"deletedAssetRetentionDays":"30"}`},
		{name: "float", valueJSON: `{"deletedAssetRetentionDays":1.5}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newSettingsTestDB(t)
			repo := NewRepository(db)
			now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
			seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
			seedSettingsRow(t, db, "tenant-a", KeyStorageRetention, tc.valueJSON, now)
			scope, err := tenant.NewScope("tenant-a")
			if err != nil {
				t.Fatalf("tenant scope: %v", err)
			}

			_, err = LoadStorageRetention(context.Background(), repo, scope)
			if !errors.Is(err, ErrStoredStorageRetentionInvalid) {
				t.Fatalf("LoadStorageRetention error = %v, want ErrStoredStorageRetentionInvalid", err)
			}
		})
	}
}

func TestLoadEnabledStorageRetentionsSkipsNullInvalidAndInactiveTenants(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-valid", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-null", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-invalid", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-inactive", "DISABLED", now)
	seedSettingsRow(t, db, "tenant-valid", KeyStorageRetention, `{"deletedAssetRetentionDays":7}`, now)
	seedSettingsRow(t, db, "tenant-null", KeyStorageRetention, `{"deletedAssetRetentionDays":null}`, now)
	seedSettingsRow(t, db, "tenant-invalid", KeyStorageRetention, `{"deletedAssetRetentionDays":0}`, now)
	seedSettingsRow(t, db, "tenant-inactive", KeyStorageRetention, `{"deletedAssetRetentionDays":3}`, now)

	enabled, invalid, err := LoadEnabledStorageRetentions(context.Background(), repo)
	if err != nil {
		t.Fatalf("LoadEnabledStorageRetentions returned error: %v", err)
	}
	if len(enabled) != 1 || enabled[0].TenantID != "tenant-valid" || enabled[0].DeletedAssetRetentionDays != 7 {
		t.Fatalf("enabled retentions = %#v, want tenant-valid days=7", enabled)
	}
	if len(invalid) != 1 || invalid[0].TenantID != "tenant-invalid" {
		t.Fatalf("invalid retentions = %#v, want tenant-invalid only", invalid)
	}
}

func newSettingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open settings sqlite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access settings sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&database.Tenant{}, &database.SystemSetting{}, &database.ImageAsset{}); err != nil {
		t.Fatalf("migrate settings test schema: %v", err)
	}
	return db
}

func seedSettingsTenant(t *testing.T, db *gorm.DB, tenantID string, status string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.Tenant{
		ID:        tenantID,
		Name:      tenantID,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant %s: %v", tenantID, err)
	}
}

func seedSettingsAsset(t *testing.T, db *gorm.DB, tenantID string, assetID string, sizeBytes int64, deletedAt *time.Time, purgedAt *time.Time, now time.Time) {
	t.Helper()
	record := database.ImageAsset{
		ID:        assetID,
		TenantID:  tenantID,
		ProjectID: "project-" + tenantID,
		Kind:      "REFERENCE",
		Filename:  assetID + ".png",
		ObjectKey: "settings/" + tenantID + "/" + assetID + ".png",
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
		t.Fatalf("seed asset %s/%s: %v", tenantID, assetID, err)
	}
}

func seedSettingsRow(t *testing.T, db *gorm.DB, tenantID string, key string, valueJSON string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.SystemSetting{
		ID:        "setting-" + tenantID + "-" + key,
		TenantID:  tenantID,
		Key:       key,
		ValueJSON: valueJSON,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed setting %s/%s: %v", tenantID, key, err)
	}
}
