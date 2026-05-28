package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"gorm.io/gorm"
)

func TestAssetRoutesUploadListUpdateFavoriteDownloadDeleteAndAudit(t *testing.T) {
	router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	projectID := createAssetTestProject(t, router, adminSession, "Asset Project")

	seedActiveUser(t, db, adminSession.tenantID, "editor-user", "editor@example.com", "Editor User", "editor-password-123")
	assignRole(t, db, adminSession.tenantID, "editor-user", "seller")
	addMember(t, router, adminSession, projectID, "editor-user", project.RoleEditor)
	editorSession := loginProjectRouteUser(t, router, adminSession.tenantID, "editor@example.com", "editor-password-123")

	imageBytes := validPNG(t, 2, 2)
	uploadResponse := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "../hero.png", "image/png", imageBytes, map[string]string{
		"kind":       asset.KindReference,
		"category":   "reference",
		"filename":   "../display.png",
		"isFavorite": "true",
	}, editorSession.cookies, editorSession.csrfHeader())
	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d: %s", uploadResponse.Code, http.StatusCreated, uploadResponse.Body.String())
	}
	uploadData := decodeData(t, uploadResponse)
	assetID := stringField(t, uploadData, "id")
	if stringField(t, uploadData, "kind") != asset.KindReference {
		t.Fatalf("asset kind = %q, want REFERENCE", stringField(t, uploadData, "kind"))
	}
	if stringField(t, uploadData, "filename") != "display.png" {
		t.Fatalf("filename = %q, want display.png", stringField(t, uploadData, "filename"))
	}
	if stringField(t, uploadData, "previewUrl") != "/api/v1/assets/"+assetID+"/download" {
		t.Fatalf("previewUrl = %q", stringField(t, uploadData, "previewUrl"))
	}
	if stringField(t, uploadData, "thumbnailUrl") != "/api/v1/assets/"+assetID+"/thumbnail" {
		t.Fatalf("thumbnailUrl = %q", stringField(t, uploadData, "thumbnailUrl"))
	}

	var record database.ImageAsset
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, assetID).First(&record).Error; err != nil {
		t.Fatalf("load image asset metadata: %v", err)
	}
	expectedObjectKey := "tenants/" + adminSession.tenantID + "/projects/" + projectID + "/assets/" + assetID + "/original.png"
	if record.ObjectKey != expectedObjectKey {
		t.Fatalf("object key = %q, want %q", record.ObjectKey, expectedObjectKey)
	}
	expectedThumbnailKey := "tenants/" + adminSession.tenantID + "/projects/" + projectID + "/assets/" + assetID + "/thumbnail.jpg"
	if record.ThumbnailObjectKey == nil || *record.ThumbnailObjectKey != expectedThumbnailKey {
		t.Fatalf("thumbnail object key = %#v, want %q", record.ThumbnailObjectKey, expectedThumbnailKey)
	}
	if record.MimeType != "image/png" || record.SizeBytes != int64(len(imageBytes)) || record.Width != 2 || record.Height != 2 || record.SHA256 == "" {
		t.Fatalf("stored metadata is incomplete: %#v", record)
	}
	if !record.IsFavorite {
		t.Fatal("upload should persist is_favorite metadata")
	}
	if !store.has(config.DefaultStorageConfig().BucketOriginals, record.ObjectKey) {
		t.Fatal("uploaded object was not written to object storage")
	}
	if !store.has(config.DefaultStorageConfig().BucketThumbnails, *record.ThumbnailObjectKey) {
		t.Fatal("uploaded thumbnail was not written to thumbnails storage")
	}

	listResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/assets?kind=REFERENCE&category=reference&favorite=true&pageNum=1&pageSize=10", nil, editorSession.cookies, nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d: %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	listData := decodeData(t, listResponse)
	if total, ok := listData["total"].(float64); !ok || total != 1 {
		t.Fatalf("asset list total = %#v, want 1", listData["total"])
	}
	listRecord := recordsField(t, listData)[0].(map[string]any)
	if stringField(t, listRecord, "thumbnailUrl") != "/api/v1/assets/"+assetID+"/thumbnail" {
		t.Fatalf("list thumbnailUrl = %q", stringField(t, listRecord, "thumbnailUrl"))
	}

	detailResponse := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID, nil, editorSession.cookies, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d: %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}
	if stringField(t, decodeData(t, detailResponse), "thumbnailUrl") != "/api/v1/assets/"+assetID+"/thumbnail" {
		t.Fatalf("detail thumbnailUrl = %q", stringField(t, decodeData(t, detailResponse), "thumbnailUrl"))
	}

	updateResponse := performJSON(router, http.MethodPatch, "/api/v1/assets/"+assetID, map[string]any{
		"category":   "hero",
		"filename":   "safe-name.png",
		"isFavorite": false,
		"objectKey":  "client/must/not/win",
	}, editorSession.cookies, editorSession.csrfHeader())
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d: %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}
	if stringField(t, decodeData(t, updateResponse), "category") != "hero" {
		t.Fatalf("updated category response = %s", updateResponse.Body.String())
	}
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, assetID).First(&record).Error; err != nil {
		t.Fatalf("reload updated asset: %v", err)
	}
	if record.ObjectKey != expectedObjectKey {
		t.Fatal("PATCH must not modify object_key")
	}

	favoriteResponse := performJSON(router, http.MethodPost, "/api/v1/assets/"+assetID+"/favorite", nil, editorSession.cookies, editorSession.csrfHeader())
	if favoriteResponse.Code != http.StatusOK {
		t.Fatalf("favorite status = %d, want %d: %s", favoriteResponse.Code, http.StatusOK, favoriteResponse.Body.String())
	}
	unfavoriteResponse := performJSON(router, http.MethodDelete, "/api/v1/assets/"+assetID+"/favorite", nil, editorSession.cookies, editorSession.csrfHeader())
	if unfavoriteResponse.Code != http.StatusOK {
		t.Fatalf("unfavorite status = %d, want %d: %s", unfavoriteResponse.Code, http.StatusOK, unfavoriteResponse.Body.String())
	}

	downloadResponse := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID+"/download", nil, editorSession.cookies, nil)
	if downloadResponse.Code != http.StatusOK {
		t.Fatalf("download status = %d, want %d: %s", downloadResponse.Code, http.StatusOK, downloadResponse.Body.String())
	}
	if downloadResponse.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("download content type = %q, want image/png", downloadResponse.Header().Get("Content-Type"))
	}
	if !bytes.Equal(downloadResponse.Body.Bytes(), imageBytes) {
		t.Fatal("downloaded bytes do not match uploaded object")
	}
	thumbnailResponse := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID+"/thumbnail", nil, editorSession.cookies, nil)
	if thumbnailResponse.Code != http.StatusOK {
		t.Fatalf("thumbnail status = %d, want %d: %s", thumbnailResponse.Code, http.StatusOK, thumbnailResponse.Body.String())
	}
	if thumbnailResponse.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("thumbnail content type = %q, want image/jpeg", thumbnailResponse.Header().Get("Content-Type"))
	}
	if _, err := jpeg.Decode(bytes.NewReader(thumbnailResponse.Body.Bytes())); err != nil {
		t.Fatalf("thumbnail body is not a JPEG: %v", err)
	}

	editorDeleteResponse := performJSON(router, http.MethodDelete, "/api/v1/assets/"+assetID, nil, editorSession.cookies, editorSession.csrfHeader())
	if editorDeleteResponse.Code != http.StatusForbidden {
		t.Fatalf("editor delete status = %d, want %d", editorDeleteResponse.Code, http.StatusForbidden)
	}
	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/assets/"+assetID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d: %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	deletedDetail := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID, nil, adminSession.cookies, nil)
	if deletedDetail.Code != http.StatusNotFound {
		t.Fatalf("deleted detail status = %d, want %d", deletedDetail.Code, http.StatusNotFound)
	}
	deletedList := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/assets", nil, adminSession.cookies, nil)
	if deletedList.Code != http.StatusOK {
		t.Fatalf("deleted list status = %d, want %d: %s", deletedList.Code, http.StatusOK, deletedList.Body.String())
	}
	if total, ok := decodeData(t, deletedList)["total"].(float64); !ok || total != 0 {
		t.Fatalf("deleted asset list total = %#v, want 0", decodeData(t, deletedList)["total"])
	}
	deletedDownload := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID+"/download", nil, adminSession.cookies, nil)
	if deletedDownload.Code != http.StatusNotFound {
		t.Fatalf("deleted download status = %d, want %d", deletedDownload.Code, http.StatusNotFound)
	}
	deletedThumbnail := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID+"/thumbnail", nil, adminSession.cookies, nil)
	if deletedThumbnail.Code != http.StatusNotFound {
		t.Fatalf("deleted thumbnail status = %d, want %d", deletedThumbnail.Code, http.StatusNotFound)
	}

	assertAssetOperationLogs(t, db, []string{
		"asset.upload",
		"asset.update",
		"asset.favorite",
		"asset.unfavorite",
		"asset.download",
		"asset.delete",
	})
}

