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
		"taskConcurrency": map[string]any{"tenantLimit": 1},
	}, sellerSession.cookies, sellerSession.csrfHeader())
	if patchResponse.Code != http.StatusForbidden {
		t.Fatalf("seller PATCH status = %d, want %d: %s", patchResponse.Code, http.StatusForbidden, patchResponse.Body.String())
	}

	noCSRFResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskConcurrency": map[string]any{"tenantLimit": 1},
	}, adminSession.cookies, nil)
	if noCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("admin PATCH without CSRF status = %d, want %d: %s", noCSRFResponse.Code, http.StatusForbidden, noCSRFResponse.Body.String())
	}

	var settingRows int64
	if err := db.Model(&database.SystemSetting{}).Where("`key` = ?", "task_concurrency").Count(&settingRows).Error; err != nil {
		t.Fatalf("count rejected taskConcurrency rows: %v", err)
	}
	var logRows int64
	if err := db.Model(&database.OperationLog{}).Where("resource_id = ?", "task_concurrency").Count(&logRows).Error; err != nil {
		t.Fatalf("count rejected taskConcurrency logs: %v", err)
	}
	if settingRows != 0 || logRows != 0 {
		t.Fatalf("rejected taskConcurrency writes created rows settings=%d logs=%d", settingRows, logRows)
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
	assertStorageRetention(t, response, nil)
	assertStorageQuota(t, response, nil, 0)
	assertLogRetention(t, response, nil, nil, nil)
	assertSystemSettingsConstraints(t, response, upload)
	assertResponseExcludes(t, response.Body.String(), "tenantConcurrency", "storageQuotaBytes", "allowedMimeTypes")

	var rows int64
	if err := db.Model(&database.SystemSetting{}).Where("tenant_id = ?", adminSession.tenantID).Count(&rows).Error; err != nil {
		t.Fatalf("count system settings: %v", err)
	}
	if rows != 0 {
		t.Fatalf("fallback GET created %d override rows, want 0", rows)
	}
}

