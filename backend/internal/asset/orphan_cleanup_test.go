package asset

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"gorm.io/gorm"
)

func TestInspectStorageOrphansRequiresPatternTenantAgeAndMetadataExclusion(t *testing.T) {
	db := newCleanupTestDB(t)
	store := newOrphanFakeStore()
	storageConfig := config.StorageConfig{BucketOriginals: "originals-test", BucketGenerated: "generated-test", BucketThumbnails: "thumbs-test"}
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	referencedOriginal := "tenants/tenant-a/projects/project-a/assets/asset-referenced/original.png"
	referencedThumbnail := "tenants/tenant-a/projects/project-a/assets/asset-thumb/thumbnail.jpg"

	store.add("originals-test", "tenants/tenant-a/projects/project-a/assets/asset-orphan/original.png", old)
	store.add("generated-test", "tenants/tenant-a/projects/project-a/assets/asset-generated/original.webp", old)
	store.add("thumbs-test", "tenants/tenant-a/projects/project-a/assets/asset-thumb-orphan/thumbnail.jpg", old)
	store.add("originals-test", "tenants/tenant-a/projects/project-a/assets/asset-new/original.png", now.Add(-30*time.Minute))
	store.add("originals-test", "tenants/tenant-a/projects/project-a/assets/asset-mismatch/thumbnail.jpg", old)
	store.add("originals-test", "tenants/tenant-a/projects/project-a/assets/asset-unknown/preview.png", old)
	store.add("originals-test", referencedOriginal, old)
	store.add("thumbs-test", referencedThumbnail, old)
	seedCleanupAsset(t, db, cleanupSeed{ID: "asset-referenced", TenantID: "tenant-a", Kind: KindReference, ObjectKey: referencedOriginal, UpdatedAt: now})
	seedCleanupAsset(t, db, cleanupSeed{ID: "asset-thumb", TenantID: "tenant-a", Kind: KindReference, ObjectKey: "tenants/tenant-a/projects/project-a/assets/asset-thumb/original.png", ThumbnailObjectKey: &referencedThumbnail, UpdatedAt: now})

	service := newOrphanTestService(db, store, storageConfig, now)
	response, err := service.inspectStorageOrphans(context.Background(), auth.Principal{TenantID: "tenant-a", UserID: "admin-a"}, orphanOperationOptions{
		DryRun:           true,
		BatchLimit:       20,
		GracePeriodHours: 24,
	})
	if err != nil {
		t.Fatalf("inspectStorageOrphans: %v", err)
	}

	if response.Candidates != 3 || response.Deleted != 0 {
		t.Fatalf("candidates/deleted = %d/%d, want 3/0; response=%#v", response.Candidates, response.Deleted, response)
	}
	for _, key := range []string{"metadata_referenced", "too_new", "bucket_pattern_mismatch", "unrecognized_pattern"} {
		if response.Skipped[key] == 0 {
			t.Fatalf("missing skipped category %s in %#v", key, response.Skipped)
		}
	}
	if len(response.CandidateIDs) != 3 {
		t.Fatalf("candidate ids = %#v, want 3 opaque ids", response.CandidateIDs)
	}
	for _, candidateID := range response.CandidateIDs {
		if candidateID == "" || len(candidateID) < len("sha256:")+64 {
			t.Fatalf("candidate id is not an opaque hash: %q", candidateID)
		}
	}
	if len(store.removed) != 0 {
		t.Fatalf("dry-run removed objects: %#v", store.removed)
	}
}

