package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"gorm.io/gorm"
)

// newDiagnosticsTestRouter creates a router with a fake queue depth inspector.
func newDiagnosticsTestRouter(t *testing.T, inspector queue.QueueDepthInspector) (http.Handler, *gorm.DB, projectRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	router := NewRouter(RouterOptions{
		Config:              authRouteTestConfig("test"),
		Logger:              discardLogger(),
		Database:            db,
		QueueDepthInspector: inspector,
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

func TestAdminDiagnosticsSummaryRejectsUnauthenticated(t *testing.T) {
	router, _, _ := newDiagnosticsTestRouter(t, nil)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, nil, nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d: %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
}

func TestAdminDiagnosticsSummaryRejectsNonAdmin(t *testing.T) {
	router, db, adminSession := newDiagnosticsTestRouter(t, nil)
	seedActiveUser(t, db, adminSession.tenantID, "seller-diag", "seller-diag@example.com", "Seller Diag", "seller-diag-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-diag", "seller")
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-diag@example.com", "seller-diag-password-123")

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, sellerSession.cookies, nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-admin status = %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
}

func TestAdminDiagnosticsSummaryDefaultResponse(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available", Pending: 3, Processing: 1, Delayed: 0, Dead: 0},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed some tasks
	seedDiagnosticsTask(t, db, adminSession.tenantID, "task-success-1", task.StatusSucceeded, "", "")
	seedDiagnosticsTask(t, db, adminSession.tenantID, "task-success-2", task.StatusSucceeded, "", "")
	seedDiagnosticsTask(t, db, adminSession.tenantID, "task-failed-1", task.StatusFailed, "PROVIDER_ERROR", "Provider returned 500")
	seedDiagnosticsTask(t, db, adminSession.tenantID, "task-queued-1", task.StatusQueued, "", "")

	// Seed API call logs
	seedDiagnosticsAPICallLog(t, db, adminSession.tenantID, "api-call-1", "provider-a", "SUCCESS")
	seedDiagnosticsAPICallLog(t, db, adminSession.tenantID, "api-call-2", "provider-a", "FAILURE")
	seedDiagnosticsAPICallLog(t, db, adminSession.tenantID, "api-call-3", "provider-b", "SUCCESS")

	// Seed image assets
	seedDiagnosticsAsset(t, db, adminSession.tenantID, "asset-active-1", false, false)
	seedDiagnosticsAsset(t, db, adminSession.tenantID, "asset-deleted-1", true, false)
	seedDiagnosticsAsset(t, db, adminSession.tenantID, "asset-purged-1", true, true)

	// Seed maintenance operation logs
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-orphan-1", "storage.orphan_cleanup", `{"scanned":100,"candidates":5,"deleted":3,"status":"completed","objectKeys":["leak/this"],"bucketName":"secret-bucket"}`)
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-log-1", "log_retention.cleanup", `{"processed":50,"deleted":45,"status":"completed","rawErrors":"should-be-stripped"}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	data := decodeData(t, response)

	// Verify tasks section
	tasks := objectField(t, data, "tasks")
	statusCounts := objectField(t, tasks, "statusCounts")
	assertFloatField(t, statusCounts, "SUCCEEDED", 2)
	assertFloatField(t, statusCounts, "FAILED", 1)
	assertFloatField(t, statusCounts, "QUEUED", 1)
	assertFloatField(t, statusCounts, "RUNNING", 0)
	assertFloatField(t, statusCounts, "CANCELLED", 0)
	assertFloatField(t, statusCounts, "RETRYING", 0)
	assertFloatField(t, statusCounts, "TIMED_OUT", 0)
	assertFloatField(t, tasks, "totalTasks", 4)

	recentFailures, ok := tasks["recentFailures"].([]any)
	if !ok {
		t.Fatalf("recentFailures is not an array: %#v", tasks["recentFailures"])
	}
	if len(recentFailures) != 1 {
		t.Fatalf("recentFailures len = %d, want 1", len(recentFailures))
	}
	failure := recentFailures[0].(map[string]any)
	if stringField(t, failure, "taskId") != "task-failed-1" {
		t.Fatalf("failure taskId = %q, want task-failed-1", stringField(t, failure, "taskId"))
	}
	if stringField(t, failure, "errorCode") != "PROVIDER_ERROR" {
		t.Fatalf("failure errorCode = %q, want PROVIDER_ERROR", stringField(t, failure, "errorCode"))
	}

	// Verify queue section
	queueSection := objectField(t, data, "queue")
	if stringField(t, queueSection, "status") != "available" {
		t.Fatalf("queue status = %q, want available", stringField(t, queueSection, "status"))
	}
	assertFloatField(t, queueSection, "pending", 3)
	assertFloatField(t, queueSection, "processing", 1)

	// Verify providers section
	providers := objectField(t, data, "providers")
	assertFloatField(t, providers, "windowHours", 24)
	assertFloatField(t, providers, "totalCalls", 3)
	assertFloatField(t, providers, "successCount", 2)
	assertFloatField(t, providers, "failureCount", 1)

	byProvider, ok := providers["byProvider"].([]any)
	if !ok {
		t.Fatalf("byProvider is not an array: %#v", providers["byProvider"])
	}
	if len(byProvider) == 0 {
		t.Fatal("byProvider is empty")
	}

	// Verify storage section
	storage := objectField(t, data, "storage")
	assertFloatField(t, storage, "totalAssets", 3)
	assertFloatField(t, storage, "activeAssets", 1)
	assertFloatField(t, storage, "softDeletedAssets", 1)
	assertFloatField(t, storage, "purgedAssets", 1)

	// Verify maintenance section
	maintenance := objectField(t, data, "maintenance")
	recentOps, ok := maintenance["recentOperations"].([]any)
	if !ok {
		t.Fatalf("recentOperations is not an array: %#v", maintenance["recentOperations"])
	}
	if len(recentOps) != 2 {
		t.Fatalf("recentOperations len = %d, want 2", len(recentOps))
	}

	// Verify generatedAt exists
	if _, ok := data["generatedAt"].(string); !ok {
		t.Fatal("generatedAt is missing or not a string")
	}

	// Security: verify no sensitive data leaked
	body := response.Body.String()
	assertResponseExcludes(t, body,
		"leak/this",           // object key
		"secret-bucket",       // bucket name
		"objectKeys",          // raw metadata field
		"bucketName",          // raw metadata field
		"rawErrors",           // raw metadata field
		"should-be-stripped",  // unsanitized value
	)
}

func TestAdminDiagnosticsSummaryTenantIsolation(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed cross-tenant data
	seedOtherTenantProject(t, db)
	seedDiagnosticsTask(t, db, "tenant-b", "task-tenant-b-1", task.StatusFailed, "CROSS_TENANT", "Cross tenant error")
	seedDiagnosticsAPICallLog(t, db, "tenant-b", "api-call-tenant-b", "provider-tenant-b", "FAILURE")
	seedDiagnosticsAsset(t, db, "tenant-b", "asset-tenant-b", false, false)
	seedDiagnosticsMaintenanceLog(t, db, "tenant-b", "maint-tenant-b", "storage.orphan_cleanup", `{"scanned":999}`)

	// Seed admin-tenant data
	seedDiagnosticsTask(t, db, adminSession.tenantID, "task-admin-1", task.StatusSucceeded, "", "")

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"tenant-b",
		"task-tenant-b",
		"provider-tenant-b",
		"asset-tenant-b",
		"CROSS_TENANT",
		"Cross tenant error",
	)

	// Verify admin tenant data is present
	data := decodeData(t, response)
	tasks := objectField(t, data, "tasks")
	assertFloatField(t, tasks, "totalTasks", 1)
}

func TestAdminDiagnosticsSummaryQueueUnavailable(t *testing.T) {
	// nil inspector results in NilQueueDepthInspector which returns unavailable
	router, _, adminSession := newDiagnosticsTestRouter(t, nil)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	data := decodeData(t, response)
	queueSection := objectField(t, data, "queue")
	if stringField(t, queueSection, "status") != "unavailable" {
		t.Fatalf("queue status = %q, want unavailable", stringField(t, queueSection, "status"))
	}
	if stringField(t, queueSection, "reason") != "queue_unavailable" {
		t.Fatalf("queue reason = %q, want queue_unavailable", stringField(t, queueSection, "reason"))
	}
}

func TestAdminDiagnosticsSummaryProviderTotalNotTruncatedByLimit(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed 3 providers with varying call counts
	seedDiagnosticsAPICallLog(t, db, adminSession.tenantID, "api-prov-a-1", "provider-a", "SUCCESS")
	seedDiagnosticsAPICallLog(t, db, adminSession.tenantID, "api-prov-a-2", "provider-a", "FAILURE")
	seedDiagnosticsAPICallLog(t, db, adminSession.tenantID, "api-prov-b-1", "provider-b", "SUCCESS")
	seedDiagnosticsAPICallLog(t, db, adminSession.tenantID, "api-prov-c-1", "provider-c", "FAILURE")

	// Query with limit=1 — byProvider should have 1 entry, but top-level totals should reflect all 4 calls
	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary?limit=1", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	data := decodeData(t, response)
	providers := objectField(t, data, "providers")
	assertFloatField(t, providers, "totalCalls", 4)
	assertFloatField(t, providers, "failureCount", 2)

	byProvider, ok := providers["byProvider"].([]any)
	if !ok {
		t.Fatalf("byProvider is not an array: %#v", providers["byProvider"])
	}
	if len(byProvider) != 1 {
		t.Fatalf("byProvider len = %d, want 1 (limit=1)", len(byProvider))
	}
}

func TestAdminDiagnosticsSummaryMaintenanceErrorsFieldBlocked(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed maintenance log with errors field containing sensitive data
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-errors", "storage.orphan_cleanup",
		`{"errors":["Authorization: Bearer sk-secret-12345 objectKey: tenant/project/abc.jpg"],"scanned":10,"deleted":2}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"sk-secret-12345",
		"abc.jpg",
		"Bearer",
	)
	// Verify scanned/deleted are still present
	if !strings.Contains(body, `"scanned"`) {
		t.Fatal("scanned should still be present")
	}
}

func TestAdminDiagnosticsSummaryMaintenanceNestedUnsafeKeyBlocked(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed maintenance log with nested map under non-whitelisted key
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-nested", "storage.orphan_cleanup",
		`{"Authorization":{"token":"Bearer secret"},"nested":{"processed":10,"bucketName":"inner-bucket"},"scanned":5}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"inner-bucket",
		"nested",
		"Bearer",
		"Authorization",
	)
}

func TestAdminDiagnosticsSummaryMaintenanceStringSanitization(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed with completedAt containing a URL — not a valid RFC3339, will be dropped
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-str-sens", "storage.orphan_cleanup",
		`{"status":"completed","completedAt":"http://minio:9000/leaked-url"}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"minio:9000",
		"leaked-url",
	)
	// completedAt should be entirely dropped (not redacted) because it's not valid RFC3339
	if strings.Contains(body, `"completedAt"`) {
		t.Fatal("invalid completedAt should be dropped entirely, not included in response")
	}
}

func TestAdminDiagnosticsSummaryInvalidWindowHours(t *testing.T) {
	router, _, adminSession := newDiagnosticsTestRouter(t, nil)

	for _, raw := range []string{"0", "-1", "721", "abc", "999999"} {
		response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary?windowHours="+raw, nil, adminSession.cookies, nil)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("windowHours=%s status = %d, want %d: %s", raw, response.Code, http.StatusUnprocessableEntity, response.Body.String())
		}
	}
}

func TestAdminDiagnosticsSummaryInvalidLimit(t *testing.T) {
	router, _, adminSession := newDiagnosticsTestRouter(t, nil)

	for _, raw := range []string{"0", "-1", "51", "abc"} {
		response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary?limit="+raw, nil, adminSession.cookies, nil)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("limit=%s status = %d, want %d: %s", raw, response.Code, http.StatusUnprocessableEntity, response.Body.String())
		}
	}
}

func TestAdminDiagnosticsSummaryCustomWindowAndLimit(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed a recent and an old API call log
	now := time.Now().UTC()
	seedDiagnosticsAPICallLogAt(t, db, adminSession.tenantID, "api-recent", "provider-a", "SUCCESS", now.Add(-time.Hour))
	seedDiagnosticsAPICallLogAt(t, db, adminSession.tenantID, "api-old", "provider-a", "FAILURE", now.Add(-48*time.Hour))

	// Query with 2 hour window - should only see the recent one
	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary?windowHours=2&limit=5", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	data := decodeData(t, response)
	providers := objectField(t, data, "providers")
	assertFloatField(t, providers, "windowHours", 2)
	assertFloatField(t, providers, "totalCalls", 1)
	assertFloatField(t, providers, "failureCount", 0)
}

func TestAdminDiagnosticsSummaryEmptyDatabase(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, _, adminSession := newDiagnosticsTestRouter(t, inspector)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	data := decodeData(t, response)

	tasks := objectField(t, data, "tasks")
	assertFloatField(t, tasks, "totalTasks", 0)
	recentFailures, ok := tasks["recentFailures"].([]any)
	if !ok || len(recentFailures) != 0 {
		t.Fatalf("empty recentFailures = %#v", tasks["recentFailures"])
	}

	providers := objectField(t, data, "providers")
	assertFloatField(t, providers, "totalCalls", 0)
	assertFloatField(t, providers, "failureRate", 0)

	storage := objectField(t, data, "storage")
	assertFloatField(t, storage, "totalAssets", 0)

	maintenance := objectField(t, data, "maintenance")
	recentOps := maintenance["recentOperations"].([]any)
	if len(recentOps) != 0 {
		t.Fatalf("empty recentOperations = %#v", maintenance["recentOperations"])
	}
}

func TestAdminDiagnosticsSummaryMaintenanceMetadataWhitelist(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed maintenance log with both safe and unsafe fields
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-whitelist", "storage.orphan_cleanup",
		`{
			"scanned": 100,
			"candidates": 5,
			"deleted": 3,
			"status": "completed",
			"dryRun": false,
			"objectKeys": ["tenant/project/abc.jpg", "tenant/project/def.jpg"],
			"bucketName": "studio-assets",
			"minioUrl": "http://minio:9000/studio-assets",
			"signedUrl": "http://minio:9000/signed/abc?token=secret",
			"Authorization": "Bearer secret-token",
			"apiKey": "sk-secret-key-12345",
			"nested": {"processed": 10, "bucketName": "inner-bucket"}
		}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"abc.jpg",
		"def.jpg",
		"studio-assets",
		"minio:9000",
		"signed/abc",
		"secret-token",
		"sk-secret-key",
		"inner-bucket",
		"objectKeys",
		"bucketName",
		"minioUrl",
		"signedUrl",
		"Authorization",
		"apiKey",
	)

	// Verify safe fields ARE present
	if !strings.Contains(body, `"scanned"`) {
		t.Fatal("response should contain scanned field")
	}
	if !strings.Contains(body, `"candidates"`) {
		t.Fatal("response should contain candidates field")
	}
	if !strings.Contains(body, `"deleted"`) {
		t.Fatal("response should contain deleted field")
	}
	if !strings.Contains(body, `"status"`) {
		t.Fatal("response should contain status field")
	}
}