func TestSystemSettingsStorageQuotaGetPatchClearValidationUsageAndAudit(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{MaxFileSizeBytes: 2048, MaxWidth: 10, MaxHeight: 10, MaxPixels: 100})
	tenantBSession := seedTenantAdminSession(t, router, db, "tenant-b", "tenant-b-admin", "tenant-b-admin@example.com", "tenant-b-password-123")
	now := time.Now().UTC()
	purgedAt := now
	seedQuotaAsset(t, db, adminSession.tenantID, "quota-active", 100, nil, nil)
	seedQuotaAsset(t, db, adminSession.tenantID, "quota-soft-deleted", 200, &now, nil)
	seedQuotaAsset(t, db, adminSession.tenantID, "quota-purged", 400, &now, &purgedAt)
	seedQuotaAsset(t, db, "tenant-b", "quota-cross-tenant", 800, nil, nil)

	getBefore := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if getBefore.Code != http.StatusOK {
		t.Fatalf("GET before status = %d, want %d: %s", getBefore.Code, http.StatusOK, getBefore.Body.String())
	}
	assertStorageQuota(t, getBefore, nil, 300)

	setResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageQuota": map[string]any{"maxBytes": 1},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setResponse.Code != http.StatusOK {
		t.Fatalf("set storageQuota status = %d, want %d: %s", setResponse.Code, http.StatusOK, setResponse.Body.String())
	}
	assertStorageQuota(t, setResponse, ptrInt64(1), 300)

	var setting database.SystemSetting
	if err := db.Where("tenant_id = ? AND `key` = ?", adminSession.tenantID, settings.KeyStorageQuota).First(&setting).Error; err != nil {
		t.Fatalf("load storage quota setting: %v", err)
	}
	if !strings.Contains(setting.ValueJSON, `"maxBytes":1`) {
		t.Fatalf("storage quota value_json = %s, want maxBytes=1", setting.ValueJSON)
	}

	tenantBGet := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, tenantBSession.cookies, nil)
	if tenantBGet.Code != http.StatusOK {
		t.Fatalf("tenant B GET status = %d, want %d: %s", tenantBGet.Code, http.StatusOK, tenantBGet.Body.String())
	}
	assertStorageQuota(t, tenantBGet, nil, 800)

	cases := []struct {
		name string
		body any
	}{
		{name: "usedBytes read only", body: map[string]any{"storageQuota": map[string]any{"usedBytes": 1}}},
		{name: "unknown nested field", body: map[string]any{"storageQuota": map[string]any{"maxBytes": 1024, "extra": 1}}},
		{name: "zero", body: map[string]any{"storageQuota": map[string]any{"maxBytes": 0}}},
		{name: "negative", body: map[string]any{"storageQuota": map[string]any{"maxBytes": -1}}},
		{name: "non integer", body: map[string]any{"storageQuota": map[string]any{"maxBytes": 1.5}}},
		{name: "string", body: map[string]any{"storageQuota": map[string]any{"maxBytes": "1024"}}},
		{name: "over range", body: map[string]any{"storageQuota": map[string]any{"maxBytes": int64(109951162777600) + 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", tc.body, adminSession.cookies, adminSession.csrfHeader())
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("PATCH status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
			}
			if tc.name == "provider over cap" {
				body := response.Body.String()
				for _, expected := range []string{
					`"message":"Provider 并发上限必须在 1 到 6 之间。"`,
					`"field":"taskConcurrency.providerLimit"`,
					`"min":1`,
					`"max":6`,
				} {
					if !strings.Contains(body, expected) {
						t.Fatalf("structured validation response missing %s: %s", expected, body)
					}
				}
			}
			getResponse := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
			if getResponse.Code != http.StatusOK {
				t.Fatalf("GET status = %d, want %d: %s", getResponse.Code, http.StatusOK, getResponse.Body.String())
			}
			assertStorageQuota(t, getResponse, ptrInt64(1), 300)
		})
	}

	clearResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageQuota": map[string]any{"maxBytes": nil},
	}, adminSession.cookies, adminSession.csrfHeader())
	if clearResponse.Code != http.StatusOK {
		t.Fatalf("clear storageQuota status = %d, want %d: %s", clearResponse.Code, http.StatusOK, clearResponse.Body.String())
	}
	assertStorageQuota(t, clearResponse, nil, 300)

	var logs []database.OperationLog
	if err := db.Where("tenant_id = ? AND action = ? AND resource_id = ?", adminSession.tenantID, settings.ActionUpdateSystemSettings, settings.KeyStorageQuota).Find(&logs).Error; err != nil {
		t.Fatalf("load storageQuota operation logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("storageQuota operation logs = %d, want 2: %#v", len(logs), logs)
	}
	for _, log := range logs {
		metadata := strings.ToLower(log.MetadataJSON)
		if !strings.Contains(metadata, "storage_quota") || !strings.Contains(metadata, "changedfields") || !strings.Contains(metadata, "usedbytes") {
			t.Fatalf("operation log missing sanitized quota metadata: %s", log.MetadataJSON)
		}
		for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "jwt", "base64", "data:image", "value_json", "bucket", "object_key", "minio"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %s", forbidden, log.MetadataJSON)
			}
		}
	}
}

