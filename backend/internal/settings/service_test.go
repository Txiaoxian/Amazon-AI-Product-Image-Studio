package settings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	modelpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	providerpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
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

func TestLoadLogRetentionUsesNullableTenantScopedValue(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-b", "ACTIVE", now)
	seedSettingsRow(t, db, "tenant-b", KeyLogRetention, `{"operationLogRetentionDays":30,"apiCallLogRetentionDays":null,"taskEventRetentionDays":7}`, now)

	tenantA, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant A scope: %v", err)
	}
	retention, err := LoadLogRetention(context.Background(), repo, tenantA)
	if err != nil {
		t.Fatalf("load tenant A log retention: %v", err)
	}
	if retention.OperationLogRetentionDays != nil || retention.APICallLogRetentionDays != nil || retention.TaskEventRetentionDays != nil {
		t.Fatalf("tenant A log retention = %#v, want nil fallback", retention)
	}

	tenantB, err := tenant.NewScope("tenant-b")
	if err != nil {
		t.Fatalf("tenant B scope: %v", err)
	}
	retention, err = LoadLogRetention(context.Background(), repo, tenantB)
	if err != nil {
		t.Fatalf("load tenant B log retention: %v", err)
	}
	if retention.OperationLogRetentionDays == nil || *retention.OperationLogRetentionDays != 30 {
		t.Fatalf("tenant B operationLogRetentionDays = %#v, want 30", retention.OperationLogRetentionDays)
	}
	if retention.APICallLogRetentionDays != nil {
		t.Fatalf("tenant B apiCallLogRetentionDays = %#v, want nil", retention.APICallLogRetentionDays)
	}
	if retention.TaskEventRetentionDays == nil || *retention.TaskEventRetentionDays != 7 {
		t.Fatalf("tenant B taskEventRetentionDays = %#v, want 7", retention.TaskEventRetentionDays)
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

func TestListActiveTenantIDsIsBoundedAndSkipsInactiveTenants(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-b", "DISABLED", now)
	seedSettingsTenant(t, db, "tenant-c", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-d", "ACTIVE", now)

	tenantIDs, err := repo.ListActiveTenantIDs(context.Background(), 2)
	if err != nil {
		t.Fatalf("ListActiveTenantIDs returned error: %v", err)
	}
	if len(tenantIDs) != 2 || tenantIDs[0] != "tenant-a" || tenantIDs[1] != "tenant-c" {
		t.Fatalf("active tenant ids = %#v, want tenant-a and tenant-c", tenantIDs)
	}

	tenantIDs, err = repo.ListActiveTenantIDsAfter(context.Background(), "tenant-c", 2)
	if err != nil {
		t.Fatalf("ListActiveTenantIDsAfter returned error: %v", err)
	}
	if len(tenantIDs) != 1 || tenantIDs[0] != "tenant-d" {
		t.Fatalf("active tenant ids after tenant-c = %#v, want tenant-d", tenantIDs)
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

func TestLoadTaskDefaultsRejectsUnavailableStoredProviderModel(t *testing.T) {
	cases := []struct {
		name           string
		providerStatus string
		modelStatus    string
		deleteProvider bool
		deleteModel    bool
		crossTenant    bool
	}{
		{name: "disabled provider", providerStatus: providerpkg.StatusDisabled, modelStatus: modelpkg.StatusEnabled},
		{name: "deleted provider", providerStatus: providerpkg.StatusEnabled, modelStatus: modelpkg.StatusEnabled, deleteProvider: true},
		{name: "disabled model", providerStatus: providerpkg.StatusEnabled, modelStatus: modelpkg.StatusDisabled},
		{name: "deleted model", providerStatus: providerpkg.StatusEnabled, modelStatus: modelpkg.StatusEnabled, deleteModel: true},
		{name: "model belongs to another tenant", providerStatus: providerpkg.StatusEnabled, modelStatus: modelpkg.StatusEnabled, crossTenant: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newSettingsTestDB(t)
			repo := NewRepository(db)
			now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
			seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
			seedSettingsTenant(t, db, "tenant-b", "ACTIVE", now)
			providerTenantID := "tenant-a"
			modelTenantID := "tenant-a"
			if tc.crossTenant {
				modelTenantID = "tenant-b"
			}
			seedSettingsProvider(t, db, providerTenantID, "provider-default", tc.providerStatus, now)
			seedSettingsModel(t, db, modelTenantID, "provider-default", "model-default", tc.modelStatus, now)
			if tc.deleteProvider {
				if err := db.Where("tenant_id = ? AND id = ?", "tenant-a", "provider-default").Delete(&database.AIProvider{}).Error; err != nil {
					t.Fatalf("delete provider: %v", err)
				}
			}
			if tc.deleteModel {
				if err := db.Where("tenant_id = ? AND id = ?", "tenant-a", "model-default").Delete(&database.AIModel{}).Error; err != nil {
					t.Fatalf("delete model: %v", err)
				}
			}
			seedSettingsRow(t, db, "tenant-a", KeyTaskDefaults, `{"defaultProviderId":"provider-default","defaultModelId":"model-default"}`, now)
			scope, err := tenant.NewScope("tenant-a")
			if err != nil {
				t.Fatalf("tenant scope: %v", err)
			}

			_, err = LoadTaskDefaults(context.Background(), repo, scope)
			if !errors.Is(err, ErrStoredTaskDefaultsInvalid) {
				t.Fatalf("LoadTaskDefaults error = %v, want ErrStoredTaskDefaultsInvalid", err)
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

func TestStorageQuotaReservationLifecycleFinalizesAndReleasesCounter(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsRow(t, db, "tenant-a", KeyStorageQuota, `{"maxBytes":150}`, now)
	seedSettingsAsset(t, db, "tenant-a", "asset-active", 20, nil, nil, now)
	scope, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant scope: %v", err)
	}

	reservation, err := ReserveStorageQuota(context.Background(), repo, scope, 30)
	if err != nil {
		t.Fatalf("ReserveStorageQuota: %v", err)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 20, 30)
	if err := FinalizeStorageQuotaReservation(context.Background(), repo, scope, reservation, 30); err != nil {
		t.Fatalf("FinalizeStorageQuotaReservation: %v", err)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 50, 0)
	used, err := repo.StorageUsedBytes(context.Background(), scope)
	if err != nil {
		t.Fatalf("StorageUsedBytes: %v", err)
	}
	if used != 50 {
		t.Fatalf("used bytes = %d, want 50", used)
	}

	released, err := ReserveStorageQuota(context.Background(), repo, scope, 40)
	if err != nil {
		t.Fatalf("second ReserveStorageQuota: %v", err)
	}
	if err := ReleaseStorageQuotaReservation(context.Background(), repo, scope, released); err != nil {
		t.Fatalf("ReleaseStorageQuotaReservation: %v", err)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 50, 0)
}

func TestStorageQuotaFinalizeFailsClosedForReleasedReservationAndIsIdempotentForMatchingFinalized(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsRow(t, db, "tenant-a", KeyStorageQuota, `{"maxBytes":150}`, now)
	scope, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant scope: %v", err)
	}

	finalized, err := ReserveStorageQuota(context.Background(), repo, scope, 30)
	if err != nil {
		t.Fatalf("reserve finalized candidate: %v", err)
	}
	if err := FinalizeStorageQuotaReservation(context.Background(), repo, scope, finalized, 30); err != nil {
		t.Fatalf("finalize reservation: %v", err)
	}
	if err := FinalizeStorageQuotaReservation(context.Background(), repo, scope, finalized, 30); err != nil {
		t.Fatalf("idempotent finalize error = %v, want nil", err)
	}
	if err := FinalizeStorageQuotaReservation(context.Background(), repo, scope, finalized, 20); !errors.Is(err, ErrStorageQuotaReservationInvalid) {
		t.Fatalf("mismatched idempotent finalize error = %v, want ErrStorageQuotaReservationInvalid", err)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 30, 0)

	released, err := ReserveStorageQuota(context.Background(), repo, scope, 40)
	if err != nil {
		t.Fatalf("reserve released candidate: %v", err)
	}
	if err := ReleaseStorageQuotaReservation(context.Background(), repo, scope, released); err != nil {
		t.Fatalf("release reservation: %v", err)
	}
	if err := FinalizeStorageQuotaReservation(context.Background(), repo, scope, released, 40); !errors.Is(err, ErrStorageQuotaReservationInvalid) {
		t.Fatalf("released finalize error = %v, want ErrStorageQuotaReservationInvalid", err)
	}
	if err := FinalizeStorageQuotaReservation(context.Background(), repo, scope, released, 0); err != nil {
		t.Fatalf("idempotent released finalize zero error = %v, want nil", err)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 30, 0)
}

func TestStorageQuotaReservationCountsReservedBytesForOverlappingUploads(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsRow(t, db, "tenant-a", KeyStorageQuota, `{"maxBytes":100}`, now)
	scope, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant scope: %v", err)
	}

	first, err := ReserveStorageQuota(context.Background(), repo, scope, 60)
	if err != nil {
		t.Fatalf("first ReserveStorageQuota: %v", err)
	}
	if _, err := ReserveStorageQuota(context.Background(), repo, scope, 41); !errors.Is(err, ErrStorageQuotaExceeded) {
		t.Fatalf("second ReserveStorageQuota error = %v, want ErrStorageQuotaExceeded", err)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 0, 60)
	if err := ReleaseStorageQuotaReservation(context.Background(), repo, scope, first); err != nil {
		t.Fatalf("release first reservation: %v", err)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 0, 0)
}

func TestStorageQuotaReservationConcurrentUploadsAllowOnlyOneWhenCombinedExceedsQuota(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsRow(t, db, "tenant-a", KeyStorageQuota, `{"maxBytes":100}`, now)
	scope, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant scope: %v", err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	reservations := make(chan StorageQuotaReservation, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			reservation, err := ReserveStorageQuota(context.Background(), repo, scope, 60)
			if err == nil {
				reservations <- reservation
			}
			errs <- err
		}()
	}
	close(start)

	var success int
	var exceeded int
	for i := 0; i < 2; i++ {
		err := <-errs
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrStorageQuotaExceeded):
			exceeded++
		default:
			t.Fatalf("concurrent reserve error = %v", err)
		}
	}
	if success != 1 || exceeded != 1 {
		t.Fatalf("concurrent reserve success/exceeded = %d/%d, want 1/1", success, exceeded)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 0, 60)
	close(reservations)
	for reservation := range reservations {
		if err := ReleaseStorageQuotaReservation(context.Background(), repo, scope, reservation); err != nil {
			t.Fatalf("release successful reservation: %v", err)
		}
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 0, 0)
}

func TestStorageQuotaMalformedCounterFailsClosed(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsRow(t, db, "tenant-a", KeyStorageQuota, `{"maxBytes":100}`, now)
	if err := db.Create(&database.StorageQuotaCounter{
		ID:            "counter-malformed",
		TenantID:      "tenant-a",
		UsedBytes:     -1,
		ReservedBytes: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("seed malformed counter: %v", err)
	}
	scope, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant scope: %v", err)
	}

	if _, err := repo.StorageUsedBytes(context.Background(), scope); !errors.Is(err, ErrStorageQuotaCounterInvalid) {
		t.Fatalf("StorageUsedBytes error = %v, want ErrStorageQuotaCounterInvalid", err)
	}
	if _, err := ReserveStorageQuota(context.Background(), repo, scope, 1); !errors.Is(err, ErrStorageQuotaCounterInvalid) {
		t.Fatalf("ReserveStorageQuota error = %v, want ErrStorageQuotaCounterInvalid", err)
	}
}

func TestReconcileStorageQuotaCounterUsesMetadataTruthAndReleasesStaleReservations(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	purgedAt := now
	seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
	seedSettingsAsset(t, db, "tenant-a", "asset-active", 100, nil, nil, now)
	seedSettingsAsset(t, db, "tenant-a", "asset-soft", 50, &now, nil, now)
	seedSettingsAsset(t, db, "tenant-a", "asset-purged", 200, &now, &purgedAt, now)
	if err := db.Create(&database.StorageQuotaCounter{
		ID:            "counter-stale",
		TenantID:      "tenant-a",
		UsedBytes:     1,
		ReservedBytes: 70,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("seed stale counter: %v", err)
	}
	seedSettingsReservation(t, db, "stale-reservation", "tenant-a", 30, storageQuotaReservationStatusReserved, time.Now().UTC().Add(-time.Hour), now)
	seedSettingsReservation(t, db, "active-reservation", "tenant-a", 40, storageQuotaReservationStatusReserved, time.Now().UTC().Add(time.Hour), now)
	scope, err := tenant.NewScope("tenant-a")
	if err != nil {
		t.Fatalf("tenant scope: %v", err)
	}

	if err := ReconcileStorageQuotaCounter(context.Background(), repo, scope); err != nil {
		t.Fatalf("ReconcileStorageQuotaCounter: %v", err)
	}
	assertSettingsQuotaCounter(t, db, "tenant-a", 150, 40)
	var status string
	if err := db.Model(&database.StorageQuotaReservation{}).
		Select("status").
		Where("tenant_id = ? AND id = ?", "tenant-a", "stale-reservation").
		Scan(&status).Error; err != nil {
		t.Fatalf("load stale reservation status: %v", err)
	}
	if status != storageQuotaReservationStatusReleased {
		t.Fatalf("stale reservation status = %q, want RELEASED", status)
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

func TestLoadLogRetentionRejectsMalformedStoredValues(t *testing.T) {
	tests := []struct {
		name      string
		valueJSON string
	}{
		{name: "invalid json", valueJSON: `{`},
		{name: "unknown field", valueJSON: `{"operationLogRetentionDays":30,"storageQuotaBytes":1}`},
		{name: "operation zero", valueJSON: `{"operationLogRetentionDays":0,"apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`},
		{name: "api negative", valueJSON: `{"operationLogRetentionDays":null,"apiCallLogRetentionDays":-1,"taskEventRetentionDays":null}`},
		{name: "task over range", valueJSON: `{"operationLogRetentionDays":null,"apiCallLogRetentionDays":null,"taskEventRetentionDays":3651}`},
		{name: "string", valueJSON: `{"operationLogRetentionDays":"30","apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`},
		{name: "float", valueJSON: `{"operationLogRetentionDays":1.5,"apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newSettingsTestDB(t)
			repo := NewRepository(db)
			now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
			seedSettingsTenant(t, db, "tenant-a", "ACTIVE", now)
			seedSettingsRow(t, db, "tenant-a", KeyLogRetention, tc.valueJSON, now)
			scope, err := tenant.NewScope("tenant-a")
			if err != nil {
				t.Fatalf("tenant scope: %v", err)
			}

			_, err = LoadLogRetention(context.Background(), repo, scope)
			if !errors.Is(err, ErrStoredLogRetentionInvalid) {
				t.Fatalf("LoadLogRetention error = %v, want ErrStoredLogRetentionInvalid", err)
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

func TestLoadEnabledLogRetentionsSkipsNullInvalidAndInactiveTenants(t *testing.T) {
	db := newSettingsTestDB(t)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	seedSettingsTenant(t, db, "tenant-valid", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-null", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-invalid", "ACTIVE", now)
	seedSettingsTenant(t, db, "tenant-inactive", "DISABLED", now)
	seedSettingsRow(t, db, "tenant-valid", KeyLogRetention, `{"operationLogRetentionDays":7,"apiCallLogRetentionDays":null,"taskEventRetentionDays":3}`, now)
	seedSettingsRow(t, db, "tenant-null", KeyLogRetention, `{"operationLogRetentionDays":null,"apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`, now)
	seedSettingsRow(t, db, "tenant-invalid", KeyLogRetention, `{"operationLogRetentionDays":0,"apiCallLogRetentionDays":null,"taskEventRetentionDays":null}`, now)
	seedSettingsRow(t, db, "tenant-inactive", KeyLogRetention, `{"operationLogRetentionDays":1,"apiCallLogRetentionDays":1,"taskEventRetentionDays":1}`, now)

	enabled, invalid, err := LoadEnabledLogRetentions(context.Background(), repo)
	if err != nil {
		t.Fatalf("LoadEnabledLogRetentions returned error: %v", err)
	}
	if len(enabled) != 1 || enabled[0].TenantID != "tenant-valid" {
		t.Fatalf("enabled retentions = %#v, want tenant-valid only", enabled)
	}
	if enabled[0].OperationLogRetentionDays == nil || *enabled[0].OperationLogRetentionDays != 7 {
		t.Fatalf("operation days = %#v, want 7", enabled[0].OperationLogRetentionDays)
	}
	if enabled[0].APICallLogRetentionDays != nil {
		t.Fatalf("api call days = %#v, want nil", enabled[0].APICallLogRetentionDays)
	}
	if enabled[0].TaskEventRetentionDays == nil || *enabled[0].TaskEventRetentionDays != 3 {
		t.Fatalf("task event days = %#v, want 3", enabled[0].TaskEventRetentionDays)
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
	if err := db.AutoMigrate(&database.Tenant{}, &database.SystemSetting{}, &database.StorageQuotaCounter{}, &database.StorageQuotaReservation{}, &database.ImageAsset{}, &database.AIProvider{}, &database.AIModel{}); err != nil {
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

func seedSettingsReservation(t *testing.T, db *gorm.DB, id string, tenantID string, bytes int64, status string, expiresAt time.Time, now time.Time) {
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
		t.Fatalf("seed reservation %s: %v", id, err)
	}
}

func assertSettingsQuotaCounter(t *testing.T, db *gorm.DB, tenantID string, usedBytes int64, reservedBytes int64) {
	t.Helper()
	var counter database.StorageQuotaCounter
	if err := db.Model(&database.StorageQuotaCounter{}).
		Select("tenant_id, used_bytes, reserved_bytes").
		Where("tenant_id = ?", tenantID).
		First(&counter).Error; err != nil {
		t.Fatalf("load quota counter: %v", err)
	}
	if counter.UsedBytes != usedBytes || counter.ReservedBytes != reservedBytes {
		t.Fatalf("quota counter used/reserved = %d/%d, want %d/%d", counter.UsedBytes, counter.ReservedBytes, usedBytes, reservedBytes)
	}
}

func seedSettingsProvider(t *testing.T, db *gorm.DB, tenantID string, providerID string, status string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.AIProvider{
		ID:               providerID,
		TenantID:         tenantID,
		Type:             providerpkg.TypeOpenAICompatible,
		Name:             "Provider " + providerID,
		BaseURL:          "https://api.openai.com/v1",
		EncryptedAPIKey:  "encrypted",
		APIKeyHint:       "****test",
		Status:           status,
		TimeoutSeconds:   10,
		ConcurrencyLimit: 1,
		CreatedBy:        "seed",
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("seed settings provider %s/%s: %v", tenantID, providerID, err)
	}
}

func seedSettingsModel(t *testing.T, db *gorm.DB, tenantID string, providerID string, modelID string, status string, now time.Time) {
	t.Helper()
	if err := db.Create(&database.AIModel{
		ID:                         modelID,
		TenantID:                   tenantID,
		ProviderID:                 providerID,
		ModelName:                  "model-default",
		DisplayName:                "Model Default",
		SupportsGenerate:           true,
		SupportsEdit:               true,
		SupportsMultiReference:     false,
		SupportsN:                  false,
		MaxOutputCount:             1,
		SupportedSizesJSON:         `["1024x1024"]`,
		SupportedQualitiesJSON:     `["standard"]`,
		SupportedOutputFormatsJSON: `["png"]`,
		PricingJSON:                `{"currency":"USD","unitPrices":{}}`,
		Status:                     status,
		CreatedBy:                  "seed",
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}).Error; err != nil {
		t.Fatalf("seed settings model %s/%s: %v", tenantID, modelID, err)
	}
}
