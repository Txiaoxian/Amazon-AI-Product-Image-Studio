package asset

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestCleanupServicePurgeDeletedAssetsScopesCutoffBatchAndIdempotency(t *testing.T) {
	db := newCleanupTestDB(t)
	store := newCleanupFakeStore()
	storageConfig := config.StorageConfig{
		BucketOriginals:  "originals-test",
		BucketGenerated:  "generated-test",
		BucketThumbnails: "thumbs-test",
	}
	service := NewCleanupService(db, slog.New(slog.NewTextHandler(io.Discard, nil)), storageConfig, store)
	now := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	cutoff := now.Add(-24 * time.Hour)

	seedCleanupAsset(t, db, cleanupSeed{ID: "old-reference", TenantID: "tenant-a", Kind: KindReference, ObjectKey: "original-old-reference", ThumbnailObjectKey: ptr("thumb-old-reference"), DeletedAt: ptrTime(cutoff.Add(-3 * time.Hour)), UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "old-generated", TenantID: "tenant-a", Kind: KindGenerated, ObjectKey: "original-old-generated", DeletedAt: ptrTime(cutoff.Add(-2 * time.Hour)), UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "old-edited", TenantID: "tenant-a", Kind: KindEdited, ObjectKey: "original-old-edited", DeletedAt: ptrTime(cutoff.Add(-1 * time.Hour)), UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "active", TenantID: "tenant-a", Kind: KindReference, ObjectKey: "original-active", UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "new-delete", TenantID: "tenant-a", Kind: KindReference, ObjectKey: "original-new-delete", DeletedAt: ptrTime(cutoff.Add(time.Minute)), UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "already-purged", TenantID: "tenant-a", Kind: KindReference, ObjectKey: "original-already-purged", DeletedAt: ptrTime(cutoff.Add(-4 * time.Hour)), PurgedAt: ptrTime(now.Add(-time.Hour)), UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "tenant-b-old", TenantID: "tenant-b", Kind: KindReference, ObjectKey: "original-tenant-b-old", DeletedAt: ptrTime(cutoff.Add(-5 * time.Hour)), UpdatedAt: now})

	result, err := service.PurgeDeletedAssets(context.Background(), "tenant-a", cutoff, PurgeOptions{BatchLimit: 2})
	if err != nil {
		t.Fatalf("first purge returned error: %v", err)
	}
	if result.Processed != 2 || result.Purged != 2 || result.Failed != 0 {
		t.Fatalf("first purge result = %#v, want processed=2 purged=2 failed=0", result)
	}
	assertRemovedObjects(t, store.removed, []removedObject{
		{bucket: "originals-test", key: "original-old-reference"},
		{bucket: "thumbs-test", key: "thumb-old-reference"},
		{bucket: "generated-test", key: "original-old-generated"},
	})
	assertPurged(t, db, "tenant-a", "old-reference", true)
	assertPurged(t, db, "tenant-a", "old-generated", true)
	assertPurged(t, db, "tenant-a", "old-edited", false)
	assertPurged(t, db, "tenant-a", "active", false)
	assertPurged(t, db, "tenant-a", "new-delete", false)
	assertPurged(t, db, "tenant-b", "tenant-b-old", false)

	result, err = service.PurgeDeletedAssets(context.Background(), "tenant-a", cutoff, PurgeOptions{BatchLimit: 2})
	if err != nil {
		t.Fatalf("second purge returned error: %v", err)
	}
	if result.Processed != 1 || result.Purged != 1 || result.Failed != 0 {
		t.Fatalf("second purge result = %#v, want processed=1 purged=1 failed=0", result)
	}
	assertRemovedObjects(t, store.removed, []removedObject{
		{bucket: "originals-test", key: "original-old-reference"},
		{bucket: "thumbs-test", key: "thumb-old-reference"},
		{bucket: "generated-test", key: "original-old-generated"},
		{bucket: "generated-test", key: "original-old-edited"},
	})
	assertPurged(t, db, "tenant-a", "old-edited", true)

	result, err = service.PurgeDeletedAssets(context.Background(), "tenant-a", cutoff, PurgeOptions{BatchLimit: 2})
	if err != nil {
		t.Fatalf("third purge returned error: %v", err)
	}
	if result.Processed != 0 || len(store.removed) != 4 {
		t.Fatalf("third purge was not idempotent: result=%#v removed=%#v", result, store.removed)
	}
}