func TestSystemSettingsStorageRetentionPatchClearValidationAndTenantIsolation(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{MaxFileSizeBytes: 2048, MaxWidth: 10, MaxHeight: 10, MaxPixels: 100})
	tenantBSession := seedTenantAdminSession(t, router, db, "tenant-b", "tenant-b-admin", "tenant-b-admin@example.com", "tenant-b-password-123")

	getBefore := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if getBefore.Code != http.StatusOK {
		t.Fatalf("GET before status = %d, want %d: %s", getBefore.Code, http.StatusOK, getBefore.Body.String())
	}
	assertStorageRetention(t, getBefore, nil)

	setResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageRetention": map[string]any{"deletedAssetRetentionDays": 30},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setResponse.Code != http.StatusOK {
		t.Fatalf("set storageRetention status = %d, want %d: %s", setResponse.Code, http.StatusOK, setResponse.Body.String())
	}
	assertStorageRetention(t, setResponse, ptrInt(30))

	var setting database.SystemSetting
	if err := db.Where("tenant_id = ? AND `key` = ?", adminSession.tenantID, settings.KeyStorageRetention).First(&setting).Error; err != nil {
		t.Fatalf("load storage retention setting: %v", err)
	}
	if !strings.Contains(setting.ValueJSON, `"deletedAssetRetentionDays":30`) {
		t.Fatalf("storage retention value_json = %s, want deletedAssetRetentionDays=30", setting.ValueJSON)
	}

	tenantBGet := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, tenantBSession.cookies, nil)
	if tenantBGet.Code != http.StatusOK {
		t.Fatalf("tenant B GET status = %d, want %d: %s", tenantBGet.Code, http.StatusOK, tenantBGet.Body.String())
	}
	assertStorageRetention(t, tenantBGet, nil)

	clearResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageRetention": map[string]any{"deletedAssetRetentionDays": nil},
	}, adminSession.cookies, adminSession.csrfHeader())
	if clearResponse.Code != http.StatusOK {
		t.Fatalf("clear storageRetention status = %d, want %d: %s", clearResponse.Code, http.StatusOK, clearResponse.Body.String())
	}
	assertStorageRetention(t, clearResponse, nil)

	var logs []database.OperationLog
	if err := db.Where("tenant_id = ? AND action = ? AND resource_id = ?", adminSession.tenantID, settings.ActionUpdateSystemSettings, settings.KeyStorageRetention).Find(&logs).Error; err != nil {
		t.Fatalf("load storageRetention operation logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("storageRetention operation logs = %d, want 2: %#v", len(logs), logs)
	}
	for _, log := range logs {
		metadata := strings.ToLower(log.MetadataJSON)
		if !strings.Contains(metadata, "storage_retention") || !strings.Contains(metadata, "changedfields") {
			t.Fatalf("operation log missing sanitized retention metadata: %s", log.MetadataJSON)
		}
		for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "jwt", "base64", "data:image", "value_json", "bucket", "object_key", "minio"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %s", forbidden, log.MetadataJSON)
			}
		}
	}
}

func TestSystemSettingsLogRetentionPatchClearValidationTenantIsolationAndAudit(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{MaxFileSizeBytes: 2048, MaxWidth: 10, MaxHeight: 10, MaxPixels: 100})
	tenantBSession := seedTenantAdminSession(t, router, db, "tenant-b", "tenant-b-admin", "tenant-b-admin@example.com", "tenant-b-password-123")

	getBefore := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if getBefore.Code != http.StatusOK {
		t.Fatalf("GET before status = %d, want %d: %s", getBefore.Code, http.StatusOK, getBefore.Body.String())
	}
	assertLogRetention(t, getBefore, nil, nil, nil)

	setResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"logRetention": map[string]any{
			"operationLogRetentionDays": 30,
			"apiCallLogRetentionDays":   14,
			"taskEventRetentionDays":    nil,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setResponse.Code != http.StatusOK {
		t.Fatalf("set logRetention status = %d, want %d: %s", setResponse.Code, http.StatusOK, setResponse.Body.String())
	}
	assertLogRetention(t, setResponse, ptrInt(30), ptrInt(14), nil)

	var setting database.SystemSetting
	if err := db.Where("tenant_id = ? AND `key` = ?", adminSession.tenantID, settings.KeyLogRetention).First(&setting).Error; err != nil {
		t.Fatalf("load log retention setting: %v", err)
	}
	if !strings.Contains(setting.ValueJSON, `"operationLogRetentionDays":30`) || !strings.Contains(setting.ValueJSON, `"apiCallLogRetentionDays":14`) || !strings.Contains(setting.ValueJSON, `"taskEventRetentionDays":null`) {
		t.Fatalf("log retention value_json = %s, want patched nullable fields", setting.ValueJSON)
	}

	partialResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"logRetention": map[string]any{"taskEventRetentionDays": 7},
	}, adminSession.cookies, adminSession.csrfHeader())
	if partialResponse.Code != http.StatusOK {
		t.Fatalf("partial logRetention status = %d, want %d: %s", partialResponse.Code, http.StatusOK, partialResponse.Body.String())
	}
	assertLogRetention(t, partialResponse, ptrInt(30), ptrInt(14), ptrInt(7))

	tenantBGet := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, tenantBSession.cookies, nil)
	if tenantBGet.Code != http.StatusOK {
		t.Fatalf("tenant B GET status = %d, want %d: %s", tenantBGet.Code, http.StatusOK, tenantBGet.Body.String())
	}
	assertLogRetention(t, tenantBGet, nil, nil, nil)

	clearResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"logRetention": map[string]any{
			"operationLogRetentionDays": nil,
			"apiCallLogRetentionDays":   nil,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if clearResponse.Code != http.StatusOK {
		t.Fatalf("clear logRetention status = %d, want %d: %s", clearResponse.Code, http.StatusOK, clearResponse.Body.String())
	}
	assertLogRetention(t, clearResponse, nil, nil, ptrInt(7))

	var logs []database.OperationLog
	if err := db.Where("tenant_id = ? AND action = ? AND resource_id = ?", adminSession.tenantID, settings.ActionUpdateSystemSettings, settings.KeyLogRetention).Find(&logs).Error; err != nil {
		t.Fatalf("load logRetention operation logs: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("logRetention operation logs = %d, want 3: %#v", len(logs), logs)
	}
	for _, log := range logs {
		metadata := strings.ToLower(log.MetadataJSON)
		if !strings.Contains(metadata, "log_retention") || !strings.Contains(metadata, "changedfields") {
			t.Fatalf("operation log missing sanitized log retention metadata: %s", log.MetadataJSON)
		}
		for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "jwt", "base64", "data:image", "value_json", "bucket", "object_key", "minio", "request"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %s", forbidden, log.MetadataJSON)
			}
		}
	}
}

