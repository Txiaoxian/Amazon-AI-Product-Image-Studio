package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"gorm.io/gorm"
)

func TestAdminAuditUsageReadRoutesRejectNonAdmin(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	seedActiveUser(t, db, adminSession.tenantID, "seller-audit", "seller-audit@example.com", "Seller Audit", "seller-audit-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-audit", "seller")
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-audit@example.com", "seller-audit-password-123")

	for _, path := range []string{
		"/api/v1/admin/usage/summary",
		"/api/v1/admin/usage/records",
		"/api/v1/admin/operation-logs",
		"/api/v1/admin/api-call-logs",
		"/api/v1/admin/api-call-logs/api-log-a",
	} {
		response := performJSON(router, http.MethodGet, path, nil, sellerSession.cookies, nil)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d, want %d: %s", path, response.Code, http.StatusForbidden, response.Body.String())
		}
	}
}

func TestAdminUsageRecordsListTenantIsolationPaginationValidationAndRedaction(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	seedUsageRecord(t, db, usageSeed{
		ID:            "usage-a-new",
		TenantID:      adminSession.tenantID,
		TaskID:        "task-a-new",
		UserID:        adminSession.userID,
		ProjectID:     "project-a",
		ProviderID:    "provider-a",
		ModelID:       "model-a",
		InputTokens:   11,
		OutputTokens:  22,
		ImageCount:    1,
		EstimatedCost: "0.11000000",
		RawUsageJSON:  `{"safe":"ok","headers":{"Authorization":"Bearer usage-token"},"nested":[{"api_key":"sk-usage-secret-001"},{"note":"contains sk-usage-secret-002"}],"image":{"b64_json":"raw-image-base64"}}`,
		CreatedAt:     now.Add(time.Minute),
	})
	seedUsageRecord(t, db, usageSeed{
		ID:            "usage-a-old",
		TenantID:      adminSession.tenantID,
		TaskID:        "task-a-old",
		UserID:        adminSession.userID,
		ProjectID:     "project-a",
		ProviderID:    "provider-a",
		ModelID:       "model-a",
		InputTokens:   3,
		OutputTokens:  4,
		ImageCount:    2,
		EstimatedCost: "0.22000000",
		RawUsageJSON:  `{"safe":"older"}`,
		CreatedAt:     now.Add(-time.Minute),
	})
	seedUsageRecord(t, db, usageSeed{
		ID:            "usage-tenant-b",
		TenantID:      "tenant-b",
		TaskID:        "task-b",
		UserID:        "user-b",
		ProjectID:     "project-b",
		ProviderID:    "provider-b",
		ModelID:       "model-b",
		InputTokens:   999,
		OutputTokens:  999,
		ImageCount:    9,
		EstimatedCost: "9.99000000",
		RawUsageJSON:  `{"safe":"tenant-b","apiKey":"sk-cross-tenant"}`,
		CreatedAt:     now.Add(2 * time.Minute),
	})

	response := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?pageNum=1&pageSize=1&providerId=provider-a&createdAtFrom=2026-05-18T09:00:00Z&createdAtTo=2026-05-18T11:00:00Z", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("usage records status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertResponseExcludes(t, response.Body.String(), "sk-usage-secret", "usage-token", "raw-image-base64", "authorization", "api_key", "b64_json", "tenant-b")
	data := decodeData(t, response)
	assertPageMeta(t, data, 2, 1, 1)
	records := recordsField(t, data)
	if len(records) != 1 {
		t.Fatalf("usage page len = %d, want 1", len(records))
	}
	first := records[0].(map[string]any)
	if stringField(t, first, "id") != "usage-a-new" {
		t.Fatalf("first usage id = %q, want usage-a-new", stringField(t, first, "id"))
	}
	rawUsage := objectField(t, first, "rawUsage")
	if rawUsage["safe"] != "ok" {
		t.Fatalf("rawUsage.safe = %#v, want ok", rawUsage["safe"])
	}

	empty := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?projectId=project-b", nil, adminSession.cookies, nil)
	if empty.Code != http.StatusOK {
		t.Fatalf("empty usage status = %d, want %d: %s", empty.Code, http.StatusOK, empty.Body.String())
	}
	emptyData := decodeData(t, empty)
	assertPageMeta(t, emptyData, 0, 1, 20)
	if len(recordsField(t, emptyData)) != 0 {
		t.Fatalf("empty usage records = %#v, want []", recordsField(t, emptyData))
	}

	invalid := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?pageNum=0", nil, adminSession.cookies, nil)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid usage page status = %d, want %d", invalid.Code, http.StatusUnprocessableEntity)
	}
}

