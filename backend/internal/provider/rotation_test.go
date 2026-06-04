package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	rotationOldSecret = "0123456789abcdef0123456789abcdef"
	rotationNewSecret = "abcdef0123456789abcdef0123456789"
	rotationOldKeyID  = "rotation-key-v1"
	rotationNewKeyID  = "rotation-key-v2"
)

func TestAPIKeyRotationServiceDryRunDoesNotWrite(t *testing.T) {
	db := newAPIKeyRotationTestDB(t)
	oldCipher := mustAPIKeyCipher(t, rotationOldSecret, rotationOldKeyID)
	newCipher := mustAPIKeyCipher(t, rotationNewSecret, rotationNewKeyID)
	original := seedAPIKeyRotationProvider(t, db, oldCipher, "tenant-a", "provider-a", "sk-dry-run-marker", false)

	summary, err := NewAPIKeyRotationService(db).Rotate(context.Background(), oldCipher, newCipher, false)
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	if summary != (APIKeyRotationSummary{ProviderCount: 1, Applied: false}) {
		t.Fatalf("summary = %+v", summary)
	}

	stored := loadAPIKeyRotationProvider(t, db, original.ID, false)
	assertProviderCredentialMetadataUnchanged(t, stored, original)
	if stored.EncryptedAPIKey != original.EncryptedAPIKey {
		t.Fatal("dry-run updated encrypted api key")
	}
}

func TestAPIKeyRotationServiceApplyUpdatesAllActiveProvidersOnly(t *testing.T) {
	db := newAPIKeyRotationTestDB(t)
	oldCipher := mustAPIKeyCipher(t, rotationOldSecret, rotationOldKeyID)
	newCipher := mustAPIKeyCipher(t, rotationNewSecret, rotationNewKeyID)
	first := seedAPIKeyRotationProvider(t, db, oldCipher, "tenant-a", "provider-a", "sk-active-one", false)
	second := seedAPIKeyRotationProvider(t, db, oldCipher, "tenant-b", "provider-b", "sk-active-two", false)
	deleted := seedAPIKeyRotationProvider(t, db, oldCipher, "tenant-c", "provider-deleted", "sk-deleted", true)

	summary, err := NewAPIKeyRotationService(db).Rotate(context.Background(), oldCipher, newCipher, true)
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	if summary != (APIKeyRotationSummary{ProviderCount: 2, DeletedProviderEraseCount: 1, Applied: true}) {
		t.Fatalf("summary = %+v", summary)
	}

	for _, expected := range []struct {
		original database.AIProvider
		plain    string
	}{
		{original: first, plain: "sk-active-one"},
		{original: second, plain: "sk-active-two"},
	} {
		stored := loadAPIKeyRotationProvider(t, db, expected.original.ID, false)
		assertProviderCredentialMetadataUnchanged(t, stored, expected.original)
		if stored.EncryptedAPIKey == expected.original.EncryptedAPIKey {
			t.Fatalf("provider %q encrypted api key was not rotated", stored.ID)
		}
		if _, err := oldCipher.Decrypt(stored.EncryptedAPIKey); err == nil {
			t.Fatalf("old cipher decrypted rotated provider %q", stored.ID)
		}
		plain, err := newCipher.Decrypt(stored.EncryptedAPIKey)
		if err != nil {
			t.Fatalf("new cipher decrypt provider %q returned error: %v", stored.ID, err)
		}
		if plain != expected.plain {
			t.Fatalf("new cipher decrypt provider %q = %q", stored.ID, plain)
		}
	}

	storedDeleted := loadAPIKeyRotationProvider(t, db, deleted.ID, true)
	if storedDeleted.EncryptedAPIKey != "" || storedDeleted.APIKeyHint != "" || storedDeleted.APIKeyUpdatedAt != nil {
		t.Fatalf("soft-deleted provider credential metadata was not erased: %#v", storedDeleted)
	}
}

func TestAPIKeyRotationServiceDryRunReportsDeletedProviderEraseCandidatesWithoutWriting(t *testing.T) {
	db := newAPIKeyRotationTestDB(t)
	oldCipher := mustAPIKeyCipher(t, rotationOldSecret, rotationOldKeyID)
	newCipher := mustAPIKeyCipher(t, rotationNewSecret, rotationNewKeyID)
	deleted := seedAPIKeyRotationProvider(t, db, oldCipher, "tenant-a", "provider-deleted", "sk-deleted", true)

	summary, err := NewAPIKeyRotationService(db).Rotate(context.Background(), oldCipher, newCipher, false)
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	if summary != (APIKeyRotationSummary{DeletedProviderEraseCount: 1, Applied: false}) {
		t.Fatalf("summary = %+v", summary)
	}
	stored := loadAPIKeyRotationProvider(t, db, deleted.ID, true)
	assertProviderCredentialMetadataUnchanged(t, stored, deleted)
}

func TestAPIKeyRotationServiceApplyRollsBackEveryWriteWhenAnyProviderFails(t *testing.T) {
	db := newAPIKeyRotationTestDB(t)
	oldCipher := mustAPIKeyCipher(t, rotationOldSecret, rotationOldKeyID)
	newCipher := mustAPIKeyCipher(t, rotationNewSecret, rotationNewKeyID)
	first := seedAPIKeyRotationProvider(t, db, oldCipher, "tenant-a", "provider-a", "sk-valid-before-bad-row", false)
	bad := seedAPIKeyRotationProvider(t, db, oldCipher, "tenant-a", "provider-b", "sk-bad-row-marker", false)
	const sensitiveMarker = "encrypted-payload-marker-that-must-not-leak"
	if err := db.Model(&database.AIProvider{}).Where("id = ?", bad.ID).UpdateColumn("encrypted_api_key", sensitiveMarker).Error; err != nil {
		t.Fatalf("corrupt encrypted api key: %v", err)
	}

	_, err := NewAPIKeyRotationService(db).Rotate(context.Background(), oldCipher, newCipher, true)
	if !errors.Is(err, ErrAPIKeyRotationFailed) {
		t.Fatalf("Rotate error = %v, want ErrAPIKeyRotationFailed", err)
	}
	if strings.Contains(err.Error(), sensitiveMarker) || strings.Contains(err.Error(), "sk-bad-row-marker") {
		t.Fatalf("Rotate error leaked sensitive marker: %v", err)
	}

	storedFirst := loadAPIKeyRotationProvider(t, db, first.ID, false)
	if storedFirst.EncryptedAPIKey != first.EncryptedAPIKey {
		t.Fatal("provider update was not rolled back after later failure")
	}
}