func TestSystemSettingsLogRetentionRejectsInvalidFieldsAndMalformedStoredRows(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{MaxFileSizeBytes: 2048, MaxWidth: 10, MaxHeight: 10, MaxPixels: 100})

	initial := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"logRetention": map[string]any{"operationLogRetentionDays": 30},
	}, adminSession.cookies, adminSession.csrfHeader())
	if initial.Code != http.StatusOK {
		t.Fatalf("initial logRetention PATCH status = %d, want %d: %s", initial.Code, http.StatusOK, initial.Body.String())
	}

	cases := []struct {
		name string
		body any
	}{
		{name: "zero", body: map[string]any{"logRetention": map[string]any{"operationLogRetentionDays": 0}}},
		{name: "negative", body: map[string]any{"logRetention": map[string]any{"apiCallLogRetentionDays": -1}}},
		{name: "non integer", body: map[string]any{"logRetention": map[string]any{"taskEventRetentionDays": 1.5}}},
		{name: "string", body: map[string]any{"logRetention": map[string]any{"operationLogRetentionDays": "30"}}},
		{name: "over range", body: map[string]any{"logRetention": map[string]any{"apiCallLogRetentionDays": 3651}}},
		{name: "unknown nested field", body: map[string]any{"logRetention": map[string]any{"operationLogRetentionDays": 30, "extra": 1}}},
		{name: "empty object", body: map[string]any{"logRetention": map[string]any{}}},
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
			assertLogRetention(t, getResponse, ptrInt(30), nil, nil)
		})
	}

	if err := db.Model(&database.SystemSetting{}).
		Where("tenant_id = ? AND `key` = ?", adminSession.tenantID, settings.KeyLogRetention).
		Update("value_json", `{"operationLogRetentionDays":0}`).Error; err != nil {
		t.Fatalf("damage log retention setting: %v", err)
	}
	getMalformed := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if getMalformed.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed GET status = %d, want %d: %s", getMalformed.Code, http.StatusUnprocessableEntity, getMalformed.Body.String())
	}
	patchMalformed := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"logRetention": map[string]any{"operationLogRetentionDays": 31},
	}, adminSession.cookies, adminSession.csrfHeader())
	if patchMalformed.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed PATCH status = %d, want %d: %s", patchMalformed.Code, http.StatusUnprocessableEntity, patchMalformed.Body.String())
	}

	var logs int64
	if err := db.Model(&database.OperationLog{}).Where("tenant_id = ? AND action = ? AND resource_id = ?", adminSession.tenantID, settings.ActionUpdateSystemSettings, settings.KeyLogRetention).Count(&logs).Error; err != nil {
		t.Fatalf("count logRetention operation logs: %v", err)
	}
	if logs != 1 {
		t.Fatalf("logRetention operation logs = %d, want only initial successful update", logs)
	}
}