func TestAdminUsageSummaryAggregatesWithinTenant(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	seedUsageRecord(t, db, usageSeed{ID: "usage-summary-a1", TenantID: adminSession.tenantID, TaskID: "task-a1", UserID: adminSession.userID, ProjectID: "project-a", ProviderID: "provider-a", ModelID: "model-a", InputTokens: 10, OutputTokens: 20, ImageCount: 1, EstimatedCost: "0.10000000", RawUsageJSON: `{}`, CreatedAt: now})
	seedUsageRecord(t, db, usageSeed{ID: "usage-summary-a2", TenantID: adminSession.tenantID, TaskID: "task-a2", UserID: adminSession.userID, ProjectID: "project-a", ProviderID: "provider-a", ModelID: "model-b", InputTokens: 30, OutputTokens: 40, ImageCount: 2, EstimatedCost: "0.30000000", RawUsageJSON: `{}`, CreatedAt: now.Add(time.Minute)})
	seedUsageRecord(t, db, usageSeed{ID: "usage-summary-b", TenantID: "tenant-b", TaskID: "task-b", UserID: "user-b", ProjectID: "project-b", ProviderID: "provider-a", ModelID: "model-b", InputTokens: 1000, OutputTokens: 1000, ImageCount: 10, EstimatedCost: "10.00000000", RawUsageJSON: `{}`, CreatedAt: now.Add(2 * time.Minute)})

	response := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=provider&pageNum=1&pageSize=10", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("usage summary status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	data := decodeData(t, response)
	assertPageMeta(t, data, 1, 1, 10)
	records := recordsField(t, data)
	if len(records) != 1 {
		t.Fatalf("summary len = %d, want 1", len(records))
	}
	summary := records[0].(map[string]any)
	if stringField(t, summary, "dimension") != "provider" || stringField(t, summary, "dimensionId") != "provider-a" {
		t.Fatalf("summary dimension = %#v", summary)
	}
	assertFloatField(t, summary, "inputTokens", 40)
	assertFloatField(t, summary, "outputTokens", 60)
	assertFloatField(t, summary, "imageCount", 3)
	if stringField(t, summary, "estimatedCost") != "0.40000000" {
		t.Fatalf("summary estimatedCost = %q, want 0.40000000", stringField(t, summary, "estimatedCost"))
	}

	invalid := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=tenant", nil, adminSession.cookies, nil)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid summary dimension status = %d, want %d", invalid.Code, http.StatusUnprocessableEntity)
	}
}