func TestInspectStorageOrphansCleanupIsBatchLimitedAndRetrySafe(t *testing.T) {
	db := newCleanupTestDB(t)
	store := newOrphanFakeStore()
	storageConfig := config.StorageConfig{BucketOriginals: "originals-test", BucketGenerated: "generated-test", BucketThumbnails: "thumbs-test"}
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	deletedKey := "tenants/tenant-a/projects/project-a/assets/asset-a-delete/original.png"
	missingKey := "tenants/tenant-a/projects/project-a/assets/asset-b-missing/original.png"
	failedKey := "tenants/tenant-a/projects/project-a/assets/asset-c-fail/original.png"

	store.add("originals-test", deletedKey, old)
	store.add("originals-test", missingKey, old)
	store.add("originals-test", failedKey, old)
	seedCleanupQuotaCounter(t, db, "tenant-a", 7, 3, now)
	store.removeErrs["originals-test/"+missingKey] = storage.ErrNotFound
	store.removeErrs["originals-test/"+failedKey] = storage.ErrUnavailable
	service := newOrphanTestService(db, store, storageConfig, now)

	response, err := service.inspectStorageOrphans(context.Background(), auth.Principal{TenantID: "tenant-a", UserID: "admin-a"}, orphanOperationOptions{
		DryRun:           false,
		BatchLimit:       2,
		GracePeriodHours: 24,
	})
	if err != nil {
		t.Fatalf("cleanup inspectStorageOrphans: %v", err)
	}

	if response.Scanned != 2 || response.Candidates != 2 || response.Deleted != 2 {
		t.Fatalf("scanned/candidates/deleted = %d/%d/%d, want 2/2/2; response=%#v", response.Scanned, response.Candidates, response.Deleted, response)
	}
	if !response.HasMore {
		t.Fatal("batch-limited cleanup should report hasMore")
	}
	if store.exists("originals-test", deletedKey) {
		t.Fatal("eligible object was not deleted")
	}
	if !store.exists("originals-test", failedKey) {
		t.Fatal("unscanned object should remain for a later batch")
	}
	assertCleanupQuotaCounter(t, db, "tenant-a", 7, 3)

	response, err = service.inspectStorageOrphans(context.Background(), auth.Principal{TenantID: "tenant-a", UserID: "admin-a"}, orphanOperationOptions{
		DryRun:           false,
		BatchLimit:       10,
		GracePeriodHours: 24,
	})
	if err != nil {
		t.Fatalf("retry inspectStorageOrphans: %v", err)
	}
	if response.Errors["storage_unavailable"] != 1 || response.Deleted != 0 {
		t.Fatalf("retry errors/deleted = %#v/%d, want storage_unavailable once and no deletes", response.Errors, response.Deleted)
	}
	if !store.exists("originals-test", failedKey) {
		t.Fatal("failed delete should remain retryable")
	}
	assertCleanupQuotaCounter(t, db, "tenant-a", 7, 3)
}

func TestInspectStorageOrphansDefensivelyCapsOverLimitListings(t *testing.T) {
	db := newCleanupTestDB(t)
	store := newOrphanFakeStore()
	store.ignoreLimit = true
	storageConfig := config.StorageConfig{BucketOriginals: "originals-test", BucketGenerated: "generated-test", BucketThumbnails: "thumbs-test"}
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	for _, assetID := range []string{"asset-a", "asset-b", "asset-c"} {
		store.add("originals-test", "tenants/tenant-a/projects/project-a/assets/"+assetID+"/original.png", old)
	}
	service := newOrphanTestService(db, store, storageConfig, now)

	response, err := service.inspectStorageOrphans(context.Background(), auth.Principal{TenantID: "tenant-a", UserID: "admin-a"}, orphanOperationOptions{
		DryRun:           false,
		BatchLimit:       2,
		GracePeriodHours: 24,
	})
	if err != nil {
		t.Fatalf("inspectStorageOrphans with over-limit listing: %v", err)
	}
	if response.Scanned != 2 || response.Candidates != 2 || response.Deleted != 2 {
		t.Fatalf("scanned/candidates/deleted = %d/%d/%d, want 2/2/2", response.Scanned, response.Candidates, response.Deleted)
	}
	if !response.HasMore || response.NextCursor == "" {
		t.Fatalf("missing continuation cursor for capped listing: %#v", response)
	}
	if len(store.removed) != 2 {
		t.Fatalf("removed objects = %#v, want exactly 2", store.removed)
	}
}

func TestInspectStorageOrphansCursorContinuesPastSkippedPage(t *testing.T) {
	db := newCleanupTestDB(t)
	store := newOrphanFakeStore()
	storageConfig := config.StorageConfig{BucketOriginals: "originals-test", BucketGenerated: "generated-test", BucketThumbnails: "thumbs-test"}
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	referencedKey := "tenants/tenant-a/projects/project-a/assets/asset-a-referenced/original.png"
	candidateKey := "tenants/tenant-a/projects/project-a/assets/asset-b-orphan/original.png"
	store.add("originals-test", referencedKey, old)
	store.add("originals-test", candidateKey, old)
	seedCleanupAsset(t, db, cleanupSeed{ID: "asset-a-referenced", TenantID: "tenant-a", Kind: KindReference, ObjectKey: referencedKey, UpdatedAt: now})
	service := newOrphanTestService(db, store, storageConfig, now)

	first, err := service.inspectStorageOrphans(context.Background(), auth.Principal{TenantID: "tenant-a", UserID: "admin-a"}, orphanOperationOptions{
		DryRun:           true,
		BatchLimit:       1,
		GracePeriodHours: 24,
	})
	if err != nil {
		t.Fatalf("first inspectStorageOrphans: %v", err)
	}
	if first.Candidates != 0 || first.Skipped["metadata_referenced"] != 1 || first.NextCursor == "" {
		t.Fatalf("first page = %#v, want skipped page with cursor", first)
	}
	if strings.Contains(first.NextCursor, "tenants/") || strings.Contains(first.NextCursor, "originals-test") || strings.Contains(first.NextCursor, candidateKey) {
		t.Fatalf("cursor leaks raw storage details: %q", first.NextCursor)
	}

	second, err := service.inspectStorageOrphans(context.Background(), auth.Principal{TenantID: "tenant-a", UserID: "admin-a"}, orphanOperationOptions{
		DryRun:           true,
		BatchLimit:       1,
		GracePeriodHours: 24,
		Cursor:           first.NextCursor,
	})
	if err != nil {
		t.Fatalf("second inspectStorageOrphans: %v", err)
	}
	if second.Candidates != 1 || len(second.CandidateIDs) != 1 {
		t.Fatalf("second page = %#v, want later candidate", second)
	}
}