func TestAPIKeyRotationServiceHandlesEmptyTable(t *testing.T) {
	db := newAPIKeyRotationTestDB(t)
	oldCipher := mustAPIKeyCipher(t, rotationOldSecret, rotationOldKeyID)
	newCipher := mustAPIKeyCipher(t, rotationNewSecret, rotationNewKeyID)

	summary, err := NewAPIKeyRotationService(db).Rotate(context.Background(), oldCipher, newCipher, true)
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	if summary != (APIKeyRotationSummary{ProviderCount: 0, Applied: true}) {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestAPIKeyRotationServiceRejectsSameKeyIDWithoutWriting(t *testing.T) {
	db := newAPIKeyRotationTestDB(t)
	oldCipher := mustAPIKeyCipher(t, rotationOldSecret, rotationOldKeyID)
	newCipher := mustAPIKeyCipher(t, rotationNewSecret, rotationOldKeyID)
	original := seedAPIKeyRotationProvider(t, db, oldCipher, "tenant-a", "provider-a", "sk-same-id", false)

	_, err := NewAPIKeyRotationService(db).Rotate(context.Background(), oldCipher, newCipher, true)
	if !errors.Is(err, ErrAPIKeyRotationSameKeyID) {
		t.Fatalf("Rotate error = %v, want ErrAPIKeyRotationSameKeyID", err)
	}
	stored := loadAPIKeyRotationProvider(t, db, original.ID, false)
	if stored.EncryptedAPIKey != original.EncryptedAPIKey {
		t.Fatal("same key id rejection updated encrypted api key")
	}
}

func newAPIKeyRotationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access sqlite database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := db.Exec(`
CREATE TABLE ai_providers (
  id text PRIMARY KEY,
  tenant_id text NOT NULL,
  type text NOT NULL,
  name text NOT NULL,
  base_url text NOT NULL,
  encrypted_api_key text NOT NULL,
  api_key_hint text NOT NULL,
  api_key_updated_at datetime,
  status text NOT NULL,
  timeout_seconds integer NOT NULL,
  concurrency_limit integer NOT NULL,
  last_test_status text,
  last_tested_at datetime,
  last_test_error text,
  created_by text NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime NOT NULL,
  deleted_at datetime
)`).Error; err != nil {
		t.Fatalf("create sqlite provider table: %v", err)
	}
	return db
}

func mustAPIKeyCipher(t *testing.T, secret string, keyID string) APIKeyCipher {
	t.Helper()
	cipher, err := NewAPIKeyCipher(secret, keyID)
	if err != nil {
		t.Fatalf("NewAPIKeyCipher returned error: %v", err)
	}
	return cipher
}

func seedAPIKeyRotationProvider(t *testing.T, db *gorm.DB, cipher APIKeyCipher, tenantID string, providerID string, plainAPIKey string, deleted bool) database.AIProvider {
	t.Helper()

	encrypted, hint, err := cipher.Encrypt(plainAPIKey)
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	now := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC)
	record := database.AIProvider{
		ID:               providerID,
		TenantID:         tenantID,
		Type:             TypeOpenAI,
		Name:             "rotation test provider",
		BaseURL:          "https://provider.invalid/v1",
		EncryptedAPIKey:  encrypted,
		APIKeyHint:       hint,
		APIKeyUpdatedAt:  &now,
		Status:           StatusEnabled,
		TimeoutSeconds:   60,
		ConcurrencyLimit: 1,
		CreatedBy:        "operator-test",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if deleted {
		record.DeletedAt = gorm.DeletedAt{Time: now.Add(time.Hour), Valid: true}
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create provider %q: %v", providerID, err)
	}
	return record
}

func loadAPIKeyRotationProvider(t *testing.T, db *gorm.DB, providerID string, unscoped bool) database.AIProvider {
	t.Helper()

	query := db.Model(&database.AIProvider{})
	if unscoped {
		query = query.Unscoped()
	}
	var record database.AIProvider
	if err := query.Where("id = ?", providerID).First(&record).Error; err != nil {
		t.Fatalf("load provider %q: %v", providerID, err)
	}
	return record
}

func assertProviderCredentialMetadataUnchanged(t *testing.T, stored database.AIProvider, original database.AIProvider) {
	t.Helper()
	if stored.APIKeyHint != original.APIKeyHint {
		t.Fatalf("provider %q api key hint changed: got %q want %q", stored.ID, stored.APIKeyHint, original.APIKeyHint)
	}
	if !stored.APIKeyUpdatedAt.Equal(*original.APIKeyUpdatedAt) {
		t.Fatalf("provider %q api key updated at changed: got %v want %v", stored.ID, stored.APIKeyUpdatedAt, original.APIKeyUpdatedAt)
	}
	if !stored.UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatalf("provider %q updated at changed: got %v want %v", stored.ID, stored.UpdatedAt, original.UpdatedAt)
	}
}