func TestAssetUploadEnforcesTenantStorageQuotaAndDoesNotLeakObject(t *testing.T) {
	router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{MaxFileSizeBytes: 1024 * 1024, MaxWidth: 10, MaxHeight: 10, MaxPixels: 100, AllowedMIMETypes: []string{"image/png"}})
	projectID := createAssetTestProject(t, router, adminSession, "Quota Project")
	imageBytes := validPNG(t, 2, 2)
	now := time.Now().UTC()
	purgedAt := now
	seedQuotaAsset(t, db, adminSession.tenantID, "quota-active-upload", 10, nil, nil)
	seedQuotaAsset(t, db, adminSession.tenantID, "quota-soft-upload", 20, &now, nil)
	seedQuotaAsset(t, db, adminSession.tenantID, "quota-purged-upload", 1000, &now, &purgedAt)
	seedQuotaAsset(t, db, "tenant-other-upload", "quota-cross-upload", 1000, nil, nil)

	setQuota := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageQuota": map[string]any{"maxBytes": int64(len(imageBytes)) + 29},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setQuota.Code != http.StatusOK {
		t.Fatalf("set storageQuota status = %d, want %d: %s", setQuota.Code, http.StatusOK, setQuota.Body.String())
	}

	response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "asset.png", "image/png", imageBytes, nil, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusConflict {
		t.Fatalf("quota exceeded upload status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "STORAGE_QUOTA_EXCEEDED") {
		t.Fatalf("quota response missing stable error code: %s", response.Body.String())
	}
	if store.count() != 0 {
		t.Fatalf("quota failed upload stored objects = %d, want 0", store.count())
	}
	var newRows int64
	if err := db.Model(&database.ImageAsset{}).Where("tenant_id = ? AND project_id = ?", adminSession.tenantID, projectID).Count(&newRows).Error; err != nil {
		t.Fatalf("count quota project assets: %v", err)
	}
	if newRows != 0 {
		t.Fatalf("quota failed upload created %d project asset rows, want 0", newRows)
	}
	var uploadLogs int64
	if err := db.Model(&database.OperationLog{}).Where("tenant_id = ? AND action = ?", adminSession.tenantID, "asset.upload").Count(&uploadLogs).Error; err != nil {
		t.Fatalf("count upload logs: %v", err)
	}
	if uploadLogs != 0 {
		t.Fatalf("quota failed upload wrote %d success logs, want 0", uploadLogs)
	}
	assertQuotaCounter(t, db, adminSession.tenantID, 30, 0)

	setWithinQuota := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageQuota": map[string]any{"maxBytes": int64(len(imageBytes)) + 30},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setWithinQuota.Code != http.StatusOK {
		t.Fatalf("set within quota storageQuota status = %d, want %d: %s", setWithinQuota.Code, http.StatusOK, setWithinQuota.Body.String())
	}
	withinQuota := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "asset-within.png", "image/png", imageBytes, nil, adminSession.cookies, adminSession.csrfHeader())
	if withinQuota.Code != http.StatusCreated {
		t.Fatalf("within quota upload status = %d, want %d: %s", withinQuota.Code, http.StatusCreated, withinQuota.Body.String())
	}
	if store.count() != 2 {
		t.Fatalf("within quota upload stored objects = %d, want 2", store.count())
	}
	assertQuotaCounter(t, db, adminSession.tenantID, 30+int64(len(imageBytes)), 0)

	clearQuota := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageQuota": map[string]any{"maxBytes": nil},
	}, adminSession.cookies, adminSession.csrfHeader())
	if clearQuota.Code != http.StatusOK {
		t.Fatalf("clear storageQuota status = %d, want %d: %s", clearQuota.Code, http.StatusOK, clearQuota.Body.String())
	}
	success := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "asset.png", "image/png", imageBytes, nil, adminSession.cookies, adminSession.csrfHeader())
	if success.Code != http.StatusCreated {
		t.Fatalf("upload after clearing quota status = %d, want %d: %s", success.Code, http.StatusCreated, success.Body.String())
	}
	if store.count() != 4 {
		t.Fatalf("upload after clearing quota stored objects = %d, want 4", store.count())
	}
	assertQuotaCounter(t, db, adminSession.tenantID, 30+int64(len(imageBytes))*2, 0)
}