func TestSystemSettingsStorageRetentionRejectsInvalidAndDeferredFields(t *testing.T) {
	router, db, _, adminSession := newAssetRouteTestRouter(t, config.UploadConfig{MaxFileSizeBytes: 2048, MaxWidth: 10, MaxHeight: 10, MaxPixels: 100})

	initial := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"storageRetention": map[string]any{"deletedAssetRetentionDays": 30},
	}, adminSession.cookies, adminSession.csrfHeader())
	if initial.Code != http.StatusOK {
		t.Fatalf("initial storageRetention PATCH status = %d, want %d: %s", initial.Code, http.StatusOK, initial.Body.String())
	}

	cases := []struct {
		name string
		body any
	}{
		{name: "zero", body: map[string]any{"storageRetention": map[string]any{"deletedAssetRetentionDays": 0}}},
		{name: "negative", body: map[string]any{"storageRetention": map[string]any{"deletedAssetRetentionDays": -1}}},
		{name: "non integer", body: map[string]any{"storageRetention": map[string]any{"deletedAssetRetentionDays": 1.5}}},
		{name: "string", body: map[string]any{"storageRetention": map[string]any{"deletedAssetRetentionDays": "30"}}},
		{name: "over range", body: map[string]any{"storageRetention": map[string]any{"deletedAssetRetentionDays": 3651}}},
		{name: "unknown nested field", body: map[string]any{"storageRetention": map[string]any{"deletedAssetRetentionDays": 30, "extra": 1}}},
		{name: "empty slice", body: map[string]any{"storageRetention": map[string]any{}}},
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
			assertStorageRetention(t, getResponse, ptrInt(30))
		})
	}

	var logs int64
	if err := db.Model(&database.OperationLog{}).Where("tenant_id = ? AND action = ? AND resource_id = ?", adminSession.tenantID, settings.ActionUpdateSystemSettings, settings.KeyStorageRetention).Count(&logs).Error; err != nil {
		t.Fatalf("count storageRetention operation logs: %v", err)
	}
	if logs != 1 {
		t.Fatalf("storageRetention operation logs = %d, want only initial successful update", logs)
	}
}

func TestSystemSettingsTaskConcurrencyFallbackPatchPartialAndTenantIsolation(t *testing.T) {
	hardCaps := config.QueueConfig{
		TenantConcurrency:   8,
		UserConcurrency:     7,
		ProviderConcurrency: 6,
		ModelConcurrency:    5,
	}
	router, db, adminSession := newSystemSettingsRouteTestRouter(t, hardCaps)
	tenantBSession := seedTenantAdminSession(t, router, db, "tenant-b", "tenant-b-admin", "tenant-b-admin@example.com", "tenant-b-password-123")

	fallback := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if fallback.Code != http.StatusOK {
		t.Fatalf("fallback GET status = %d, want %d: %s", fallback.Code, http.StatusOK, fallback.Body.String())
	}
	assertTaskConcurrency(t, fallback, 8, 7, 6, 5)

	setResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskConcurrency": map[string]any{
			"tenantLimit":   4,
			"userLimit":     3,
			"providerLimit": 2,
			"modelLimit":    2,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setResponse.Code != http.StatusOK {
		t.Fatalf("set taskConcurrency status = %d, want %d: %s", setResponse.Code, http.StatusOK, setResponse.Body.String())
	}
	assertTaskConcurrency(t, setResponse, 4, 3, 2, 2)

	partialResponse := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskConcurrency": map[string]any{"modelLimit": 1},
	}, adminSession.cookies, adminSession.csrfHeader())
	if partialResponse.Code != http.StatusOK {
		t.Fatalf("partial taskConcurrency status = %d, want %d: %s", partialResponse.Code, http.StatusOK, partialResponse.Body.String())
	}
	assertTaskConcurrency(t, partialResponse, 4, 3, 2, 1)

	tenantBGet := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, tenantBSession.cookies, nil)
	if tenantBGet.Code != http.StatusOK {
		t.Fatalf("tenant B GET status = %d, want %d: %s", tenantBGet.Code, http.StatusOK, tenantBGet.Body.String())
	}
	assertTaskConcurrency(t, tenantBGet, 8, 7, 6, 5)

	var logs []database.OperationLog
	if err := db.Where("tenant_id = ? AND action = ? AND resource_id = ?", adminSession.tenantID, settings.ActionUpdateSystemSettings, "task_concurrency").Find(&logs).Error; err != nil {
		t.Fatalf("load taskConcurrency operation logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("taskConcurrency operation logs = %d, want 2: %#v", len(logs), logs)
	}
	for _, log := range logs {
		metadata := strings.ToLower(log.MetadataJSON)
		if !strings.Contains(metadata, "task_concurrency") || !strings.Contains(metadata, "changedfields") {
			t.Fatalf("operation log missing sanitized concurrency metadata: %s", log.MetadataJSON)
		}
		for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "secret", "jwt", "base64", "data:image", "value_json"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %s", forbidden, log.MetadataJSON)
			}
		}
	}
}

