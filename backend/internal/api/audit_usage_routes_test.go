package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/redaction"
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

	for _, dimension := range []string{"tenant", "user", "project", "provider", "model"} {
		t.Run(dimension, func(t *testing.T) {
			response := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension="+dimension+"&pageNum=1&pageSize=10", nil, adminSession.cookies, nil)
			if response.Code != http.StatusOK {
				t.Fatalf("usage summary status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
			}
			assertResponseExcludes(t, response.Body.String(), "tenant-b", "task-b", "user-b", "project-b", "10.00000000")
			data := decodeData(t, response)
			records := recordsField(t, data)
			if len(records) == 0 {
				t.Fatalf("summary records empty for dimension %s", dimension)
			}
			for _, record := range records {
				summary := record.(map[string]any)
				if stringField(t, summary, "dimension") != dimension {
					t.Fatalf("summary dimension = %#v, want %s", summary, dimension)
				}
				if dimension == "tenant" && stringField(t, summary, "dimensionId") != adminSession.tenantID {
					t.Fatalf("tenant summary dimensionId = %q, want current tenant", stringField(t, summary, "dimensionId"))
				}
			}
		})
	}

	response := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=provider&pageNum=1&pageSize=10", nil, adminSession.cookies, nil)
	data := decodeData(t, response)
	assertPageMeta(t, data, 1, 1, 10)
	summary := recordsField(t, data)[0].(map[string]any)
	if stringField(t, summary, "dimensionId") != "provider-a" {
		t.Fatalf("summary dimensionId = %#v", summary)
	}
	assertFloatField(t, summary, "inputTokens", 40)
	assertFloatField(t, summary, "outputTokens", 60)
	assertFloatField(t, summary, "imageCount", 3)
	if stringField(t, summary, "estimatedCost") != "0.40000000" {
		t.Fatalf("summary estimatedCost = %q, want 0.40000000", stringField(t, summary, "estimatedCost"))
	}

	invalid := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=account", nil, adminSession.cookies, nil)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid summary dimension status = %d, want %d", invalid.Code, http.StatusUnprocessableEntity)
	}
}

func TestAdminUsageSummaryTenantDimensionReturnsExactAggregate(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	seedUsageRecord(t, db, usageSeed{ID: "usage-tenant-exact-a", TenantID: adminSession.tenantID, TaskID: "task-tenant-exact-a", UserID: adminSession.userID, ProjectID: "project-a", ProviderID: "provider-a", ModelID: "model-a", InputTokens: 10, OutputTokens: 20, ImageCount: 1, EstimatedCost: "0.10000000", RawUsageJSON: `{}`, CreatedAt: now})
	seedUsageRecord(t, db, usageSeed{ID: "usage-tenant-exact-b", TenantID: adminSession.tenantID, TaskID: "task-tenant-exact-b", UserID: adminSession.userID, ProjectID: "project-b", ProviderID: "provider-b", ModelID: "model-b", InputTokens: 30, OutputTokens: 40, ImageCount: 2, EstimatedCost: "0.30000000", RawUsageJSON: `{}`, CreatedAt: now.Add(time.Minute)})
	seedUsageRecord(t, db, usageSeed{ID: "usage-tenant-exact-cross", TenantID: "tenant-b", TaskID: "task-tenant-exact-cross", UserID: "user-b", ProjectID: "project-cross", ProviderID: "provider-cross", ModelID: "model-cross", InputTokens: 1000, OutputTokens: 1000, ImageCount: 10, EstimatedCost: "10.00000000", RawUsageJSON: `{}`, CreatedAt: now.Add(2 * time.Minute)})

	response := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=tenant&pageNum=1&pageSize=10", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("usage summary status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertResponseExcludes(t, response.Body.String(), "tenant-b", "task-tenant-exact-cross", "10.00000000")
	data := decodeData(t, response)
	assertPageMeta(t, data, 1, 1, 10)
	records := recordsField(t, data)
	if len(records) != 1 {
		t.Fatalf("tenant summary len = %d, want 1", len(records))
	}
	assertUsageSummary(t, records[0].(map[string]any), "tenant", adminSession.tenantID, "USD", 2, 40, 60, 3, "0.40000000")
}