func TestAssetUploadConcurrentQuotaReservationAllowsOnlyOneCombinedOverLimit(t *testing.T) {
	router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{MaxFileSizeBytes: 1024 * 1024, MaxWidth: 10, MaxHeight: 10, MaxPixels: 100, AllowedMIMETypes: []string{"image/png"}})
	projectID := createAssetTestProject(t, router, adminSession, "Concurrent Quota Project")
	imageBytes := validPNG(t, 2, 2)
	now := time.Now().UTC()
	seedQuotaAsset(t, db, adminSession.tenantID, "quota-active-concurrent", 10, nil, nil)
	seedQuotaAsset(t, db, adminSession.tenantID, "quota-soft-concurrent", 20, &now, nil)

	setQuota := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageQuota": map[string]any{"maxBytes": int64(len(imageBytes)) + 30},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setQuota.Code != http.StatusOK {
		t.Fatalf("set storageQuota status = %d, want %d: %s", setQuota.Code, http.StatusOK, setQuota.Body.String())
	}

	start := make(chan struct{})
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			<-start
			response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", fmt.Sprintf("asset-%d.png", i), "image/png", imageBytes, nil, adminSession.cookies, adminSession.csrfHeader())
			statuses <- response.Code
		}()
	}
	close(start)

	var created int
	var exceeded int
	for i := 0; i < 2; i++ {
		switch code := <-statuses; code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			exceeded++
		default:
			t.Fatalf("concurrent upload status = %d, want created or conflict", code)
		}
	}
	if created != 1 || exceeded != 1 {
		t.Fatalf("concurrent upload created/conflict = %d/%d, want 1/1", created, exceeded)
	}
	var projectAssets int64
	if err := db.Model(&database.ImageAsset{}).Where("tenant_id = ? AND project_id = ?", adminSession.tenantID, projectID).Count(&projectAssets).Error; err != nil {
		t.Fatalf("count concurrent upload assets: %v", err)
	}
	if projectAssets != 1 {
		t.Fatalf("concurrent upload asset rows = %d, want 1", projectAssets)
	}
	if store.count() != 2 {
		t.Fatalf("concurrent upload stored objects = %d, want 2", store.count())
	}
	assertQuotaCounter(t, db, adminSession.tenantID, 30+int64(len(imageBytes)), 0)
}

func TestAssetUploadFailsClosedWhenReservationReleasedBeforeMetadataCommit(t *testing.T) {
	router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	projectID := createAssetTestProject(t, router, adminSession, "Released Reservation Project")
	imageBytes := validPNG(t, 2, 2)
	setQuota := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageQuota": map[string]any{"maxBytes": int64(len(imageBytes)) * 2},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setQuota.Code != http.StatusOK {
		t.Fatalf("set storageQuota status = %d, want %d: %s", setQuota.Code, http.StatusOK, setQuota.Body.String())
	}
	var once sync.Once
	store.onPut = func() {
		once.Do(func() {
			releaseReservedQuotaRowsForTest(t, db, adminSession.tenantID)
		})
	}

	response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "asset.png", "image/png", imageBytes, nil, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("upload status = %d, want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
	assertNoAssetRows(t, db, adminSession.tenantID)
	if store.count() != 0 {
		t.Fatalf("released reservation failure left stored objects: %#v", store.objects)
	}
	assertQuotaCounter(t, db, adminSession.tenantID, 0, 0)
}