func TestAdminDiagnosticsSummaryErrorMessageSanitization(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// Seed a task with a very long error message
	longError := strings.Repeat("a", 500)
	seedDiagnosticsTask(t, db, adminSession.tenantID, "task-long-error", task.StatusFailed, "LONG_ERROR", longError)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	data := decodeData(t, response)
	tasks := objectField(t, data, "tasks")
	recentFailures := tasks["recentFailures"].([]any)
	if len(recentFailures) != 1 {
		t.Fatalf("recentFailures len = %d, want 1", len(recentFailures))
	}
	failure := recentFailures[0].(map[string]any)
	errorMessage := stringField(t, failure, "errorMessage")
	if len([]rune(errorMessage)) > 200 {
		t.Fatalf("errorMessage rune count = %d, want <= 200", len([]rune(errorMessage)))
	}
}

func TestAdminDiagnosticsSummaryCompletedAtObjectKeyDropped(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// completedAt with an object key path — not RFC3339, must be dropped
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-objkey", "storage.orphan_cleanup",
		`{"scanned":10,"completedAt":"tenant-a/project-x/assets/abc.jpg","status":"completed"}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"tenant-a",
		"project-x",
		"abc.jpg",
	)
	if strings.Contains(body, `"completedAt"`) {
		t.Fatal("completedAt with object key should be dropped, not included")
	}
	// scanned should still be present as a numeric field
	if !strings.Contains(body, `"scanned"`) {
		t.Fatal("scanned should still be present")
	}
}

func TestAdminDiagnosticsSummaryCompletedAtUnknownSecretDropped(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// completedAt with a secret-like string that doesn't match old markers
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-secret", "storage.orphan_cleanup",
		`{"scanned":5,"completedAt":"sk-live-abc123xyz","status":"completed"}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"sk-live-abc123xyz",
	)
	if strings.Contains(body, `"completedAt"`) {
		t.Fatal("completedAt with unknown secret should be dropped, not included")
	}
}