func TestAdminUsageSummaryGroupsByDimensionAndCurrency(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	seedUsageRecord(t, db, usageSeed{ID: "usage-currency-usd", TenantID: adminSession.tenantID, TaskID: "task-currency-usd", UserID: adminSession.userID, ProjectID: "project-a", ProviderID: "provider-a", ModelID: "model-a", InputTokens: 1, OutputTokens: 2, ImageCount: 1, EstimatedCost: "0.10000000", Currency: "USD", RawUsageJSON: `{}`, CreatedAt: now})
	seedUsageRecord(t, db, usageSeed{ID: "usage-currency-eur", TenantID: adminSession.tenantID, TaskID: "task-currency-eur", UserID: adminSession.userID, ProjectID: "project-a", ProviderID: "provider-a", ModelID: "model-a", InputTokens: 3, OutputTokens: 4, ImageCount: 2, EstimatedCost: "0.30000000", Currency: "EUR", RawUsageJSON: `{}`, CreatedAt: now})

	response := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=provider&pageNum=1&pageSize=10", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("usage summary status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	data := decodeData(t, response)
	assertPageMeta(t, data, 2, 1, 10)
	records := recordsField(t, data)
	if len(records) != 2 {
		t.Fatalf("currency summary len = %d, want 2", len(records))
	}
	assertUsageSummary(t, records[0].(map[string]any), "provider", "provider-a", "EUR", 1, 3, 4, 2, "0.30000000")
	assertUsageSummary(t, records[1].(map[string]any), "provider", "provider-a", "USD", 1, 1, 2, 1, "0.10000000")
}

func TestAdminUsageSummaryPaginationIsStableWhenLatestCreatedAtMatches(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	createdAt := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	for _, projectID := range []string{"project-c", "project-a", "project-b"} {
		seedUsageRecord(t, db, usageSeed{
			ID:            "usage-summary-" + projectID,
			TenantID:      adminSession.tenantID,
			TaskID:        "task-summary-" + projectID,
			UserID:        adminSession.userID,
			ProjectID:     projectID,
			ProviderID:    "provider-a",
			ModelID:       "model-a",
			InputTokens:   1,
			OutputTokens:  1,
			ImageCount:    1,
			EstimatedCost: "0.01000000",
			RawUsageJSON:  `{}`,
			CreatedAt:     createdAt,
		})
	}

	pageOne := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=project&pageNum=1&pageSize=2", nil, adminSession.cookies, nil)
	pageTwo := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=project&pageNum=2&pageSize=2", nil, adminSession.cookies, nil)
	repeatedPageOne := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=project&pageNum=1&pageSize=2", nil, adminSession.cookies, nil)
	repeatedPageTwo := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=project&pageNum=2&pageSize=2", nil, adminSession.cookies, nil)

	assertSummaryDimensionIDs(t, pageOne, []string{"project-a", "project-b"})
	assertSummaryDimensionIDs(t, pageTwo, []string{"project-c"})
	assertSummaryDimensionIDs(t, repeatedPageOne, []string{"project-a", "project-b"})
	assertSummaryDimensionIDs(t, repeatedPageTwo, []string{"project-c"})
}