func TestCleanupServicePurgeDeletedAssetsTreatsNotFoundAsSuccessAndRetriesStorageErrors(t *testing.T) {
	db := newCleanupTestDB(t)
	store := newCleanupFakeStore()
	storageConfig := config.StorageConfig{
		BucketOriginals:  "originals-test",
		BucketGenerated:  "generated-test",
		BucketThumbnails: "thumbs-test",
	}
	service := NewCleanupService(db, slog.New(slog.NewTextHandler(io.Discard, nil)), storageConfig, store)
	now := time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC)
	cutoff := now

	seedCleanupAsset(t, db, cleanupSeed{ID: "missing-objects", TenantID: "tenant-a", Kind: KindReference, ObjectKey: "missing-original", ThumbnailObjectKey: ptr("missing-thumb"), DeletedAt: ptrTime(cutoff.Add(-time.Hour)), UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "thumbnail-error", TenantID: "tenant-a", Kind: KindReference, ObjectKey: "ok-original", ThumbnailObjectKey: ptr("error-thumb"), DeletedAt: ptrTime(cutoff.Add(-30 * time.Minute)), UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "original-error", TenantID: "tenant-a", Kind: KindGenerated, ObjectKey: "error-original", ThumbnailObjectKey: ptr("should-not-delete-thumb"), DeletedAt: ptrTime(cutoff.Add(-20 * time.Minute)), UpdatedAt: now})

	store.errByObject["originals-test/missing-original"] = storage.ErrNotFound
	store.errByObject["thumbs-test/missing-thumb"] = storage.ErrNotFound
	store.errByObject["thumbs-test/error-thumb"] = errors.New("temporary storage failure with object key")
	store.errByObject["generated-test/error-original"] = errors.New("temporary original failure with object key")

	result, err := service.PurgeDeletedAssets(context.Background(), "tenant-a", cutoff, PurgeOptions{BatchLimit: 10})
	if !errors.Is(err, ErrCleanupFailed) {
		t.Fatalf("first purge error = %v, want ErrCleanupFailed", err)
	}
	if result.Processed != 3 || result.Purged != 1 || result.Failed != 2 {
		t.Fatalf("first purge result = %#v, want processed=3 purged=1 failed=2", result)
	}
	assertPurged(t, db, "tenant-a", "missing-objects", true)
	assertPurged(t, db, "tenant-a", "thumbnail-error", false)
	assertPurged(t, db, "tenant-a", "original-error", false)
	if store.wasRemoved("thumbs-test", "should-not-delete-thumb") {
		t.Fatal("cleanup should not delete thumbnail after original delete failed")
	}

	store.errByObject = map[string]error{}
	result, err = service.PurgeDeletedAssets(context.Background(), "tenant-a", cutoff, PurgeOptions{BatchLimit: 10})
	if err != nil {
		t.Fatalf("retry purge returned error: %v", err)
	}
	if result.Processed != 2 || result.Purged != 2 || result.Failed != 0 {
		t.Fatalf("retry purge result = %#v, want processed=2 purged=2 failed=0", result)
	}
	assertPurged(t, db, "tenant-a", "thumbnail-error", true)
	assertPurged(t, db, "tenant-a", "original-error", true)
}

type cleanupFakeStore struct {
	removed     []removedObject
	errByObject map[string]error
}

type removedObject struct {
	bucket string
	key    string
}

func newCleanupFakeStore() *cleanupFakeStore {
	return &cleanupFakeStore{errByObject: map[string]error{}}
}

