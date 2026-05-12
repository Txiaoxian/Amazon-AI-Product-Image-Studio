package api

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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

	var record database.ImageAsset
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, assetID).First(&record).Error; err != nil {
		t.Fatalf("load image asset metadata: %v", err)
	}
	expectedObjectKey := "tenants/" + adminSession.tenantID + "/projects/" + projectID + "/assets/" + assetID + "/original.png"
	if record.ObjectKey != expectedObjectKey {
		t.Fatalf("object key = %q, want %q", record.ObjectKey, expectedObjectKey)
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

	listResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/assets?kind=REFERENCE&category=reference&favorite=true&pageNum=1&pageSize=10", nil, editorSession.cookies, nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d: %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	listData := decodeData(t, listResponse)
	if total, ok := listData["total"].(float64); !ok || total != 1 {
		t.Fatalf("asset list total = %#v, want 1", listData["total"])
	}

	detailResponse := performJSON(router, http.MethodGet, "/api/v1/assets/"+assetID, nil, editorSession.cookies, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d: %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
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

	assertAssetOperationLogs(t, db, []string{
		"asset.upload",
		"asset.update",
		"asset.favorite",
		"asset.unfavorite",
		"asset.download",
		"asset.delete",
	})
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
	})

	t.Run("database failure deletes uploaded object", func(t *testing.T) {
		router, db, store, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
		projectID := createAssetTestProject(t, router, adminSession, "Database Failure Project")
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
	})
}

type fakeObjectStore struct {
	mu          sync.Mutex
	objects     map[string]fakeObject
	failPut     bool
	removeCount int
}

type fakeObject struct {
	contentType string
	body        []byte
}

func newFakeObjectStore() *fakeObjectStore {
	return &fakeObjectStore{objects: map[string]fakeObject{}}
}

func (s *fakeObjectStore) PutObject(_ context.Context, bucket string, key string, body io.Reader, _ int64, contentType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failPut {
		return storage.ErrUnavailable
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[bucket+"/"+key] = fakeObject{contentType: contentType, body: data}
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

func (s *fakeObjectStore) RemoveObject(_ context.Context, bucket string, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeCount++
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

func performMultipart(router http.Handler, method string, path string, fileField string, filename string, contentType string, data []byte, fields map[string]string, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
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

	request := httptest.NewRequest(method, path, body)
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
