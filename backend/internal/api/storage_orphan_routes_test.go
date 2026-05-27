package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"gorm.io/gorm"
)

func TestStorageOrphanRoutesDryRunCleanupAndRedaction(t *testing.T) {
	router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	storageConfig := config.DefaultStorageConfig()
	old := time.Now().UTC().Add(-48 * time.Hour)
	candidateKey := "tenants/" + adminSession.tenantID + "/projects/project-a/assets/asset-orphan/original.png"
	referencedKey := "tenants/" + adminSession.tenantID + "/projects/project-a/assets/asset-referenced/original.png"
	tooNewKey := "tenants/" + adminSession.tenantID + "/projects/project-a/assets/asset-new/original.png"
	unknownKey := "tenants/" + adminSession.tenantID + "/projects/project-a/assets/asset-unknown/sidecar.txt"
	store.putListedObject(storageConfig.BucketOriginals, candidateKey, old)
	store.putListedObject(storageConfig.BucketOriginals, referencedKey, old)
	store.putListedObject(storageConfig.BucketOriginals, tooNewKey, time.Now().UTC())
	store.putListedObject(storageConfig.BucketOriginals, unknownKey, old)
	seedStorageOrphanAsset(t, db, adminSession.tenantID, "asset-referenced", referencedKey, nil)

	rejectedScan := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/scan", map[string]any{
		"dryRun": false,
	}, adminSession.cookies, adminSession.csrfHeader())
	if rejectedScan.Code != http.StatusUnprocessableEntity {
		t.Fatalf("scan dryRun=false status = %d, want %d", rejectedScan.Code, http.StatusUnprocessableEntity)
	}

	scan := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/scan", map[string]any{
		"batchLimit":       10,
		"gracePeriodHours": 24,
	}, adminSession.cookies, adminSession.csrfHeader())
	if scan.Code != http.StatusOK {
		t.Fatalf("scan status = %d, want %d: %s", scan.Code, http.StatusOK, scan.Body.String())
	}
	scanData := decodeData(t, scan)
	if scanData["dryRun"] != true || scanData["candidates"].(float64) != 1 || scanData["deleted"].(float64) != 0 {
		t.Fatalf("scan data = %#v, want dry-run one candidate and no delete", scanData)
	}
	if !store.has(storageConfig.BucketOriginals, candidateKey) {
		t.Fatal("dry-run scan deleted candidate")
	}
	assertResponseExcludes(t, scan.Body.String(), candidateKey, referencedKey, tooNewKey, unknownKey, storageConfig.BucketOriginals, "tenants/", "objectKey", "bucket", "minio", "http://")

	defaultCleanup := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/cleanup", nil, adminSession.cookies, adminSession.csrfHeader())
	if defaultCleanup.Code != http.StatusOK {
		t.Fatalf("default cleanup status = %d, want %d: %s", defaultCleanup.Code, http.StatusOK, defaultCleanup.Body.String())
	}
	defaultCleanupData := decodeData(t, defaultCleanup)
	if defaultCleanupData["dryRun"] != true || defaultCleanupData["deleted"].(float64) != 0 {
		t.Fatalf("default cleanup data = %#v, want dry-run and no delete", defaultCleanupData)
	}
	if !store.has(storageConfig.BucketOriginals, candidateKey) {
		t.Fatal("default dry-run cleanup deleted candidate")
	}

	rejectedCleanup := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/cleanup", map[string]any{
		"dryRun":  false,
		"confirm": "wrong",
	}, adminSession.cookies, adminSession.csrfHeader())
	if rejectedCleanup.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cleanup without confirmation status = %d, want %d", rejectedCleanup.Code, http.StatusUnprocessableEntity)
	}
	if !store.has(storageConfig.BucketOriginals, candidateKey) {
		t.Fatal("cleanup without confirmation deleted candidate")
	}

	cleanup := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/cleanup", map[string]any{
		"dryRun":           false,
		"confirm":          "DELETE_ORPHANS",
		"batchLimit":       10,
		"gracePeriodHours": 24,
	}, adminSession.cookies, adminSession.csrfHeader())
	if cleanup.Code != http.StatusOK {
		t.Fatalf("cleanup status = %d, want %d: %s", cleanup.Code, http.StatusOK, cleanup.Body.String())
	}
	cleanupData := decodeData(t, cleanup)
	if cleanupData["dryRun"] != false || cleanupData["candidates"].(float64) != 1 || cleanupData["deleted"].(float64) != 1 {
		t.Fatalf("cleanup data = %#v, want one deleted candidate", cleanupData)
	}
	if store.has(storageConfig.BucketOriginals, candidateKey) {
		t.Fatal("confirmed cleanup did not delete candidate")
	}
	if !store.has(storageConfig.BucketOriginals, referencedKey) {
		t.Fatal("cleanup deleted metadata-referenced object")
	}
	if !store.has(storageConfig.BucketOriginals, tooNewKey) || !store.has(storageConfig.BucketOriginals, unknownKey) {
		t.Fatal("cleanup deleted too-new or unrecognized object")
	}
	assertResponseExcludes(t, cleanup.Body.String(), candidateKey, referencedKey, tooNewKey, unknownKey, storageConfig.BucketOriginals, "tenants/", "objectKey", "bucket", "minio", "http://")
	assertStorageOrphanOperationLogRedacted(t, db, candidateKey, referencedKey, storageConfig.BucketOriginals)
}