func TestAdminDiagnosticsSummaryCompletedAtValidRFC3339Preserved(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	validTimestamp := "2025-06-15T10:30:00Z"
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-valid-ts", "storage.orphan_cleanup",
		`{"scanned":5,"completedAt":"`+validTimestamp+`","status":"completed"}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	if !strings.Contains(body, validTimestamp) {
		t.Fatalf("valid RFC3339 completedAt should be preserved, body: %s", body)
	}
}

func TestAdminDiagnosticsSummaryNumericFieldRejectsString(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// "scanned" is a numeric field — string value should be silently dropped
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-numeric-str", "storage.orphan_cleanup",
		`{"scanned":"tenant/project/leak.jpg","deleted":3,"status":"completed"}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"leak.jpg",
		"tenant/project",
	)
	// deleted (numeric) should still be present
	if !strings.Contains(body, `"deleted"`) {
		t.Fatal("deleted should still be present")
	}
}

func TestAdminDiagnosticsSummaryStatusRejectsNonEnum(t *testing.T) {
	inspector := &fakeQueueDepthInspector{
		result: queue.QueueDepth{Status: "available"},
	}
	router, db, adminSession := newDiagnosticsTestRouter(t, inspector)

	// "status" with a non-enum string — must be dropped
	seedDiagnosticsMaintenanceLog(t, db, adminSession.tenantID, "maint-bad-status", "storage.orphan_cleanup",
		`{"scanned":1,"status":"completed at tenant/project/abc.jpg"}`)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/diagnostics/summary", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := response.Body.String()
	assertResponseExcludes(t, body,
		"abc.jpg",
		"tenant/project",
	)
}