func TestAdminOperationLogsListTenantIsolationAndRecursiveRedaction(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	actor := adminSession.userID
	seedOperationLog(t, db, database.OperationLog{
		ID:           "operation-a",
		TenantID:     adminSession.tenantID,
		ActorUserID:  &actor,
		Action:       "provider.test",
		ResourceType: "provider",
		ResourceID:   "provider-a",
		IP:           "127.0.0.1",
		UserAgent:    "test-agent",
		MetadataJSON: `{"safe":"ok","headers":{"Cookie":"session=secret","Authorization":"Bearer op-secret"},"events":[{"body":"data:image/png;base64,abc"}],"note":"sk-operation-secret-001"}`,
		CreatedAt:    now,
	})
	tenantBActor := "user-b"
	seedOperationLog(t, db, database.OperationLog{
		ID:           "operation-b",
		TenantID:     "tenant-b",
		ActorUserID:  &tenantBActor,
		Action:       "provider.test",
		ResourceType: "provider",
		ResourceID:   "provider-b",
		IP:           "127.0.0.1",
		UserAgent:    "test-agent",
		MetadataJSON: `{"safe":"tenant-b","apiKey":"sk-cross-tenant"}`,
		CreatedAt:    now.Add(time.Minute),
	})

	response := performJSON(router, http.MethodGet, "/api/v1/admin/operation-logs?action=provider.test&resourceType=provider&actorUserId="+adminSession.userID, nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("operation logs status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertResponseExcludes(t, response.Body.String(), "sk-operation-secret", "op-secret", "session=secret", "base64", "authorization", "cookie", "tenant-b")
	data := decodeData(t, response)
	assertPageMeta(t, data, 1, 1, 20)
	records := recordsField(t, data)
	if len(records) != 1 {
		t.Fatalf("operation records len = %d, want 1", len(records))
	}
	metadata := objectField(t, records[0].(map[string]any), "metadata")
	if metadata["safe"] != "ok" {
		t.Fatalf("operation metadata.safe = %#v, want ok", metadata["safe"])
	}

	empty := performJSON(router, http.MethodGet, "/api/v1/admin/operation-logs?resourceId=provider-b", nil, adminSession.cookies, nil)
	if empty.Code != http.StatusOK {
		t.Fatalf("cross-tenant operation status = %d, want %d: %s", empty.Code, http.StatusOK, empty.Body.String())
	}
	if len(recordsField(t, decodeData(t, empty))) != 0 {
		t.Fatalf("cross-tenant operation logs leaked: %s", empty.Body.String())
	}
}

func TestAdminAPICallLogsListDetailTenantIsolationAndProviderPayloadRedaction(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	seedGenerationTaskForAPILog(t, db, adminSession.tenantID, "task-a", "project-a", adminSession.userID, now)
	seedGenerationTaskForAPILog(t, db, "tenant-b", "task-b", "project-b", "user-b", now)
	statusOK := 200
	seedAPICallLog(t, db, database.APICallLog{
		ID:                   "api-log-a",
		TenantID:             adminSession.tenantID,
		TaskID:               "task-a",
		ProviderID:           "provider-a",
		ModelID:              "model-a",
		Status:               "SUCCESS",
		DurationMs:           123,
		RequestID:            "provider-request-a",
		HTTPStatus:           &statusOK,
		ErrorCode:            "",
		ErrorMessage:         "provider mentioned sk-error-secret-001",
		RedactedRequestJSON:  `{"headers":{"Authorization":"Bearer api-secret"},"api_key":"sk-api-secret-001","prompt":"safe"}`,
		RedactedResponseJSON: `{"data":[{"b64_json":"raw-provider-image-base64"}],"message":"contains sk-response-secret-001"}`,
		CreatedAt:            now,
	})
	seedAPICallLog(t, db, database.APICallLog{
		ID:                   "api-log-b",
		TenantID:             "tenant-b",
		TaskID:               "task-b",
		ProviderID:           "provider-b",
		ModelID:              "model-b",
		Status:               "FAILURE",
		DurationMs:           999,
		RequestID:            "provider-request-b",
		HTTPStatus:           &statusOK,
		ErrorCode:            "ERR",
		ErrorMessage:         "tenant b",
		RedactedRequestJSON:  `{"prompt":"tenant-b","apiKey":"sk-cross-tenant"}`,
		RedactedResponseJSON: `{}`,
		CreatedAt:            now.Add(time.Minute),
	})

	list := performJSON(router, http.MethodGet, "/api/v1/admin/api-call-logs?projectId=project-a&userId="+adminSession.userID+"&status=SUCCESS&pageNum=1&pageSize=10", nil, adminSession.cookies, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("api call list status = %d, want %d: %s", list.Code, http.StatusOK, list.Body.String())
	}
	assertResponseExcludes(t, list.Body.String(), "sk-api-secret", "sk-response-secret", "sk-error-secret", "api-secret", "raw-provider-image-base64", "authorization", "api_key", "b64_json", "tenant-b")
	data := decodeData(t, list)
	assertPageMeta(t, data, 1, 1, 10)
	records := recordsField(t, data)
	if len(records) != 1 {
		t.Fatalf("api call records len = %d, want 1", len(records))
	}
	if stringField(t, records[0].(map[string]any), "errorMessage") != "Provider error message redacted." {
		t.Fatalf("api call errorMessage = %#v", records[0].(map[string]any)["errorMessage"])
	}

	detail := performJSON(router, http.MethodGet, "/api/v1/admin/api-call-logs/api-log-a", nil, adminSession.cookies, nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("api call detail status = %d, want %d: %s", detail.Code, http.StatusOK, detail.Body.String())
	}
	assertResponseExcludes(t, detail.Body.String(), "sk-api-secret", "sk-response-secret", "sk-error-secret", "api-secret", "raw-provider-image-base64", "authorization", "api_key", "b64_json")
	requestMetadata := objectField(t, decodeData(t, detail), "redactedRequest")
	if requestMetadata["prompt"] != "safe" {
		t.Fatalf("redactedRequest.prompt = %#v, want safe", requestMetadata["prompt"])
	}

	crossTenantDetail := performJSON(router, http.MethodGet, "/api/v1/admin/api-call-logs/api-log-b", nil, adminSession.cookies, nil)
	if crossTenantDetail.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant detail status = %d, want %d", crossTenantDetail.Code, http.StatusNotFound)
	}

	invalid := performJSON(router, http.MethodGet, "/api/v1/admin/api-call-logs?createdAtFrom=not-a-date", nil, adminSession.cookies, nil)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid api call filter status = %d, want %d", invalid.Code, http.StatusUnprocessableEntity)
	}
}