func TestAssetRoutesAuthorizeTenantRBACAndProjectMembership(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	projectID := createAssetTestProject(t, router, adminSession, "Permission Project")
	assetID := uploadAssetForTest(t, router, adminSession, projectID)

	seedActiveUser(t, db, adminSession.tenantID, "viewer-user", "viewer@example.com", "Viewer User", "viewer-password-123")
	assignRole(t, db, adminSession.tenantID, "viewer-user", "viewer")
	addMember(t, router, adminSession, projectID, "viewer-user", project.RoleViewer)
	viewerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "viewer@example.com", "viewer-password-123")

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "detail", method: http.MethodGet, path: "/api/v1/assets/" + assetID},
		{name: "download", method: http.MethodGet, path: "/api/v1/assets/" + assetID + "/download"},
		{name: "thumbnail", method: http.MethodGet, path: "/api/v1/assets/" + assetID + "/thumbnail"},
	} {
		response := performJSON(router, tc.method, tc.path, tc.body, viewerSession.cookies, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("viewer %s status = %d, want %d: %s", tc.name, response.Code, http.StatusOK, response.Body.String())
		}
	}

	uploadResponse := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "viewer.png", "image/png", validPNG(t, 2, 2), nil, viewerSession.cookies, viewerSession.csrfHeader())
	if uploadResponse.Code != http.StatusForbidden {
		t.Fatalf("viewer upload status = %d, want %d", uploadResponse.Code, http.StatusForbidden)
	}
	updateResponse := performJSON(router, http.MethodPatch, "/api/v1/assets/"+assetID, map[string]any{"category": "blocked"}, viewerSession.cookies, viewerSession.csrfHeader())
	if updateResponse.Code != http.StatusForbidden {
		t.Fatalf("viewer update status = %d, want %d", updateResponse.Code, http.StatusForbidden)
	}
	favoriteResponse := performJSON(router, http.MethodPost, "/api/v1/assets/"+assetID+"/favorite", nil, viewerSession.cookies, viewerSession.csrfHeader())
	if favoriteResponse.Code != http.StatusForbidden {
		t.Fatalf("viewer favorite status = %d, want %d", favoriteResponse.Code, http.StatusForbidden)
	}
	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/assets/"+assetID, nil, viewerSession.cookies, viewerSession.csrfHeader())
	if deleteResponse.Code != http.StatusForbidden {
		t.Fatalf("viewer delete status = %d, want %d", deleteResponse.Code, http.StatusForbidden)
	}

	seedActiveUser(t, db, adminSession.tenantID, "nonmember-user", "nonmember@example.com", "Nonmember User", "nonmember-password-123")
	assignRole(t, db, adminSession.tenantID, "nonmember-user", "seller")
	nonMemberSession := loginProjectRouteUser(t, router, adminSession.tenantID, "nonmember@example.com", "nonmember-password-123")
	nonMemberDetail := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID, nil, nonMemberSession.cookies, nil)
	if nonMemberDetail.Code != http.StatusForbidden {
		t.Fatalf("non-member detail status = %d, want %d", nonMemberDetail.Code, http.StatusForbidden)
	}
	nonMemberThumbnail := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID+"/thumbnail", nil, nonMemberSession.cookies, nil)
	if nonMemberThumbnail.Code != http.StatusForbidden {
		t.Fatalf("non-member thumbnail status = %d, want %d", nonMemberThumbnail.Code, http.StatusForbidden)
	}

	seedActiveUser(t, db, adminSession.tenantID, "read-only-asset-user", "read-only-asset@example.com", "Read Only Asset", "read-only-asset-password-123")
	assignRole(t, db, adminSession.tenantID, "read-only-asset-user", "viewer")
	removeRolePermission(t, db, adminSession.tenantID, "viewer", "asset:download")
	addMember(t, router, adminSession, projectID, "read-only-asset-user", project.RoleViewer)
	readOnlySession := loginProjectRouteUser(t, router, adminSession.tenantID, "read-only-asset@example.com", "read-only-asset-password-123")
	readOnlyDetail := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID, nil, readOnlySession.cookies, nil)
	if readOnlyDetail.Code != http.StatusOK {
		t.Fatalf("read-only detail status = %d, want %d: %s", readOnlyDetail.Code, http.StatusOK, readOnlyDetail.Body.String())
	}
	readOnlyThumbnail := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID+"/thumbnail", nil, readOnlySession.cookies, nil)
	if readOnlyThumbnail.Code != http.StatusOK {
		t.Fatalf("read-only thumbnail status = %d, want %d: %s", readOnlyThumbnail.Code, http.StatusOK, readOnlyThumbnail.Body.String())
	}
	readOnlyDownload := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID+"/download", nil, readOnlySession.cookies, nil)
	if readOnlyDownload.Code != http.StatusForbidden {
		t.Fatalf("read-only download status = %d, want %d: %s", readOnlyDownload.Code, http.StatusForbidden, readOnlyDownload.Body.String())
	}

	seedOtherTenantProject(t, db)
	now := time.Now().UTC()
	if err := db.Create(&database.ImageAsset{
		ID:        "asset-tenant-b",
		TenantID:  "tenant-b",
		ProjectID: "project-tenant-b",
		Kind:      asset.KindReference,
		ObjectKey: "tenants/tenant-b/projects/project-tenant-b/assets/asset-tenant-b/original.png",
		MimeType:  "image/png",
		SizeBytes: 1,
		Width:     1,
		Height:    1,
		SHA256:    strings.Repeat("0", 64),
		CreatedBy: "user-tenant-b",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B asset: %v", err)
	}
	crossTenantResponse := performJSON(router, http.MethodGet, "/api/v1/assets/asset-tenant-b", nil, adminSession.cookies, nil)
	if crossTenantResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant asset status = %d, want %d", crossTenantResponse.Code, http.StatusNotFound)
	}
	crossTenantThumbnail := performJSON(router, http.MethodGet, "/api/v1/assets/asset-tenant-b/thumbnail", nil, adminSession.cookies, nil)
	if crossTenantThumbnail.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant thumbnail status = %d, want %d", crossTenantThumbnail.Code, http.StatusNotFound)
	}
}