func TestSystemSettingsTaskConcurrencyDefaultsAreNotRuntimeHardCaps(t *testing.T) {
	queueConfig := config.QueueConfig{
		GlobalConcurrency:    8,
		PolicyMaxConcurrency: 8,
		TenantConcurrency:    2,
		UserConcurrency:      2,
		ProviderConcurrency:  2,
		ModelConcurrency:     2,
	}
	router, _, adminSession := newSystemSettingsRouteTestRouter(t, queueConfig)

	fallback := performJSON(router, http.MethodGet, "/api/v1/admin/system-settings", nil, adminSession.cookies, nil)
	if fallback.Code != http.StatusOK {
		t.Fatalf("fallback GET status = %d, want %d: %s", fallback.Code, http.StatusOK, fallback.Body.String())
	}
	assertTaskConcurrency(t, fallback, 2, 2, 2, 2)
	constraints := objectField(t, decodeData(t, fallback), "constraints")
	concurrencyConstraints := objectField(t, constraints, "taskConcurrency")
	assertNumericField(t, concurrencyConstraints, "globalCapacity", 8)
	assertIntegerRange(t, objectField(t, concurrencyConstraints, "tenantLimit"), 1, 8)
	assertIntegerRange(t, objectField(t, concurrencyConstraints, "userLimit"), 1, 8)
	assertIntegerRange(t, objectField(t, concurrencyConstraints, "providerLimit"), 1, 8)
	assertIntegerRange(t, objectField(t, concurrencyConstraints, "modelLimit"), 1, 8)

	updated := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskConcurrency": map[string]any{
			"tenantLimit":   8,
			"userLimit":     6,
			"providerLimit": 5,
			"modelLimit":    4,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if updated.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, want %d: %s", updated.Code, http.StatusOK, updated.Body.String())
	}
	assertTaskConcurrency(t, updated, 8, 6, 5, 4)
}

