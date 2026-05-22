package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"gorm.io/gorm"
)

func TestSystemSettingsRoutesRejectNonAdminAndRequireCSRF(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{})
	seedActiveUser(t, db, adminSession.tenantID, "seller-settings", "seller-settings@example.com", "Seller Settings", "seller-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-settings", "seller")
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-settings@example.com", "seller-password-123")

	getResponse := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, sellerSession.cookies, nil)
	if getResponse.Code != http.StatusForbidden {
		t.Fatalf("seller GET status = %d, want %d: %s", getResponse.Code, http.StatusForbidden, getResponse.Body.String())
	}

	patchResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskDefaults": map[string]any{"defaultProviderId": "provider-a", "defaultModelId": "model-a"},
	}, sellerSession.cookies, sellerSession.csrfHeader())
	if patchResponse.Code != http.StatusForbidden {
		t.Fatalf("seller PATCH status = %d, want %d: %s", patchResponse.Code, http.StatusForbidden, patchResponse.Body.String())
	}

	noCSRFResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskDefaults": map[string]any{"defaultProviderId": "provider-a", "defaultModelId": "model-a"},
	}, adminSession.cookies, nil)
	if noCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("admin PATCH without CSRF status = %d, want %d: %s", noCSRFResponse.Code, http.StatusForbidden, noCSRFResponse.Body.String())
	}
}

func TestSystemSettingsGetReturnsConfigFallbackWithoutOverride(t *testing.T) {
	upload := config.UploadConfig{
		MaxFileSizeBytes: 4096,
		MaxWidth:         33,
		MaxHeight:        44,
		MaxPixels:        55,
		AllowedMIMETypes: []string{"image/png"},
	}
	router, db, _, adminSession := newAssetRouteTestRouter(t, upload)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertUploadPolicy(t, response, upload.MaxFileSizeBytes, upload.MaxWidth, upload.MaxHeight, upload.MaxPixels)
	assertTaskDefaults(t, response, "", "")
	assertResponseExcludes(t, response.Body.String(), "tenantConcurrency", "storageQuotaBytes", "logRetentionDays", "allowedMimeTypes")

	var rows int64
	if err := db.Model(&database.SystemSetting{}).Where("tenant_id = ?", adminSession.tenantID).Count(&rows).Error; err != nil {
		t.Fatalf("count system settings: %v", err)
	}
	if rows != 0 {
		t.Fatalf("fallback GET created %d override rows, want 0", rows)
	}
}

func TestSystemSettingsTaskDefaultsPatchGetClearValidationAndTenantIsolation(t *testing.T) {
	upload := config.UploadConfig{
		MaxFileSizeBytes: 2048,
		MaxWidth:         10,
		MaxHeight:        10,
		MaxPixels:        100,
		AllowedMIMETypes: []string{"image/png"},
	}
	router, db, _, adminSession := newAssetRouteTestRouter(t, upload)
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "settings-defaults", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
	tenantBSession := seedTenantAdminSession(t, router, db, "tenant-b", "tenant-b-admin", "tenant-b-admin@example.com", "tenant-b-password-123")

	getBefore := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if getBefore.Code != http.StatusOK {
		t.Fatalf("GET before status = %d, want %d: %s", getBefore.Code, http.StatusOK, getBefore.Body.String())
	}
	assertTaskDefaults(t, getBefore, "", "")

	setResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskDefaults": map[string]any{
			"defaultProviderId": providerID,
			"defaultModelId":    modelID,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setResponse.Code != http.StatusOK {
		t.Fatalf("set taskDefaults status = %d, want %d: %s", setResponse.Code, http.StatusOK, setResponse.Body.String())
	}
	assertUploadPolicy(t, setResponse, upload.MaxFileSizeBytes, upload.MaxWidth, upload.MaxHeight, upload.MaxPixels)
	assertTaskDefaults(t, setResponse, providerID, modelID)

	var setting database.SystemSetting
	if err := db.Where("tenant_id = ? AND `key` = ?", adminSession.tenantID, settings.KeyTaskDefaults).First(&setting).Error; err != nil {
		t.Fatalf("load task defaults setting: %v", err)
	}
	if !strings.Contains(setting.ValueJSON, providerID) || !strings.Contains(setting.ValueJSON, modelID) {
		t.Fatalf("task defaults value_json = %s, want provider/model IDs", setting.ValueJSON)
	}

	tenantBGet := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, tenantBSession.cookies, nil)
	if tenantBGet.Code != http.StatusOK {
		t.Fatalf("tenant B GET status = %d, want %d: %s", tenantBGet.Code, http.StatusOK, tenantBGet.Body.String())
	}
	assertTaskDefaults(t, tenantBGet, "", "")

	clearResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskDefaults": map[string]any{
			"defaultProviderId": nil,
			"defaultModelId":    nil,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if clearResponse.Code != http.StatusOK {
		t.Fatalf("clear taskDefaults status = %d, want %d: %s", clearResponse.Code, http.StatusOK, clearResponse.Body.String())
	}
	assertTaskDefaults(t, clearResponse, "", "")

	var logs []database.OperationLog
	if err := db.Where("tenant_id = ? AND action = ? AND resource_id = ?", adminSession.tenantID, settings.ActionUpdateSystemSettings, settings.KeyTaskDefaults).Find(&logs).Error; err != nil {
		t.Fatalf("load taskDefaults operation logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("taskDefaults operation logs = %d, want 2: %#v", len(logs), logs)
	}
	for _, log := range logs {
		metadata := strings.ToLower(log.MetadataJSON)
		for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "jwt", "base64", "data:image"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %s", forbidden, log.MetadataJSON)
			}
		}
	}
}