func TestAssetRoutesCrossTenantObjectActionsAreInvisibleAndSideEffectFree(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	seedOtherTenantProject(t, db)
	now := time.Now().UTC()
	record := database.ImageAsset{
		ID:        "asset-tenant-b",
		TenantID:  "tenant-b",
		ProjectID: "project-tenant-b",
		Kind:      asset.KindReference,
		Category:  "reference",
		Filename:  "tenant-b.png",
		ObjectKey: "tenants/tenant-b/projects/project-tenant-b/assets/asset-tenant-b/original.png",
		MimeType:  "image/png",
		SizeBytes: 1,
		Width:     1,
		Height:    1,
		SHA256:    strings.Repeat("0", 64),
		CreatedBy: "user-tenant-b",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed tenant B asset: %v", err)
	}

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "detail", method: http.MethodGet, path: "/api/v1/assets/asset-tenant-b"},
		{name: "download", method: http.MethodGet, path: "/api/v1/assets/asset-tenant-b/download"},
		{name: "thumbnail", method: http.MethodGet, path: "/api/v1/assets/asset-tenant-b/thumbnail"},
		{name: "update", method: http.MethodPatch, path: "/api/v1/assets/asset-tenant-b", body: map[string]string{"category": "stolen"}},
		{name: "favorite", method: http.MethodPost, path: "/api/v1/assets/asset-tenant-b/favorite"},
		{name: "unfavorite", method: http.MethodDelete, path: "/api/v1/assets/asset-tenant-b/favorite"},
		{name: "delete", method: http.MethodDelete, path: "/api/v1/assets/asset-tenant-b"},
	} {
		response := performJSON(router, tc.method, tc.path, tc.body, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s cross-tenant status = %d, want %d: %s", tc.name, response.Code, http.StatusNotFound, response.Body.String())
		}
		var tenantBAsset database.ImageAsset
		if err := db.Unscoped().Where("tenant_id = ? AND id = ?", "tenant-b", "asset-tenant-b").First(&tenantBAsset).Error; err != nil {
			t.Fatalf("reload tenant B asset after %s: %v", tc.name, err)
		}
		if tenantBAsset.DeletedAt.Valid || tenantBAsset.Category != "reference" || tenantBAsset.IsFavorite {
			t.Fatalf("%s changed cross-tenant asset: %#v", tc.name, tenantBAsset)
		}
	}
}