func TestInspectStorageOrphansDeduplicatesSameBucketOriginalClasses(t *testing.T) {
	db := newCleanupTestDB(t)
	store := newOrphanFakeStore()
	storageConfig := config.StorageConfig{BucketOriginals: "shared-assets", BucketGenerated: "shared-assets", BucketThumbnails: "thumbs-test"}
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	store.add("shared-assets", "tenants/tenant-a/projects/project-a/assets/asset-orphan/original.png", old)
	service := newOrphanTestService(db, store, storageConfig, now)

	response, err := service.inspectStorageOrphans(context.Background(), auth.Principal{TenantID: "tenant-a", UserID: "admin-a"}, orphanOperationOptions{
		DryRun:           true,
		BatchLimit:       10,
		GracePeriodHours: 24,
	})
	if err != nil {
		t.Fatalf("inspectStorageOrphans same-bucket: %v", err)
	}
	if response.Candidates != 1 || response.Skipped["duplicate_listing"] != 1 {
		t.Fatalf("same-bucket response = %#v, want one candidate and one duplicate skip", response)
	}
}

type orphanFakeStore struct {
	objects     map[string]storage.ListedObject
	removed     []removedObject
	removeErrs  map[string]error
	ignoreLimit bool
}

func newOrphanFakeStore() *orphanFakeStore {
	return &orphanFakeStore{
		objects:    map[string]storage.ListedObject{},
		removeErrs: map[string]error{},
	}
}

func seedCleanupQuotaCounter(t *testing.T, db *gorm.DB, tenantID string, usedBytes int64, reservedBytes int64, now time.Time) {
	t.Helper()
	if err := db.Exec(`
INSERT INTO storage_quota_counters (id, tenant_id, used_bytes, reserved_bytes, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		"counter-"+tenantID,
		tenantID,
		usedBytes,
		reservedBytes,
		now,
		now,
	).Error; err != nil {
		t.Fatalf("seed cleanup quota counter: %v", err)
	}
}

func (s *orphanFakeStore) PutObject(context.Context, string, string, io.Reader, int64, string) error {
	return nil
}

func (s *orphanFakeStore) GetObject(context.Context, string, string) (storage.Object, error) {
	return storage.Object{Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func (s *orphanFakeStore) ListObjects(_ context.Context, input storage.ListObjectsInput) (storage.ListObjectsResult, error) {
	keys := make([]string, 0, len(s.objects))
	for stored, object := range s.objects {
		prefix := input.Bucket + "/"
		if !strings.HasPrefix(stored, prefix) {
			continue
		}
		if input.Prefix != "" && !strings.HasPrefix(object.Key, input.Prefix) {
			continue
		}
		if input.Cursor != "" && object.Key <= input.Cursor {
			continue
		}
		keys = append(keys, object.Key)
	}
	sort.Strings(keys)
	hasMore := input.Limit > 0 && len(keys) > input.Limit
	if hasMore && !s.ignoreLimit {
		keys = keys[:input.Limit]
	}

	objects := make([]storage.ListedObject, 0, len(keys))
	for _, key := range keys {
		objects = append(objects, s.objects[input.Bucket+"/"+key])
	}
	result := storage.ListObjectsResult{Objects: objects}
	if hasMore && len(objects) > 0 {
		if s.ignoreLimit && input.Limit > 0 {
			result.NextCursor = objects[input.Limit-1].Key
			return result, nil
		}
		result.NextCursor = objects[len(objects)-1].Key
	}
	return result, nil
}

func (s *orphanFakeStore) RemoveObject(_ context.Context, bucket string, key string) error {
	s.removed = append(s.removed, removedObject{bucket: bucket, key: key})
	if err := s.removeErrs[bucket+"/"+key]; err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			delete(s.objects, bucket+"/"+key)
		}
		return err
	}
	delete(s.objects, bucket+"/"+key)
	return nil
}

func (s *orphanFakeStore) add(bucket string, key string, lastModified time.Time) {
	s.objects[bucket+"/"+key] = storage.ListedObject{Key: key, LastModified: lastModified.UTC()}
}

func (s *orphanFakeStore) exists(bucket string, key string) bool {
	_, ok := s.objects[bucket+"/"+key]
	return ok
}

func newOrphanTestService(gormDB *gorm.DB, store storage.ObjectStore, storageConfig config.StorageConfig, now time.Time) *Service {
	return &Service{
		db:            gormDB,
		repo:          NewRepository(gormDB),
		store:         store,
		storageConfig: storageConfig,
		log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		now: func() time.Time {
			return now
		},
	}
}