func TestSystemSettingsTaskDefaultsRejectsInvalidReferencesAndShapes(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{MaxFileSizeBytes: 2048, MaxWidth: 10, MaxHeight: 10, MaxPixels: 100})
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "settings-valid", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
	disabledProviderID, disabledProviderModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "settings-disabled-provider", provider.StatusDisabled, model.StatusEnabled, true, true, false, false, 1)
	enabledProviderID, disabledModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "settings-disabled-model", provider.StatusEnabled, model.StatusDisabled, true, true, false, false, 1)
	otherProviderID, otherModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "settings-other-provider", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
	seedOtherTenantTask(t, db)

	valid := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskDefaults": map[string]any{"defaultProviderId": providerID, "defaultModelId": modelID},
	}, adminSession.cookies, adminSession.csrfHeader())
	if valid.Code != http.StatusOK {
		t.Fatalf("valid taskDefaults status = %d, want %d: %s", valid.Code, http.StatusOK, valid.Body.String())
	}

	cases := []struct {
		name string
		body map[string]any
	}{
		{name: "empty root", body: map[string]any{}},
		{name: "top-level unknown field", body: map[string]any{"tenantConcurrency": 2}},
		{name: "top-level defaultProviderId", body: map[string]any{"defaultProviderId": providerID}},
		{name: "missing model", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": providerID}}},
		{name: "missing provider", body: map[string]any{"taskDefaults": map[string]any{"defaultModelId": modelID}}},
		{name: "single-field null", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": nil}}},
		{name: "one null one non-null", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": providerID, "defaultModelId": nil}}},
		{name: "unknown nested field", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": providerID, "defaultModelId": modelID, "extra": "blocked"}}},
		{name: "empty provider", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": " ", "defaultModelId": modelID}}},
		{name: "empty model", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": providerID, "defaultModelId": ""}}},
		{name: "disabled provider", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": disabledProviderID, "defaultModelId": disabledProviderModelID}}},
		{name: "disabled model", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": enabledProviderID, "defaultModelId": disabledModelID}}},
		{name: "cross-tenant provider", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": "provider-tenant-b", "defaultModelId": modelID}}},
		{name: "cross-tenant model", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": providerID, "defaultModelId": "model-tenant-b"}}},
		{name: "model belongs to different provider", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": otherProviderID, "defaultModelId": modelID}}},
		{name: "provider belongs to different model", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": providerID, "defaultModelId": otherModelID}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", tc.body, adminSession.cookies, adminSession.csrfHeader())
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("PATCH status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
			}
			getResponse := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
			if getResponse.Code != http.StatusOK {
				t.Fatalf("GET status = %d, want %d: %s", getResponse.Code, http.StatusOK, getResponse.Body.String())
			}
			assertTaskDefaults(t, getResponse, providerID, modelID)
			assertResponseExcludes(t, response.Body.String(), "provider-tenant-b", "model-tenant-b", disabledProviderID, disabledModelID)
		})
	}
}

func TestSystemSettingsPatchIsTenantScopedAndWritesSanitizedOperationLog(t *testing.T) {
	upload := config.UploadConfig{
		MaxFileSizeBytes: 2048,
		MaxWidth:         10,
		MaxHeight:        10,
		MaxPixels:        100,
		AllowedMIMETypes: []string{"image/png"},
	}
	router, db, _, adminSession := newAssetRouteTestRouter(t, upload)
	tenantBSession := seedTenantAdminSession(t, router, db, "tenant-b", "tenant-b-admin", "tenant-b-admin@example.com", "tenant-b-password-123")
	seedSystemSetting(t, db, "tenant-b", `{"maxFileSizeBytes":1024,"maxWidth":3,"maxHeight":3,"maxPixels":9}`)

	response := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"uploadPolicy": map[string]any{
			"maxWidth":  2,
			"maxHeight": 3,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertUploadPolicy(t, response, upload.MaxFileSizeBytes, 2, 3, upload.MaxPixels)

	tenantAGet := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if tenantAGet.Code != http.StatusOK {
		t.Fatalf("tenant A GET status = %d, want %d: %s", tenantAGet.Code, http.StatusOK, tenantAGet.Body.String())
	}
	assertUploadPolicy(t, tenantAGet, upload.MaxFileSizeBytes, 2, 3, upload.MaxPixels)

	tenantBGet := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, tenantBSession.cookies, nil)
	if tenantBGet.Code != http.StatusOK {
		t.Fatalf("tenant B GET status = %d, want %d: %s", tenantBGet.Code, http.StatusOK, tenantBGet.Body.String())
	}
	assertUploadPolicy(t, tenantBGet, 1024, 3, 3, 9)

	var tenantBSetting database.SystemSetting
	if err := db.Where("tenant_id = ? AND key = ?", "tenant-b", "upload_policy").First(&tenantBSetting).Error; err != nil {
		t.Fatalf("load tenant B setting: %v", err)
	}
	if !strings.Contains(tenantBSetting.ValueJSON, `"maxWidth":3`) {
		t.Fatalf("tenant B setting changed unexpectedly: %s", tenantBSetting.ValueJSON)
	}

	var logs []database.OperationLog
	if err := db.Where("tenant_id = ? AND action = ?", adminSession.tenantID, "system_settings.update").Find(&logs).Error; err != nil {
		t.Fatalf("load settings operation logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("settings operation logs = %d, want 1: %#v", len(logs), logs)
	}
	metadata := strings.ToLower(logs[0].MetadataJSON)
	for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "jwt", "base64", "data:image"} {
		if strings.Contains(metadata, forbidden) {
			t.Fatalf("operation log metadata contains %q: %s", forbidden, logs[0].MetadataJSON)
		}
	}
}

func TestSystemSettingsPatchRejectsInvalidOverCapAndDeferredFieldsWithoutChangingPolicy(t *testing.T) {
	upload := config.UploadConfig{
		MaxFileSizeBytes: 2048,
		MaxWidth:         10,
		MaxHeight:        10,
		MaxPixels:        100,
		AllowedMIMETypes: []string{"image/png"},
	}
	router, _, _, adminSession := newAssetRouteTestRouter(t, upload)

	initial := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"uploadPolicy": map[string]any{"maxWidth": 8},
	}, adminSession.cookies, adminSession.csrfHeader())
	if initial.Code != http.StatusOK {
		t.Fatalf("initial PATCH status = %d, want %d: %s", initial.Code, http.StatusOK, initial.Body.String())
	}

	cases := []struct {
		name string
		body any
	}{
		{name: "zero", body: map[string]any{"uploadPolicy": map[string]any{"maxWidth": 0}}},
		{name: "negative", body: map[string]any{"uploadPolicy": map[string]any{"maxHeight": -1}}},
		{name: "malformed", body: map[string]any{"uploadPolicy": map[string]any{"maxPixels": "many"}}},
		{name: "over hard cap", body: map[string]any{"uploadPolicy": map[string]any{"maxFileSizeBytes": upload.MaxFileSizeBytes + 1}}},
		{name: "deferred defaultProviderId", body: map[string]any{"defaultProviderId": "provider-a"}},
		{name: "deferred defaultModelId", body: map[string]any{"defaultModelId": "model-a"}},
		{name: "deferred tenantConcurrency", body: map[string]any{"tenantConcurrency": 2}},
		{name: "mime allowlist is not writable", body: map[string]any{"uploadPolicy": map[string]any{"allowedMimeTypes": []string{"image/svg+xml"}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", tc.body, adminSession.cookies, adminSession.csrfHeader())
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("PATCH status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
			}

			getResponse := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
			if getResponse.Code != http.StatusOK {
				t.Fatalf("GET status = %d, want %d: %s", getResponse.Code, http.StatusOK, getResponse.Body.String())
			}
			assertUploadPolicy(t, getResponse, upload.MaxFileSizeBytes, 8, upload.MaxHeight, upload.MaxPixels)
		})
	}
}

func TestAssetUploadUsesTenantUploadPolicyAndKeepsMimeAllowlistStatic(t *testing.T) {
	upload := config.UploadConfig{
		MaxFileSizeBytes: 1024 * 1024,
		MaxWidth:         10,
		MaxHeight:        10,
		MaxPixels:        100,
		AllowedMIMETypes: []string{"image/png"},
	}

	for _, tc := range []struct {
		name  string
		patch map[string]any
	}{
		{name: "file size", patch: map[string]any{"maxFileSizeBytes": 8}},
		{name: "width", patch: map[string]any{"maxWidth": 1}},
		{name: "height", patch: map[string]any{"maxHeight": 1}},
		{name: "pixels", patch: map[string]any{"maxPixels": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, _, _, adminSession := newAssetRouteTestRouter(t, upload)
			projectID := createAssetTestProject(t, router, adminSession, "Tenant A Upload Policy "+tc.name)

			tighten := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
				"uploadPolicy": tc.patch,
			}, adminSession.cookies, adminSession.csrfHeader())
			if tighten.Code != http.StatusOK {
				t.Fatalf("tighten PATCH status = %d, want %d: %s", tighten.Code, http.StatusOK, tighten.Body.String())
			}

			rejected := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "blocked.png", "image/png", validPNG(t, 2, 2), nil, adminSession.cookies, adminSession.csrfHeader())
			if rejected.Code != http.StatusUnprocessableEntity {
				t.Fatalf("tenant A upload status = %d, want %d: %s", rejected.Code, http.StatusUnprocessableEntity, rejected.Body.String())
			}
		})
	}

	router, db, _, adminSession := newAssetRouteTestRouter(t, upload)
	projectAID := createAssetTestProject(t, router, adminSession, "Tenant A Upload Policy Isolation")

	tighten := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"uploadPolicy": map[string]any{"maxWidth": 1},
	}, adminSession.cookies, adminSession.csrfHeader())
	if tighten.Code != http.StatusOK {
		t.Fatalf("tighten PATCH status = %d, want %d: %s", tighten.Code, http.StatusOK, tighten.Body.String())
	}

	rejected := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectAID+"/assets/uploads", "file", "too-wide.png", "image/png", validPNG(t, 2, 2), nil, adminSession.cookies, adminSession.csrfHeader())
	if rejected.Code != http.StatusUnprocessableEntity {
		t.Fatalf("tenant A upload status = %d, want %d: %s", rejected.Code, http.StatusUnprocessableEntity, rejected.Body.String())
	}

	tenantBSession := seedTenantAdminSession(t, router, db, "tenant-b", "tenant-b-admin", "tenant-b-admin@example.com", "tenant-b-password-123")
	seedTenantProject(t, db, "tenant-b", tenantBSession.userID, "project-tenant-b")

	tenantBUpload := performMultipart(router, http.MethodPost, "/api/v1/projects/project-tenant-b/assets/uploads", "file", "tenant-b.png", "image/png", validPNG(t, 2, 2), nil, tenantBSession.cookies, tenantBSession.csrfHeader())
	if tenantBUpload.Code != http.StatusCreated {
		t.Fatalf("tenant B upload status = %d, want %d: %s", tenantBUpload.Code, http.StatusCreated, tenantBUpload.Body.String())
	}

	svg := performMultipart(router, http.MethodPost, "/api/v1/projects/project-tenant-b/assets/uploads", "file", "unsafe.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), nil, tenantBSession.cookies, tenantBSession.csrfHeader())
	if svg.Code != http.StatusUnprocessableEntity {
		t.Fatalf("SVG upload status = %d, want %d: %s", svg.Code, http.StatusUnprocessableEntity, svg.Body.String())
	}
}

func assertUploadPolicy(t *testing.T, response *httptest.ResponseRecorder, maxFileSizeBytes int64, maxWidth int, maxHeight int, maxPixels int64) {
	t.Helper()

	data := decodeData(t, response)
	policy := objectField(t, data, "uploadPolicy")
	assertNumericField(t, policy, "maxFileSizeBytes", maxFileSizeBytes)
	assertNumericField(t, policy, "maxWidth", int64(maxWidth))
	assertNumericField(t, policy, "maxHeight", int64(maxHeight))
	assertNumericField(t, policy, "maxPixels", maxPixels)
}

func assertTaskDefaults(t *testing.T, response *httptest.ResponseRecorder, expectedProviderID string, expectedModelID string) {
	t.Helper()

	data := decodeData(t, response)
	defaults := objectField(t, data, "taskDefaults")
	if expectedProviderID == "" {
		if defaults["defaultProviderId"] != nil {
			t.Fatalf("defaultProviderId = %#v, want null", defaults["defaultProviderId"])
		}
	} else if value, ok := defaults["defaultProviderId"].(string); !ok || value != expectedProviderID {
		t.Fatalf("defaultProviderId = %#v, want %q", defaults["defaultProviderId"], expectedProviderID)
	}
	if expectedModelID == "" {
		if defaults["defaultModelId"] != nil {
			t.Fatalf("defaultModelId = %#v, want null", defaults["defaultModelId"])
		}
	} else if value, ok := defaults["defaultModelId"].(string); !ok || value != expectedModelID {
		t.Fatalf("defaultModelId = %#v, want %q", defaults["defaultModelId"], expectedModelID)
	}
}

func assertNumericField(t *testing.T, object map[string]any, field string, expected int64) {
	t.Helper()

	value, ok := object[field].(float64)
	if !ok {
		t.Fatalf("%s = %#v, want number", field, object[field])
	}
	if int64(value) != expected {
		t.Fatalf("%s = %v, want %d", field, value, expected)
	}
}

func seedTenantAdminSession(t *testing.T, router http.Handler, db *gorm.DB, tenantID string, userID string, email string, password string) projectRouteSession {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&database.Tenant{
		ID:        tenantID,
		Name:      tenantID,
		Status:    auth.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant %s: %v", tenantID, err)
	}
	seedActiveUser(t, db, tenantID, userID, email, "Tenant Admin", password)

	var permission database.Permission
	if err := db.Where("code = ?", "system:settings:manage").First(&permission).Error; err != nil {
		t.Fatalf("load settings permission: %v", err)
	}
	role := database.Role{
		ID:          "role-" + userID + "-admin",
		TenantID:    tenantID,
		Code:        "admin",
		Name:        "Admin",
		Description: "Tenant administrator",
		Status:      auth.RoleStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("seed tenant admin role: %v", err)
	}
	if err := db.Create(&database.UserRole{
		ID:        "user-role-" + userID + "-admin",
		TenantID:  tenantID,
		UserID:    userID,
		RoleID:    role.ID,
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("assign tenant admin role: %v", err)
	}
	if err := db.Create(&database.RolePermission{
		ID:           "role-permission-" + userID + "-settings",
		TenantID:     tenantID,
		RoleID:       role.ID,
		PermissionID: permission.ID,
		CreatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("assign settings permission: %v", err)
	}

	return loginProjectRouteUser(t, router, tenantID, email, password)
}

func seedTenantProject(t *testing.T, db *gorm.DB, tenantID string, userID string, projectID string) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&database.Project{
		ID:        projectID,
		TenantID:  tenantID,
		Name:      projectID,
		Status:    project.StatusActive,
		CreatedBy: userID,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant project: %v", err)
	}
}

func seedSystemSetting(t *testing.T, db *gorm.DB, tenantID string, valueJSON string) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&database.SystemSetting{
		ID:        "setting-" + tenantID,
		TenantID:  tenantID,
		Key:       "upload_policy",
		ValueJSON: valueJSON,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed system setting: %v", err)
	}
}