type usageSeed struct {
	ID            string
	TenantID      string
	TaskID        string
	UserID        string
	ProjectID     string
	ProviderID    string
	ModelID       string
	InputTokens   int64
	OutputTokens  int64
	ImageCount    int
	EstimatedCost string
	RawUsageJSON  string
	CreatedAt     time.Time
}

func seedUsageRecord(t *testing.T, db *gorm.DB, seed usageSeed) {
	t.Helper()
	if err := db.Create(&database.UsageRecord{
		ID:            seed.ID,
		TenantID:      seed.TenantID,
		TaskID:        seed.TaskID,
		UserID:        seed.UserID,
		ProjectID:     seed.ProjectID,
		ProviderID:    seed.ProviderID,
		ModelID:       seed.ModelID,
		InputTokens:   seed.InputTokens,
		OutputTokens:  seed.OutputTokens,
		ImageCount:    seed.ImageCount,
		EstimatedCost: seed.EstimatedCost,
		Currency:      "USD",
		RawUsageJSON:  seed.RawUsageJSON,
		CreatedAt:     seed.CreatedAt,
	}).Error; err != nil {
		t.Fatalf("seed usage record %s: %v", seed.ID, err)
	}
}

func seedOperationLog(t *testing.T, db *gorm.DB, record database.OperationLog) {
	t.Helper()
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed operation log %s: %v", record.ID, err)
	}
}

func seedGenerationTaskForAPILog(t *testing.T, db *gorm.DB, tenantID string, taskID string, projectID string, userID string, createdAt time.Time) {
	t.Helper()
	if err := db.Create(&database.GenerationTask{
		ID:                taskID,
		TenantID:          tenantID,
		ProjectID:         projectID,
		Type:              task.TypeImageGeneration,
		ProviderID:        "provider-" + strings.TrimPrefix(projectID, "project-"),
		ModelID:           "model-" + strings.TrimPrefix(projectID, "project-"),
		Status:            task.StatusSucceeded,
		Prompt:            "prompt",
		ImageType:         "MAIN",
		ParamsJSON:        `{}`,
		InputAssetIDsJSON: `[]`,
		Attempt:           1,
		MaxAttempts:       3,
		CreatedBy:         userID,
		ErrorCode:         "",
		ErrorMessage:      "",
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt,
	}).Error; err != nil {
		t.Fatalf("seed generation task %s: %v", taskID, err)
	}
}

func seedAPICallLog(t *testing.T, db *gorm.DB, record database.APICallLog) {
	t.Helper()
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed api call log %s: %v", record.ID, err)
	}
}

func recordsField(t *testing.T, data map[string]any) []any {
	t.Helper()
	records, ok := data["records"].([]any)
	if !ok {
		t.Fatalf("data.records is not an array: %#v", data["records"])
	}
	return records
}

func objectField(t *testing.T, data map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := data[key].(map[string]any)
	if !ok {
		t.Fatalf("data.%s is not an object: %#v", key, data[key])
	}
	return value
}

func assertPageMeta(t *testing.T, data map[string]any, total int, pageNum int, pageSize int) {
	t.Helper()
	assertFloatField(t, data, "total", float64(total))
	assertFloatField(t, data, "pageNum", float64(pageNum))
	assertFloatField(t, data, "pageSize", float64(pageSize))
}

func assertFloatField(t *testing.T, data map[string]any, key string, expected float64) {
	t.Helper()
	value, ok := data[key].(float64)
	if !ok {
		t.Fatalf("data.%s is not a number: %#v", key, data[key])
	}
	if value != expected {
		t.Fatalf("data.%s = %s, want %s", key, strconv.FormatFloat(value, 'f', -1, 64), strconv.FormatFloat(expected, 'f', -1, 64))
	}
}

func assertResponseExcludes(t *testing.T, body string, forbidden ...string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, marker := range forbidden {
		if strings.Contains(lower, strings.ToLower(marker)) {
			t.Fatalf("response contains %q: %s", marker, body)
		}
	}
	assertValidJSONResponse(t, body)
}

func assertValidJSONResponse(t *testing.T, body string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
}