func TestSystemSettingsTaskConcurrencyRejectsInvalidFieldsAndHardCapViolations(t *testing.T) {
	hardCaps := config.QueueConfig{
		TenantConcurrency:   8,
		UserConcurrency:     7,
		ProviderConcurrency: 6,
		ModelConcurrency:    5,
	}
	router, db, adminSession := newSystemSettingsRouteTestRouter(t, hardCaps)
	initial := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskConcurrency": map[string]any{
			"tenantLimit":   4,
			"userLimit":     3,
			"providerLimit": 2,
			"modelLimit":    1,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if initial.Code != http.StatusOK {
		t.Fatalf("initial taskConcurrency PATCH status = %d, want %d: %s", initial.Code, http.StatusOK, initial.Body.String())
	}

	cases := []struct {
		name string
		body any
	}{
		{name: "zero", body: map[string]any{"taskConcurrency": map[string]any{"tenantLimit": 0}}},
		{name: "negative", body: map[string]any{"taskConcurrency": map[string]any{"userLimit": -1}}},
		{name: "non integer", body: map[string]any{"taskConcurrency": map[string]any{"providerLimit": 1.5}}},
		{name: "string", body: map[string]any{"taskConcurrency": map[string]any{"modelLimit": "1"}}},
		{name: "unknown nested field", body: map[string]any{"taskConcurrency": map[string]any{"extra": 1}}},
		{name: "global field", body: map[string]any{"taskConcurrency": map[string]any{"globalLimit": 1}}},
		{name: "empty slice", body: map[string]any{"taskConcurrency": map[string]any{}}},
		{name: "tenant over cap", body: map[string]any{"taskConcurrency": map[string]any{"tenantLimit": 9}}},
		{name: "user over cap", body: map[string]any{"taskConcurrency": map[string]any{"userLimit": 9}}},
		{name: "provider over cap", body: map[string]any{"taskConcurrency": map[string]any{"providerLimit": 9}}},
		{name: "model over cap", body: map[string]any{"taskConcurrency": map[string]any{"modelLimit": 9}}},
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
			assertTaskConcurrency(t, getResponse, 4, 3, 2, 1)
		})
	}

	var logs int64
	if err := db.Model(&database.OperationLog{}).Where("tenant_id = ? AND action = ? AND resource_id = ?", adminSession.tenantID, settings.ActionUpdateSystemSettings, "task_concurrency").Count(&logs).Error; err != nil {
		t.Fatalf("count taskConcurrency operation logs: %v", err)
	}
	if logs != 1 {
		t.Fatalf("taskConcurrency operation logs = %d, want only initial successful update", logs)
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
	deletedProviderID, deletedProviderModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "settings-deleted-provider", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, deletedProviderID).Delete(&database.AIProvider{}).Error; err != nil {
		t.Fatalf("soft delete settings provider: %v", err)
	}
	deletedModelProviderID, deletedModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "settings-deleted-model", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, deletedModelID).Delete(&database.AIModel{}).Error; err != nil {
		t.Fatalf("soft delete settings model: %v", err)
	}
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
		{name: "deleted provider", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": deletedProviderID, "defaultModelId": deletedProviderModelID}}},
		{name: "disabled model", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": enabledProviderID, "defaultModelId": disabledModelID}}},
		{name: "deleted model", body: map[string]any{"taskDefaults": map[string]any{"defaultProviderId": deletedModelProviderID, "defaultModelId": deletedModelID}}},
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
			assertResponseExcludes(t, response.Body.String(), "provider-tenant-b", "model-tenant-b", disabledProviderID, disabledModelID, deletedProviderID, deletedModelID)
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

func assertTaskConcurrency(t *testing.T, response *httptest.ResponseRecorder, tenantLimit int, userLimit int, providerLimit int, modelLimit int) {
	t.Helper()

	data := decodeData(t, response)
	policy := objectField(t, data, "taskConcurrency")
	assertNumericField(t, policy, "tenantLimit", int64(tenantLimit))
	assertNumericField(t, policy, "userLimit", int64(userLimit))
	assertNumericField(t, policy, "providerLimit", int64(providerLimit))
	assertNumericField(t, policy, "modelLimit", int64(modelLimit))
}

func assertSystemSettingsConstraints(t *testing.T, response *httptest.ResponseRecorder, upload config.UploadConfig) {
	t.Helper()

	data := decodeData(t, response)
	constraints := objectField(t, data, "constraints")
	uploadConstraints := objectField(t, constraints, "uploadPolicy")
	assertIntegerRange(t, objectField(t, uploadConstraints, "maxFileSizeBytes"), 1, upload.MaxFileSizeBytes)
	assertIntegerRange(t, objectField(t, uploadConstraints, "maxWidth"), 1, int64(upload.MaxWidth))
	assertIntegerRange(t, objectField(t, uploadConstraints, "maxHeight"), 1, int64(upload.MaxHeight))
	assertIntegerRange(t, objectField(t, uploadConstraints, "maxPixels"), 1, upload.MaxPixels)

	concurrencyConstraints := objectField(t, constraints, "taskConcurrency")
	assertPositiveIntegerRange(t, objectField(t, concurrencyConstraints, "tenantLimit"))
	assertPositiveIntegerRange(t, objectField(t, concurrencyConstraints, "userLimit"))
	assertPositiveIntegerRange(t, objectField(t, concurrencyConstraints, "providerLimit"))
	assertPositiveIntegerRange(t, objectField(t, concurrencyConstraints, "modelLimit"))
	assertIntegerRange(t, objectField(t, constraints, "storageRetention"), 1, 3650)
	assertIntegerRange(t, objectField(t, constraints, "storageQuota"), 1, 109951162777600)
	assertIntegerRange(t, objectField(t, constraints, "logRetention"), 1, 3650)
}

func assertIntegerRange(t *testing.T, value map[string]any, min int64, max int64) {
	t.Helper()
	assertNumericField(t, value, "min", min)
	assertNumericField(t, value, "max", max)
}

func assertPositiveIntegerRange(t *testing.T, value map[string]any) {
	t.Helper()
	assertNumericField(t, value, "min", 1)
	maximum, ok := value["max"].(float64)
	if !ok || maximum < 1 {
		t.Fatalf("max = %#v, want a positive number", value["max"])
	}
}

func assertStorageRetention(t *testing.T, response *httptest.ResponseRecorder, expectedDays *int) {
	t.Helper()

	data := decodeData(t, response)
	retention := objectField(t, data, "storageRetention")
	if expectedDays == nil {
		if retention["deletedAssetRetentionDays"] != nil {
			t.Fatalf("deletedAssetRetentionDays = %#v, want null", retention["deletedAssetRetentionDays"])
		}
		return
	}
	assertNumericField(t, retention, "deletedAssetRetentionDays", int64(*expectedDays))
}

func assertStorageQuota(t *testing.T, response *httptest.ResponseRecorder, expectedMaxBytes *int64, expectedUsedBytes int64) {
	t.Helper()

	data := decodeData(t, response)
	quota := objectField(t, data, "storageQuota")
	if expectedMaxBytes == nil {
		if quota["maxBytes"] != nil {
			t.Fatalf("maxBytes = %#v, want null", quota["maxBytes"])
		}
	} else {
		assertNumericField(t, quota, "maxBytes", *expectedMaxBytes)
	}
	assertNumericField(t, quota, "usedBytes", expectedUsedBytes)
}

func assertLogRetention(t *testing.T, response *httptest.ResponseRecorder, expectedOperationDays *int, expectedAPICallDays *int, expectedTaskEventDays *int) {
	t.Helper()

	data := decodeData(t, response)
	retention := objectField(t, data, "logRetention")
	assertNullableIntField(t, retention, "operationLogRetentionDays", expectedOperationDays)
	assertNullableIntField(t, retention, "apiCallLogRetentionDays", expectedAPICallDays)
	assertNullableIntField(t, retention, "taskEventRetentionDays", expectedTaskEventDays)
}

func assertNullableIntField(t *testing.T, object map[string]any, field string, expected *int) {
	t.Helper()
	if expected == nil {
		if object[field] != nil {
			t.Fatalf("%s = %#v, want null", field, object[field])
		}
		return
	}
	assertNumericField(t, object, field, int64(*expected))
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

func ptrInt(value int) *int {
	return &value
}

func ptrInt64(value int64) *int64 {
	return &value
}

func seedQuotaAsset(t *testing.T, db *gorm.DB, tenantID string, assetID string, sizeBytes int64, deletedAt *time.Time, purgedAt *time.Time) {
	t.Helper()

	now := time.Now().UTC()
	record := database.ImageAsset{
		ID:        assetID,
		TenantID:  tenantID,
		ProjectID: "project-" + assetID,
		Kind:      "REFERENCE",
		Filename:  assetID + ".png",
		ObjectKey: "test/" + tenantID + "/" + assetID + ".png",
		MimeType:  "image/png",
		SizeBytes: sizeBytes,
		Width:     1,
		Height:    1,
		SHA256:    strings.Repeat("a", 64),
		CreatedBy: "user-" + assetID,
		CreatedAt: now,
		UpdatedAt: now,
		PurgedAt:  purgedAt,
	}
	if deletedAt != nil {
		record.DeletedAt.Valid = true
		record.DeletedAt.Time = deletedAt.UTC()
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed quota asset %s/%s: %v", tenantID, assetID, err)
	}
}

func newSystemSettingsRouteTestRouter(t *testing.T, hardCaps config.QueueConfig) (http.Handler, *gorm.DB, projectRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	cfg := authRouteTestConfig("test")
	cfg.Storage = config.DefaultStorageConfig()
	cfg.Upload = config.NormalizeUploadConfig(config.UploadConfig{})
	cfg.Queue = hardCaps
	router := NewRouter(RouterOptions{
		Config:      cfg,
		Logger:      discardLogger(),
		Database:    db,
		ObjectStore: newFakeObjectStore(),
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
	return router, db, projectRouteSession{
		tenantID: nestedString(t, data, "tenant", "id"),
		userID:   nestedString(t, data, "user", "id"),
		cookies:  []*http.Cookie{authCookie, csrfCookie},
		csrf:     csrfCookie.Value,
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