func TestAssetUploadValidationRejectsInvalidFilesAndAvoidsOrphans(t *testing.T) {
	cases := []struct {
		name        string
		upload      config.UploadConfig
		contentType string
		body        []byte
	}{
		{name: "forged MIME", contentType: "image/jpeg", body: validPNG(t, 2, 2)},
		{name: "invalid magic", contentType: "image/png", body: []byte("not an image")},
		{name: "svg forbidden", contentType: "image/svg+xml", body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
		{name: "file too large", upload: config.UploadConfig{MaxFileSizeBytes: 8, MaxWidth: 8192, MaxHeight: 8192, MaxPixels: 40000000, AllowedMIMETypes: []string{"image/png"}}, contentType: "image/png", body: validPNG(t, 2, 2)},
		{name: "dimensions too large", upload: config.UploadConfig{MaxFileSizeBytes: 1024 * 1024, MaxWidth: 1, MaxHeight: 8192, MaxPixels: 40000000, AllowedMIMETypes: []string{"image/png"}}, contentType: "image/png", body: validPNG(t, 2, 2)},
		{name: "pixels too large", upload: config.UploadConfig{MaxFileSizeBytes: 1024 * 1024, MaxWidth: 8192, MaxHeight: 8192, MaxPixels: 1, AllowedMIMETypes: []string{"image/png"}}, contentType: "image/png", body: validPNG(t, 2, 2)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db, store, adminSession := newAssetRouteTestRouter(t, tc.upload)
			projectID := createAssetTestProject(t, router, adminSession, "Validation Project")
			response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "bad.png", tc.contentType, tc.body, nil, adminSession.cookies, adminSession.csrfHeader())
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("upload status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
			}
			assertNoAssetRows(t, db, adminSession.tenantID)
			if store.count() != 0 {
				t.Fatalf("validation failure left stored objects: %#v", store.objects)
			}
		})
	}

	t.Run("storage failure leaves no metadata", func(t *testing.T) {
		router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
		store.failPut = true
		projectID := createAssetTestProject(t, router, adminSession, "Storage Failure Project")
		response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "ok.png", "image/png", validPNG(t, 2, 2), nil, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("upload status = %d, want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
		}
		assertNoAssetRows(t, db, adminSession.tenantID)
		if store.count() != 0 {
			t.Fatal("storage failure should not leave stored objects")
		}
		assertQuotaCounter(t, db, adminSession.tenantID, 0, 0)
	})

	t.Run("thumbnail storage failure deletes uploaded original and leaves no metadata", func(t *testing.T) {
		router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
		store.failPutBucket = config.DefaultStorageConfig().BucketThumbnails
		projectID := createAssetTestProject(t, router, adminSession, "Thumbnail Storage Failure Project")
		response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "ok.png", "image/png", validPNG(t, 2, 2), nil, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("upload status = %d, want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
		}
		assertNoAssetRows(t, db, adminSession.tenantID)
		if store.count() != 0 {
			t.Fatalf("thumbnail storage failure left orphan objects: %#v", store.objects)
		}
		assertQuotaCounter(t, db, adminSession.tenantID, 0, 0)
	})

	t.Run("database failure deletes uploaded object", func(t *testing.T) {
		router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
		projectID := createAssetTestProject(t, router, adminSession, "Database Failure Project")
		seedQuotaCounter(t, db, adminSession.tenantID, 0, 0)
		if err := db.Exec("DROP TABLE image_assets").Error; err != nil {
			t.Fatalf("drop image_assets: %v", err)
		}
		response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "ok.png", "image/png", validPNG(t, 2, 2), nil, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("upload status = %d, want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
		}
		if store.count() != 0 {
			t.Fatalf("database failure left orphan objects: %#v", store.objects)
		}
		if store.removeCount == 0 {
			t.Fatal("database failure should attempt object cleanup")
		}
		assertQuotaCounter(t, db, adminSession.tenantID, 0, 0)
	})

	t.Run("database failure after request cancellation still cleans uploaded object", func(t *testing.T) {
		router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
		projectID := createAssetTestProject(t, router, adminSession, "Canceled Database Failure Project")
		ctx, cancel := context.WithCancel(context.Background())
		store.onPut = cancel

		response := performMultipartWithContext(ctx, router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "ok.png", "image/png", validPNG(t, 2, 2), nil, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("upload status = %d, want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
		}
		assertNoAssetRows(t, db, adminSession.tenantID)
		if store.count() != 0 {
			t.Fatalf("canceled database failure left orphan objects: %#v", store.objects)
		}
		if store.removeCount == 0 {
			t.Fatal("canceled database failure should attempt independent object cleanup")
		}
		for _, removeErr := range store.removeErrs {
			if errors.Is(removeErr, context.Canceled) {
				t.Fatalf("cleanup used canceled request context: %v", removeErr)
			}
		}
		assertQuotaCounter(t, db, adminSession.tenantID, 0, 0)
	})

	t.Run("database failure with cleanup failure returns sanitized upload failure", func(t *testing.T) {
		router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
		projectID := createAssetTestProject(t, router, adminSession, "Cleanup Failure Project")
		store.failRemove = errors.New("remove failed bucket product-originals key tenants/secret/object filename ok.png base64 data:image/png")
		if err := db.Callback().Create().Before("gorm:create").Register("asset_route_test_fail_image_asset_create", func(tx *gorm.DB) {
			if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "image_assets" {
				tx.AddError(errors.New("metadata insert failed"))
			}
		}); err != nil {
			t.Fatalf("register image asset create failure callback: %v", err)
		}

		response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "ok.png", "image/png", validPNG(t, 2, 2), nil, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("upload status = %d, want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
		}
		assertNoAssetRows(t, db, adminSession.tenantID)
		if store.count() != 2 {
			t.Fatalf("cleanup failure should leave uploaded objects for later recovery, got count %d", store.count())
		}
		body := response.Body.String()
		for _, forbidden := range []string{"product-originals", "tenants/", "object_key", "objectKey", "ok.png", "base64", "data:image", "metadata insert failed", "remove failed"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("upload cleanup failure response leaked %q: %s", forbidden, body)
			}
		}
		assertQuotaCounter(t, db, adminSession.tenantID, 0, 0)
	})
}

func TestAssetUploadAcceptsAllowedImageTypesWithinPolicy(t *testing.T) {
	router, _, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	projectID := createAssetTestProject(t, router, adminSession, "Allowed Upload Project")

	for _, tc := range []struct {
		name        string
		filename    string
		contentType string
		body        []byte
		wantWidth   float64
		wantHeight  float64
	}{
		{name: "png", filename: "allowed.png", contentType: "image/png", body: validPNG(t, 2, 2), wantWidth: 2, wantHeight: 2},
		{name: "jpeg", filename: "allowed.jpg", contentType: "image/jpeg", body: validJPEG(t, 3, 2), wantWidth: 3, wantHeight: 2},
		{name: "webp", filename: "allowed.webp", contentType: "image/webp", body: validWebP(t), wantWidth: 75, wantHeight: 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", tc.filename, tc.contentType, tc.body, nil, adminSession.cookies, adminSession.csrfHeader())
			if response.Code != http.StatusCreated {
				t.Fatalf("allowed %s upload status = %d, want %d: %s", tc.name, response.Code, http.StatusCreated, response.Body.String())
			}
			data := decodeData(t, response)
			if stringField(t, data, "mimeType") != tc.contentType {
				t.Fatalf("%s mimeType = %q, want %q", tc.name, stringField(t, data, "mimeType"), tc.contentType)
			}
			assertFloatField(t, data, "width", tc.wantWidth)
			assertFloatField(t, data, "height", tc.wantHeight)
		})
	}
	if store.count() != 6 {
		t.Fatalf("stored allowed upload object count = %d, want 6", store.count())
	}
}