func TestStorageOrphanRoutesRequireTenantAdminOrSettingsManage(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	seedActiveUser(t, db, adminSession.tenantID, "seller-orphan", "seller-orphan@example.com", "Seller Orphan", "seller-orphan-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-orphan", "seller")
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-orphan@example.com", "seller-orphan-password-123")

	sellerScan := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/scan", nil, sellerSession.cookies, sellerSession.csrfHeader())
	if sellerScan.Code != http.StatusForbidden {
		t.Fatalf("seller scan status = %d, want %d", sellerScan.Code, http.StatusForbidden)
	}

	seedActiveUser(t, db, adminSession.tenantID, "settings-manager-orphan", "settings-manager-orphan@example.com", "Settings Manager Orphan", "settings-manager-password-123")
	assignRoleWithPermissions(t, db, adminSession.tenantID, "settings-manager-orphan", "settings-manager-orphan", []string{"system:settings:manage"})
	managerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "settings-manager-orphan@example.com", "settings-manager-password-123")

	managerScan := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/scan", nil, managerSession.cookies, managerSession.csrfHeader())
	if managerScan.Code != http.StatusOK {
		t.Fatalf("settings manager scan status = %d, want %d: %s", managerScan.Code, http.StatusOK, managerScan.Body.String())
	}
}

func TestStorageOrphanRoutesOpaqueCursorContinuesPastSkippedPage(t *testing.T) {
	router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	storageConfig := config.DefaultStorageConfig()
	old := time.Now().UTC().Add(-48 * time.Hour)
	referencedKey := "tenants/" + adminSession.tenantID + "/projects/project-a/assets/asset-a-referenced/original.png"
	candidateKey := "tenants/" + adminSession.tenantID + "/projects/project-a/assets/asset-b-orphan/original.png"
	store.putListedObject(storageConfig.BucketOriginals, referencedKey, old)
	store.putListedObject(storageConfig.BucketOriginals, candidateKey, old)
	seedStorageOrphanAsset(t, db, adminSession.tenantID, "asset-a-referenced", referencedKey, nil)

	first := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/scan", map[string]any{
		"batchLimit":       1,
		"gracePeriodHours": 24,
	}, adminSession.cookies, adminSession.csrfHeader())
	if first.Code != http.StatusOK {
		t.Fatalf("first scan status = %d, want %d: %s", first.Code, http.StatusOK, first.Body.String())
	}
	firstData := decodeData(t, first)
	cursor := stringField(t, firstData, "nextCursor")
	if firstData["candidates"].(float64) != 0 || cursor == "" {
		t.Fatalf("first scan data = %#v, want skipped page with cursor", firstData)
	}
	assertResponseExcludes(t, first.Body.String(), referencedKey, candidateKey, storageConfig.BucketOriginals, "tenants/", "minio", "http://")

	second := performJSON(router, http.MethodPost, "/api/v1/admin/storage/orphans/scan", map[string]any{
		"batchLimit":       1,
		"gracePeriodHours": 24,
		"cursor":           cursor,
	}, adminSession.cookies, adminSession.csrfHeader())
	if second.Code != http.StatusOK {
		t.Fatalf("second scan status = %d, want %d: %s", second.Code, http.StatusOK, second.Body.String())
	}
	secondData := decodeData(t, second)
	if secondData["candidates"].(float64) != 1 {
		t.Fatalf("second scan data = %#v, want later candidate", secondData)
	}
	assertResponseExcludes(t, second.Body.String(), referencedKey, candidateKey, storageConfig.BucketOriginals, "tenants/", "minio", "http://")
}

func seedStorageOrphanAsset(t *testing.T, db *gorm.DB, tenantID string, assetID string, objectKey string, thumbnailObjectKey *string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&database.ImageAsset{
		ID:                 assetID,
		TenantID:           tenantID,
		ProjectID:          "project-a",
		Kind:               asset.KindReference,
		Category:           "",
		Filename:           "asset.png",
		ObjectKey:          objectKey,
		ThumbnailObjectKey: thumbnailObjectKey,
		MimeType:           "image/png",
		SizeBytes:          1,
		Width:              1,
		Height:             1,
		SHA256:             "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		IsFavorite:         false,
		CreatedBy:          "user-a",
		CreatedAt:          now,
		UpdatedAt:          now,
	}).Error; err != nil {
		t.Fatalf("seed storage orphan asset %s: %v", assetID, err)
	}
}

func assertStorageOrphanOperationLogRedacted(t *testing.T, db *gorm.DB, forbidden ...string) {
	t.Helper()
	var logs []database.OperationLog
	if err := db.Where("action = ?", "storage.orphan_cleanup").Find(&logs).Error; err != nil {
		t.Fatalf("load storage orphan operation logs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("missing storage.orphan_cleanup operation log")
	}
	for _, log := range logs {
		metadata := strings.ToLower(log.MetadataJSON)
		for _, marker := range append(forbidden, "tenants/", "objectkey", "bucket", "minio", "http://", "authorization", "cookie", "jwt", "api_key", "base64") {
			if strings.Contains(metadata, strings.ToLower(marker)) {
				t.Fatalf("storage orphan operation log metadata contains %q: %#v", marker, log)
			}
		}
	}
}