func (s *cleanupFakeStore) PutObject(context.Context, string, string, io.Reader, int64, string) error {
	return nil
}

func (s *cleanupFakeStore) GetObject(context.Context, string, string) (storage.Object, error) {
	return storage.Object{Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func (s *cleanupFakeStore) ListObjects(context.Context, storage.ListObjectsInput) (storage.ListObjectsResult, error) {
	return storage.ListObjectsResult{}, nil
}

func (s *cleanupFakeStore) RemoveObject(_ context.Context, bucket string, key string) error {
	s.removed = append(s.removed, removedObject{bucket: bucket, key: key})
	if err := s.errByObject[bucket+"/"+key]; err != nil {
		return err
	}
	return nil
}

func (s *cleanupFakeStore) wasRemoved(bucket string, key string) bool {
	for _, object := range s.removed {
		if object.bucket == bucket && object.key == key {
			return true
		}
	}
	return false
}

func newCleanupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open cleanup sqlite database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access cleanup sqlite database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(`
CREATE TABLE image_assets (
	id TEXT PRIMARY KEY,
	tenant_id TEXT NOT NULL,
	project_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	category TEXT NOT NULL,
	filename TEXT NOT NULL,
	object_key TEXT NOT NULL,
	thumbnail_object_key TEXT NULL,
	mime_type TEXT NOT NULL,
	size_bytes INTEGER NOT NULL,
	width INTEGER NOT NULL,
	height INTEGER NOT NULL,
	sha256 TEXT NOT NULL,
	is_favorite BOOLEAN NOT NULL,
	source_task_id TEXT NULL,
	created_by TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	deleted_at TIMESTAMP NULL,
	purged_at TIMESTAMP NULL
)`).Error; err != nil {
		t.Fatalf("create cleanup image_assets schema: %v", err)
	}
	return db
}

type cleanupSeed struct {
	ID                 string
	TenantID           string
	Kind               string
	ObjectKey          string
	ThumbnailObjectKey *string
	DeletedAt          *time.Time
	PurgedAt           *time.Time
	UpdatedAt          time.Time
}

func seedCleanupAsset(t *testing.T, db *gorm.DB, seed cleanupSeed) {
	t.Helper()
	if err := db.Exec(`
INSERT INTO image_assets (
	id, tenant_id, project_id, kind, category, filename, object_key, thumbnail_object_key, mime_type,
	size_bytes, width, height, sha256, is_favorite, source_task_id, created_by,
	created_at, updated_at, deleted_at, purged_at
) VALUES (?, ?, ?, ?, '', '', ?, ?, 'image/png', 1, 1, 1, ?, false, NULL, 'user-a', ?, ?, ?, ?)`,
		seed.ID,
		seed.TenantID,
		"project-a",
		seed.Kind,
		seed.ObjectKey,
		seed.ThumbnailObjectKey,
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		seed.UpdatedAt,
		seed.UpdatedAt,
		seed.DeletedAt,
		seed.PurgedAt,
	).Error; err != nil {
		t.Fatalf("seed cleanup asset %s: %v", seed.ID, err)
	}
}

func assertPurged(t *testing.T, db *gorm.DB, tenantID string, assetID string, want bool) {
	t.Helper()
	var purgedAt sql.NullTime
	if err := db.Table("image_assets").Select("purged_at").Where("tenant_id = ? AND id = ?", tenantID, assetID).Scan(&purgedAt).Error; err != nil {
		t.Fatalf("load purged_at for %s/%s: %v", tenantID, assetID, err)
	}
	if purgedAt.Valid != want {
		t.Fatalf("purged_at set for %s/%s = %v, want %v", tenantID, assetID, purgedAt.Valid, want)
	}
}

func assertRemovedObjects(t *testing.T, got []removedObject, want []removedObject) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("removed objects = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("removed object %d = %#v, want %#v; all=%#v", i, got[i], want[i], got)
		}
	}
}

func ptr(value string) *string {
	return &value
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