func TestAssetRoutesExistingAssetWithoutThumbnailKeepsEmptyThumbnailURLAnd404Thumbnail(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	projectID := createAssetTestProject(t, router, adminSession, "Legacy Asset Project")
	now := time.Now().UTC()
	if err := db.Create(&database.ImageAsset{
		ID:        "legacy-asset-no-thumbnail",
		TenantID:  adminSession.tenantID,
		ProjectID: projectID,
		Kind:      asset.KindReference,
		ObjectKey: "tenants/" + adminSession.tenantID + "/projects/" + projectID + "/assets/legacy-asset-no-thumbnail/original.png",
		MimeType:  "image/png",
		SizeBytes: 1,
		Width:     1,
		Height:    1,
		SHA256:    strings.Repeat("1", 64),
		CreatedBy: adminSession.userID,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed legacy asset: %v", err)
	}

	detail := performJSON(router, http.MethodGet, "/api/v1/assets/legacy-asset-no-thumbnail", nil, adminSession.cookies, nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("legacy detail status = %d, want %d: %s", detail.Code, http.StatusOK, detail.Body.String())
	}
	if got := stringField(t, decodeData(t, detail), "thumbnailUrl"); got != "" {
		t.Fatalf("legacy detail thumbnailUrl = %q, want empty", got)
	}
	list := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/assets", nil, adminSession.cookies, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("legacy list status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}
	record := recordsField(t, decodeData(t, list))[0].(map[string]any)
	if got := stringField(t, record, "thumbnailUrl"); got != "" {
		t.Fatalf("legacy list thumbnailUrl = %q, want empty", got)
	}
	thumbnail := performJSON(router, http.MethodGet, "/api/v1/assets/legacy-asset-no-thumbnail/thumbnail", nil, adminSession.cookies, nil)
	if thumbnail.Code != http.StatusNotFound {
		t.Fatalf("legacy thumbnail status = %d, want %d: %s", thumbnail.Code, http.StatusNotFound, thumbnail.Body.String())
	}
	assertResponseExcludes(t, thumbnail.Body.String(), "legacy-asset-no-thumbnail", "tenants/", "objectKey", "thumbnailObjectKey", "product-thumbnails", "minio")
}

type fakeObjectStore struct {
	mu            sync.Mutex
	objects       map[string]fakeObject
	failPut       bool
	failPutBucket string
	failRemove    error
	onPut         func()
	removeErrs    []error
	removeCount   int
}

type fakeObject struct {
	contentType  string
	body         []byte
	lastModified time.Time
}

func newFakeObjectStore() *fakeObjectStore {
	return &fakeObjectStore{objects: map[string]fakeObject{}}
}

func (s *fakeObjectStore) PutObject(_ context.Context, bucket string, key string, body io.Reader, _ int64, contentType string) error {
	s.mu.Lock()
	if s.failPut || bucket == s.failPutBucket {
		s.mu.Unlock()
		return storage.ErrUnavailable
	}
	data, err := io.ReadAll(body)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.objects[bucket+"/"+key] = fakeObject{contentType: contentType, body: data, lastModified: time.Now().UTC()}
	onPut := s.onPut
	s.mu.Unlock()
	if onPut != nil {
		onPut()
	}
	return nil
}

func (s *fakeObjectStore) GetObject(_ context.Context, bucket string, key string) (storage.Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, ok := s.objects[bucket+"/"+key]
	if !ok {
		return storage.Object{}, storage.ErrNotFound
	}
	return storage.Object{
		Body:        io.NopCloser(bytes.NewReader(object.body)),
		Size:        int64(len(object.body)),
		ContentType: object.contentType,
	}, nil
}