func TestAdminUsageSummaryFiltersRemainTenantScoped(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	seedUsageRecord(t, db, usageSeed{ID: "usage-filter-match", TenantID: adminSession.tenantID, TaskID: "task-filter-match", UserID: adminSession.userID, ProjectID: "project-filter", ProviderID: "provider-filter", ModelID: "model-filter", InputTokens: 5, OutputTokens: 6, ImageCount: 1, EstimatedCost: "0.11000000", RawUsageJSON: `{}`, CreatedAt: now})
	seedUsageRecord(t, db, usageSeed{ID: "usage-filter-time-out", TenantID: adminSession.tenantID, TaskID: "task-filter-time-out", UserID: adminSession.userID, ProjectID: "project-filter", ProviderID: "provider-filter", ModelID: "model-filter", InputTokens: 100, OutputTokens: 100, ImageCount: 10, EstimatedCost: "1.00000000", RawUsageJSON: `{}`, CreatedAt: now.Add(-2 * time.Hour)})
	seedUsageRecord(t, db, usageSeed{ID: "usage-filter-other-project", TenantID: adminSession.tenantID, TaskID: "task-filter-other-project", UserID: adminSession.userID, ProjectID: "project-other", ProviderID: "provider-filter", ModelID: "model-filter", InputTokens: 100, OutputTokens: 100, ImageCount: 10, EstimatedCost: "1.00000000", RawUsageJSON: `{}`, CreatedAt: now})
	seedUsageRecord(t, db, usageSeed{ID: "usage-filter-cross-tenant", TenantID: "tenant-b", TaskID: "task-filter-cross-tenant", UserID: adminSession.userID, ProjectID: "project-filter", ProviderID: "provider-filter", ModelID: "model-filter", InputTokens: 1000, OutputTokens: 1000, ImageCount: 10, EstimatedCost: "10.00000000", RawUsageJSON: `{}`, CreatedAt: now})

	path := "/api/v1/admin/usage/summary?dimension=tenant&userId=" + adminSession.userID + "&projectId=project-filter&providerId=provider-filter&modelId=model-filter&createdAtFrom=2026-05-18T09:00:00Z&createdAtTo=2026-05-18T11:00:00Z"
	response := performJSON(router, http.MethodGet, path, nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("usage summary status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertResponseExcludes(t, response.Body.String(), "tenant-b", "task-filter-cross-tenant", "project-other", "10.00000000")
	data := decodeData(t, response)
	assertPageMeta(t, data, 1, 1, 20)
	records := recordsField(t, data)
	if len(records) != 1 {
		t.Fatalf("filtered summary len = %d, want 1", len(records))
	}
	assertUsageSummary(t, records[0].(map[string]any), "tenant", adminSession.tenantID, "USD", 1, 5, 6, 1, "0.11000000")
}

func TestAdminUsageSummaryPreservesLargeDecimalCost(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	seedUsageRecord(t, db, usageSeed{ID: "usage-large-decimal", TenantID: adminSession.tenantID, TaskID: "task-large-decimal", UserID: adminSession.userID, ProjectID: "project-large-decimal", ProviderID: "provider-large-decimal", ModelID: "model-large-decimal", InputTokens: 1, OutputTokens: 1, ImageCount: 1, EstimatedCost: "1234567890.12345678", RawUsageJSON: `{}`, CreatedAt: now})

	response := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=tenant", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("usage summary status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	summary := recordsField(t, decodeData(t, response))[0].(map[string]any)
	if stringField(t, summary, "estimatedCost") != "1234567890.12345678" {
		t.Fatalf("summary estimatedCost = %q, want 1234567890.12345678", stringField(t, summary, "estimatedCost"))
	}
}

func TestAdminAuditUsageReadResponsesRedactInjectedKnownSecretsRecursively(t *testing.T) {
	const knownSecret = "relay_live_1234567890abcdef"

	router, db, adminSession := newAuditUsageRouteTestRouter(t, redaction.New(knownSecret))
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	actor := adminSession.userID
	statusOK := 200

	seedUsageRecord(t, db, usageSeed{
		ID:            "usage-known-secret",
		TenantID:      adminSession.tenantID,
		TaskID:        "task-known-secret",
		UserID:        adminSession.userID,
		ProjectID:     "project-known-secret",
		ProviderID:    "provider-known-secret",
		ModelID:       "model-known-secret",
		InputTokens:   1,
		OutputTokens:  2,
		ImageCount:    1,
		EstimatedCost: "0.01000000",
		RawUsageJSON:  `{"safe":"usage","message":"provider echoed relay_live_1234567890abcdef","nested":{"prefix_relay_live_1234567890abcdef":"drop me","safe":"keep me"}}`,
		CreatedAt:     now,
	})
	seedOperationLog(t, db, database.OperationLog{
		ID:           "operation-known-secret",
		TenantID:     adminSession.tenantID,
		ActorUserID:  &actor,
		Action:       "provider.test",
		ResourceType: "provider",
		ResourceID:   "provider-known-secret",
		IP:           "127.0.0.1",
		UserAgent:    "test-agent",
		MetadataJSON: `{"safe":"operation","message":"provider echoed relay_live_1234567890abcdef","nested":{"relay_live_1234567890abcdef":"drop me","safe":"keep me"}}`,
		CreatedAt:    now,
	})
	seedGenerationTaskForAPILog(t, db, adminSession.tenantID, "task-api-known-secret", "project-known-secret", adminSession.userID, now)
	seedAPICallLog(t, db, database.APICallLog{
		ID:                   "api-log-known-secret",
		TenantID:             adminSession.tenantID,
		TaskID:               "task-api-known-secret",
		ProviderID:           "provider-known-secret",
		ModelID:              "model-known-secret",
		Status:               "SUCCESS",
		DurationMs:           10,
		RequestID:            "provider-request-known-secret",
		HTTPStatus:           &statusOK,
		ErrorCode:            "",
		ErrorMessage:         "",
		RedactedRequestJSON:  `{"safe":"request","message":"provider echoed relay_live_1234567890abcdef","nested":{"prefix_relay_live_1234567890abcdef":"drop me","safe":"keep me"}}`,
		RedactedResponseJSON: `{"safe":"response","message":"provider echoed relay_live_1234567890abcdef","nested":{"relay_live_1234567890abcdef":"drop me","safe":"keep me"}}`,
		CreatedAt:            now,
	})

	usage := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?taskId=task-known-secret", nil, adminSession.cookies, nil)
	if usage.Code != http.StatusOK {
		t.Fatalf("usage known-secret status = %d, want %d: %s", usage.Code, http.StatusOK, usage.Body.String())
	}
	assertResponseExcludes(t, usage.Body.String(), knownSecret)
	usagePayload := objectField(t, recordsField(t, decodeData(t, usage))[0].(map[string]any), "rawUsage")
	assertNestedKnownSecretKeyDropped(t, usagePayload, "nested")

	operation := performJSON(router, http.MethodGet, "/api/v1/admin/operation-logs?resourceId=provider-known-secret", nil, adminSession.cookies, nil)
	if operation.Code != http.StatusOK {
		t.Fatalf("operation known-secret status = %d, want %d: %s", operation.Code, http.StatusOK, operation.Body.String())
	}
	assertResponseExcludes(t, operation.Body.String(), knownSecret)
	operationPayload := objectField(t, recordsField(t, decodeData(t, operation))[0].(map[string]any), "metadata")
	assertNestedKnownSecretKeyDropped(t, operationPayload, "nested")

	apiCall := performJSON(router, http.MethodGet, "/api/v1/admin/api-call-logs/api-log-known-secret", nil, adminSession.cookies, nil)
	if apiCall.Code != http.StatusOK {
		t.Fatalf("api call known-secret status = %d, want %d: %s", apiCall.Code, http.StatusOK, apiCall.Body.String())
	}
	assertResponseExcludes(t, apiCall.Body.String(), knownSecret)
	apiCallPayload := decodeData(t, apiCall)
	assertNestedKnownSecretKeyDropped(t, objectField(t, apiCallPayload, "redactedRequest"), "nested")
	assertNestedKnownSecretKeyDropped(t, objectField(t, apiCallPayload, "redactedResponse"), "nested")
}

func TestAdminUsageRecordsPaginationIsStableWhenCreatedAtMatches(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	createdAt := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	for _, id := range []string{"usage-same-a", "usage-same-c", "usage-same-b"} {
		seedUsageRecord(t, db, usageSeed{
			ID:            id,
			TenantID:      adminSession.tenantID,
			TaskID:        "task-" + id,
			UserID:        adminSession.userID,
			ProjectID:     "project-a",
			ProviderID:    "provider-a",
			ModelID:       "model-a",
			InputTokens:   1,
			OutputTokens:  1,
			ImageCount:    1,
			EstimatedCost: "0.01000000",
			RawUsageJSON:  `{}`,
			CreatedAt:     createdAt,
		})
	}

	pageOne := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?pageNum=1&pageSize=2", nil, adminSession.cookies, nil)
	pageTwo := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?pageNum=2&pageSize=2", nil, adminSession.cookies, nil)
	repeatedPageOne := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?pageNum=1&pageSize=2", nil, adminSession.cookies, nil)
	repeatedPageTwo := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?pageNum=2&pageSize=2", nil, adminSession.cookies, nil)

	assertRecordIDs(t, pageOne, []string{"usage-same-c", "usage-same-b"})
	assertRecordIDs(t, pageTwo, []string{"usage-same-a"})
	assertRecordIDs(t, repeatedPageOne, []string{"usage-same-c", "usage-same-b"})
	assertRecordIDs(t, repeatedPageTwo, []string{"usage-same-a"})
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
	Currency      string
	RawUsageJSON  string
	CreatedAt     time.Time
}

func seedUsageRecord(t *testing.T, db *gorm.DB, seed usageSeed) {
	t.Helper()
	currency := strings.ToUpper(strings.TrimSpace(seed.Currency))
	if currency == "" {
		currency = "USD"
	}
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
		Currency:      currency,
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

func newAuditUsageRouteTestRouter(t *testing.T, redactor *redaction.Redactor) (http.Handler, *gorm.DB, projectRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	router := NewRouter(RouterOptions{
		Config:            authRouteTestConfig("test"),
		Logger:            discardLogger(),
		Database:          db,
		AuditReadRedactor: redactor,
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

func assertNestedKnownSecretKeyDropped(t *testing.T, payload map[string]any, key string) {
	t.Helper()
	nested := objectField(t, payload, key)
	if len(nested) != 1 || nested["safe"] != "keep me" {
		t.Fatalf("nested payload = %#v, want only safe key", nested)
	}
}

func assertRecordIDs(t *testing.T, response *httptest.ResponseRecorder, want []string) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("record page status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	records := recordsField(t, decodeData(t, response))
	if len(records) != len(want) {
		t.Fatalf("record page len = %d, want %d: %s", len(records), len(want), response.Body.String())
	}
	for index, id := range want {
		if actual := stringField(t, records[index].(map[string]any), "id"); actual != id {
			t.Fatalf("record id[%d] = %q, want %q", index, actual, id)
		}
	}
}

func assertSummaryDimensionIDs(t *testing.T, response *httptest.ResponseRecorder, want []string) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("summary page status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	records := recordsField(t, decodeData(t, response))
	if len(records) != len(want) {
		t.Fatalf("summary page len = %d, want %d: %s", len(records), len(want), response.Body.String())
	}
	for index, id := range want {
		if actual := stringField(t, records[index].(map[string]any), "dimensionId"); actual != id {
			t.Fatalf("summary dimensionId[%d] = %q, want %q", index, actual, id)
		}
	}
}

func assertUsageSummary(t *testing.T, data map[string]any, dimension string, dimensionID string, currency string, recordCount int64, inputTokens int64, outputTokens int64, imageCount int64, estimatedCost string) {
	t.Helper()
	if stringField(t, data, "dimension") != dimension {
		t.Fatalf("summary dimension = %q, want %q", stringField(t, data, "dimension"), dimension)
	}
	if stringField(t, data, "dimensionId") != dimensionID {
		t.Fatalf("summary dimensionId = %q, want %q", stringField(t, data, "dimensionId"), dimensionID)
	}
	if stringField(t, data, "currency") != currency {
		t.Fatalf("summary currency = %q, want %q", stringField(t, data, "currency"), currency)
	}
	assertFloatField(t, data, "recordCount", float64(recordCount))
	assertFloatField(t, data, "inputTokens", float64(inputTokens))
	assertFloatField(t, data, "outputTokens", float64(outputTokens))
	assertFloatField(t, data, "imageCount", float64(imageCount))
	if stringField(t, data, "estimatedCost") != estimatedCost {
		t.Fatalf("summary estimatedCost = %q, want %q", stringField(t, data, "estimatedCost"), estimatedCost)
	}
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