// --- Test helpers ---

type fakeQueueDepthInspector struct {
	result queue.QueueDepth
}

func (f *fakeQueueDepthInspector) Inspect(_ context.Context) queue.QueueDepth {
	return f.result
}

func seedDiagnosticsTask(t *testing.T, db *gorm.DB, tenantID string, taskID string, status string, errorCode string, errorMessage string) {
	t.Helper()
	now := time.Now().UTC()
	var finishedAt *time.Time
	if status == task.StatusFailed || status == task.StatusSucceeded || status == task.StatusTimedOut {
		finished := now
		finishedAt = &finished
	}
	if err := db.Create(&database.GenerationTask{
		ID:                taskID,
		TenantID:          tenantID,
		ProjectID:         "project-diag",
		Type:              task.TypeImageGeneration,
		ProviderID:        "provider-diag",
		ModelID:           "model-diag",
		Status:            status,
		Prompt:            "test prompt",
		ImageType:         "MAIN",
		ParamsJSON:        `{}`,
		InputAssetIDsJSON: `[]`,
		Attempt:           1,
		MaxAttempts:       3,
		CreatedBy:         "user-diag",
		ErrorCode:         errorCode,
		ErrorMessage:      errorMessage,
		FinishedAt:        finishedAt,
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("seed diagnostics task %s: %v", taskID, err)
	}
}

func seedDiagnosticsAPICallLog(t *testing.T, db *gorm.DB, tenantID string, logID string, providerID string, status string) {
	t.Helper()
	seedDiagnosticsAPICallLogAt(t, db, tenantID, logID, providerID, status, time.Now().UTC())
}

func seedDiagnosticsAPICallLogAt(t *testing.T, db *gorm.DB, tenantID string, logID string, providerID string, status string, createdAt time.Time) {
	t.Helper()
	if err := db.Create(&database.APICallLog{
		ID:                   logID,
		TenantID:             tenantID,
		TaskID:               "task-" + logID,
		ProviderID:           providerID,
		ModelID:              "model-diag",
		Status:               status,
		DurationMs:           100,
		RequestID:            "req-" + logID,
		ErrorCode:            "",
		ErrorMessage:         "",
		RedactedRequestJSON:  `{}`,
		RedactedResponseJSON: `{}`,
		CreatedAt:            createdAt,
	}).Error; err != nil {
		t.Fatalf("seed diagnostics api call log %s: %v", logID, err)
	}
}

func seedDiagnosticsAsset(t *testing.T, db *gorm.DB, tenantID string, assetID string, deleted bool, purged bool) {
	t.Helper()
	now := time.Now().UTC()
	asset := database.ImageAsset{
		ID:        assetID,
		TenantID:  tenantID,
		ProjectID: "project-diag",
		Kind:      "generated",
		Category:  "output",
		Filename:  "test.jpg",
		ObjectKey: tenantID + "/project-diag/" + assetID + ".jpg",
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
		Width:     100,
		Height:    100,
		SHA256:    "abc123",
		CreatedBy: "user-diag",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if deleted {
		deletedAt := now
		asset.DeletedAt = gorm.DeletedAt{Time: deletedAt, Valid: true}
	}
	if purged {
		purgedAt := now
		asset.PurgedAt = &purgedAt
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("seed diagnostics asset %s: %v", assetID, err)
	}
}

func seedDiagnosticsMaintenanceLog(t *testing.T, db *gorm.DB, tenantID string, logID string, action string, metadataJSON string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&database.OperationLog{
		ID:           logID,
		TenantID:     tenantID,
		Action:       action,
		ResourceType: "system",
		ResourceID:   "maintenance",
		IP:           "127.0.0.1",
		UserAgent:    "worker",
		MetadataJSON: metadataJSON,
		CreatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed diagnostics maintenance log %s: %v", logID, err)
	}
}