func (s *fakeObjectStore) ListObjects(_ context.Context, input storage.ListObjectsInput) (storage.ListObjectsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.Limit <= 0 {
		return storage.ListObjectsResult{}, nil
	}

	prefix := input.Bucket + "/"
	keys := make([]string, 0, len(s.objects))
	for stored := range s.objects {
		if !strings.HasPrefix(stored, prefix) {
			continue
		}
		key := strings.TrimPrefix(stored, prefix)
		if input.Prefix != "" && !strings.HasPrefix(key, input.Prefix) {
			continue
		}
		if input.Cursor != "" && key <= input.Cursor {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	hasMore := input.Limit > 0 && len(keys) > input.Limit
	if hasMore {
		keys = keys[:input.Limit]
	}

	objects := make([]storage.ListedObject, 0, len(keys))
	for _, key := range keys {
		object := s.objects[input.Bucket+"/"+key]
		objects = append(objects, storage.ListedObject{
			Key:          key,
			Size:         int64(len(object.body)),
			LastModified: object.lastModified,
		})
	}
	result := storage.ListObjectsResult{Objects: objects}
	if hasMore && len(objects) > 0 {
		result.NextCursor = objects[len(objects)-1].Key
	}
	return result, nil
}

func (s *fakeObjectStore) RemoveObject(ctx context.Context, bucket string, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeCount++
	if err := ctx.Err(); err != nil {
		s.removeErrs = append(s.removeErrs, err)
		return err
	}
	if s.failRemove != nil {
		s.removeErrs = append(s.removeErrs, s.failRemove)
		return s.failRemove
	}
	delete(s.objects, bucket+"/"+key)
	return nil
}

func (s *fakeObjectStore) has(bucket string, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.objects[bucket+"/"+key]
	return ok
}

func (s *fakeObjectStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.objects)
}

func (s *fakeObjectStore) putListedObject(bucket string, key string, lastModified time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[bucket+"/"+key] = fakeObject{contentType: "image/png", body: []byte("orphan-test-object"), lastModified: lastModified.UTC()}
}

func newAssetRouteTestRouter(t *testing.T, uploadConfig config.UploadConfig) (http.Handler, *gorm.DB, *fakeObjectStore, projectRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	store := newFakeObjectStore()
	cfg := authRouteTestConfig("test")
	cfg.Storage = config.DefaultStorageConfig()
	cfg.Upload = config.NormalizeUploadConfig(uploadConfig)
	router := NewRouter(RouterOptions{
		Config:      cfg,
		Logger:      discardLogger(),
		Database:    db,
		ObjectStore: store,
	})

	initResponse := performJSON(router, http.MethodPost, "/api/v1/auth/init-admin", map[string]string{
		"tenantName":  "Studio Tenant",
		"email":       "admin@example.com",
		"displayName": "Admin User",
		"password":    "initial-password-123",
	}, nil, nil)
	if initResponse.Code != http.StatusCreated {
		t.Fatalf("init admin status = %d, want %d: %s", initResponse.Code, http.StatusCreated, initResponse.Body.String())
	}
	data := decodeData(t, initResponse)
	authCookie := findCookie(t, initResponse, "studio_auth")
	csrfCookie := findCookie(t, initResponse, "studio_csrf")
	return router, db, store, projectRouteSession{
		tenantID: nestedString(t, data, "tenant", "id"),
		userID:   nestedString(t, data, "user", "id"),
		cookies:  []*http.Cookie{authCookie, csrfCookie},
		csrf:     csrfCookie.Value,
	}
}

func createAssetTestProject(t *testing.T, router http.Handler, session projectRouteSession, name string) string {
	t.Helper()

	response := performJSON(router, http.MethodPost, "/api/v1/projects", map[string]string{"name": name}, session.cookies, session.csrfHeader())
	if response.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	return stringField(t, decodeData(t, response), "id")
}

func addMember(t *testing.T, router http.Handler, session projectRouteSession, projectID string, userID string, role string) {
	t.Helper()

	response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": userID,
		"role":   role,
	}, session.cookies, session.csrfHeader())
	if response.Code != http.StatusCreated {
		t.Fatalf("add member status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
}

func uploadAssetForTest(t *testing.T, router http.Handler, session projectRouteSession, projectID string) string {
	t.Helper()

	response := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "asset.png", "image/png", validPNG(t, 2, 2), nil, session.cookies, session.csrfHeader())
	if response.Code != http.StatusCreated {
		t.Fatalf("upload asset status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	return stringField(t, decodeData(t, response), "id")
}

func removeRolePermission(t *testing.T, db *gorm.DB, tenantID string, roleCode string, permissionCode string) {
	t.Helper()

	var role database.Role
	if err := db.Where("tenant_id = ? AND code = ?", tenantID, roleCode).First(&role).Error; err != nil {
		t.Fatalf("find role %s: %v", roleCode, err)
	}
	var permission database.Permission
	if err := db.Where("code = ?", permissionCode).First(&permission).Error; err != nil {
		t.Fatalf("find permission %s: %v", permissionCode, err)
	}
	if err := db.Where("tenant_id = ? AND role_id = ? AND permission_id = ?", tenantID, role.ID, permission.ID).
		Delete(&database.RolePermission{}).Error; err != nil {
		t.Fatalf("remove permission %s from role %s: %v", permissionCode, roleCode, err)
	}
}

func performMultipart(router http.Handler, method string, path string, fileField string, filename string, contentType string, data []byte, fields map[string]string, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	return performMultipartWithContext(context.Background(), router, method, path, fileField, filename, contentType, data, fields, cookies, headers)
}

func performMultipartWithContext(ctx context.Context, router http.Handler, method string, path string, fileField string, filename string, contentType string, data []byte, fields map[string]string, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fileField, strings.ReplaceAll(filename, `"`, `\"`)))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		panic(err)
	}
	if _, err := part.Write(data); err != nil {
		panic(err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			panic(err)
		}
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}

	request := httptest.NewRequest(method, path, body).WithContext(ctx)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func validPNG(t *testing.T, width int, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x + 1), G: uint8(y + 1), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func validJPEG(t *testing.T, width int, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(40 + x), G: uint8(80 + y), B: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func validWebP(t *testing.T) []byte {
	t.Helper()

	data, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatalf("decode webp fixture: %v", err)
	}
	return data
}

func assertNoAssetRows(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()

	var count int64
	if err := db.Model(&database.ImageAsset{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		t.Fatalf("count assets: %v", err)
	}
	if count != 0 {
		t.Fatalf("asset rows = %d, want 0", count)
	}
}

func seedQuotaCounter(t *testing.T, db *gorm.DB, tenantID string, usedBytes int64, reservedBytes int64) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&database.StorageQuotaCounter{
		ID:            "quota-counter-" + tenantID,
		TenantID:      tenantID,
		UsedBytes:     usedBytes,
		ReservedBytes: reservedBytes,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("seed quota counter: %v", err)
	}
}

func releaseReservedQuotaRowsForTest(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.StorageQuotaReservation{}).
			Where("tenant_id = ? AND status = ?", tenantID, "RESERVED").
			Updates(map[string]any{
				"status":     "RELEASED",
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&database.StorageQuotaCounter{}).
			Where("tenant_id = ?", tenantID).
			Updates(map[string]any{
				"reserved_bytes": 0,
				"updated_at":     now,
			}).Error
	}); err != nil {
		t.Fatalf("release reserved quota rows: %v", err)
	}
}

func assertQuotaCounter(t *testing.T, db *gorm.DB, tenantID string, usedBytes int64, reservedBytes int64) {
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

func assertAssetOperationLogs(t *testing.T, db *gorm.DB, expectedActions []string) {
	t.Helper()

	var logs []database.OperationLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}

	seen := map[string]bool{}
	for _, log := range logs {
		seen[log.Action] = true
		metadata := strings.ToLower(log.MetadataJSON)
		for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "jwt", "base64", "data:image"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %#v", forbidden, log)
			}
		}
	}
	for _, action := range expectedActions {
		if !seen[action] {
			t.Fatalf("missing operation log action %s; logs = %#v", action, logs)
		}
	}
}
