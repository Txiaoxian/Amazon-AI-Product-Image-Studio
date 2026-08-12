package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/usagecost"
	"gorm.io/gorm"
)

func TestAdminAnalyticsOverviewUsesAuthoritativeMetricsNamesCostsAndTenantIsolation(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	seedAnalyticsDimensions(t, db, adminSession.tenantID, adminSession.userID)
	current := time.Date(2026, 8, 10, 2, 0, 0, 0, time.UTC)
	seedAnalyticsTask(t, db, analyticsTaskSeed{ID: "analytics-success", TenantID: adminSession.tenantID, UserID: adminSession.userID, Status: task.StatusSucceeded, StartedAt: current, FinishedAt: current.Add(time.Second), CreatedAt: current})
	seedAnalyticsTask(t, db, analyticsTaskSeed{ID: "analytics-failed", TenantID: adminSession.tenantID, UserID: adminSession.userID, Status: task.StatusFailed, StartedAt: current, FinishedAt: current.Add(3 * time.Second), CreatedAt: current.Add(time.Hour), ErrorCode: "PROVIDER_TIMEOUT"})
	seedAnalyticsTask(t, db, analyticsTaskSeed{ID: "analytics-queued", TenantID: adminSession.tenantID, UserID: adminSession.userID, Status: task.StatusQueued, CreatedAt: current.Add(2 * time.Hour)})
	for index := 0; index < 2; index++ {
		if err := db.Create(&database.TaskOutput{ID: "analytics-output-" + string(rune('a'+index)), TenantID: adminSession.tenantID, TaskID: "analytics-success", AssetID: "asset-" + string(rune('a'+index)), OutputIndex: index, CreatedAt: current.Add(time.Minute)}).Error; err != nil {
			t.Fatalf("seed analytics output: %v", err)
		}
	}
	seedUsageRecord(t, db, usageSeed{ID: "analytics-usage-priced", TenantID: adminSession.tenantID, TaskID: "analytics-success", UserID: adminSession.userID, ProjectID: "analytics-project", ProviderID: "analytics-provider", ModelID: "analytics-model", InputTokens: 10, OutputTokens: 20, ImageCount: 2, EstimatedCost: "0.50000000", Currency: "USD", CostStatus: usagecost.StatusCalculated, PricingJSON: `{"currency":"USD","unitPrices":{"image":0.25}}`, RawUsageJSON: `{}`, CreatedAt: current})
	seedUsageRecord(t, db, usageSeed{ID: "analytics-usage-unavailable", TenantID: adminSession.tenantID, TaskID: "analytics-failed", UserID: adminSession.userID, ProjectID: "analytics-project", ProviderID: "analytics-provider", ModelID: "analytics-model", EstimatedCost: "0.00000000", Currency: "USD", CostStatus: usagecost.StatusUnavailable, PricingJSON: `{}`, RawUsageJSON: `{}`, CreatedAt: current.Add(time.Hour)})
	seedAnalyticsTask(t, db, analyticsTaskSeed{ID: "analytics-cross-tenant", TenantID: "tenant-b", UserID: "user-b", Status: task.StatusSucceeded, StartedAt: current, FinishedAt: current.Add(time.Second), CreatedAt: current})
	seedUsageRecord(t, db, usageSeed{ID: "analytics-cross-cost", TenantID: "tenant-b", TaskID: "analytics-cross-tenant", UserID: "user-b", ProjectID: "analytics-project", ProviderID: "analytics-provider", ModelID: "analytics-model", ImageCount: 99, EstimatedCost: "99.00000000", Currency: "USD", CostStatus: usagecost.StatusCalculated, RawUsageJSON: `{}`, CreatedAt: current})
	actor := adminSession.userID
	seedOperationLog(t, db, database.OperationLog{ID: "analytics-login", TenantID: adminSession.tenantID, ActorUserID: &actor, Action: "auth.login", ResourceType: "session", ResourceID: "session-analytics", MetadataJSON: `{}`, CreatedAt: current})

	response := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/overview?from=2026-08-10&to=2026-08-10&compare=false", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("analytics overview status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertResponseExcludes(t, response.Body.String(), "tenant-b", "99.00000000")
	data := decodeData(t, response)
	currentMetrics := objectField(t, data, "current")
	assertFloatField(t, currentMetrics, "taskCount", 3)
	assertFloatField(t, currentMetrics, "outputCount", 2)
	assertFloatField(t, currentMetrics, "terminalTaskCount", 2)
	assertFloatField(t, currentMetrics, "taskSuccessRate", 0.5)
	assertFloatField(t, currentMetrics, "activeUserCount", 1)
	assertFloatField(t, currentMetrics, "loginActiveUserCount", 1)
	assertFloatField(t, currentMetrics, "p95DurationMs", 3000)
	costs := data["costs"].([]any)
	if len(costs) != 1 {
		t.Fatalf("overview costs = %#v, want one currency", costs)
	}
	cost := costs[0].(map[string]any)
	if stringField(t, cost, "amount") != "0.50000000" || stringField(t, cost, "currency") != "USD" {
		t.Fatalf("overview cost = %#v", cost)
	}
	assertFloatField(t, cost, "pricingCoverage", 0.5)
	rankings := data["rankings"].([]any)
	foundProviderName := false
	for _, raw := range rankings {
		item := raw.(map[string]any)
		if stringField(t, item, "dimension") == "provider" && stringField(t, item, "name") == "上海中转站" {
			foundProviderName = true
		}
	}
	if !foundProviderName {
		t.Fatalf("overview rankings did not prefer display name: %#v", rankings)
	}

	usage := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/usage?from=2026-08-10&to=2026-08-10&compare=false", nil, adminSession.cookies, nil)
	if usage.Code != http.StatusOK {
		t.Fatalf("analytics usage status = %d, want %d: %s", usage.Code, http.StatusOK, usage.Body.String())
	}
	usageData := decodeData(t, usage)
	assertFloatField(t, usageData, "outputCount", 2)
	unitCosts := usageData["unitCosts"].([]any)
	if len(unitCosts) != 1 {
		t.Fatalf("usage unit costs = %#v, want one currency", unitCosts)
	}
	unitCost := unitCosts[0].(map[string]any)
	if stringField(t, unitCost, "amount") != "0.25000000" || stringField(t, unitCost, "currency") != "USD" {
		t.Fatalf("usage unit cost = %#v", unitCost)
	}
	assertFloatField(t, unitCost, "outputCount", 2)
	if available, ok := unitCost["available"].(bool); !ok || !available {
		t.Fatalf("usage unit cost availability = %#v, want true", unitCost["available"])
	}
}

func TestAdminAnalyticsRequestsUsersAndChineseCSV(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	seedAnalyticsDimensions(t, db, adminSession.tenantID, adminSession.userID)
	createdAt := time.Date(2026, 8, 10, 3, 0, 0, 0, time.UTC)
	seedAnalyticsTask(t, db, analyticsTaskSeed{ID: "analytics-call-task", TenantID: adminSession.tenantID, UserID: adminSession.userID, Status: task.StatusSucceeded, StartedAt: createdAt, FinishedAt: createdAt.Add(2 * time.Second), CreatedAt: createdAt})
	statusOK := 200
	seedAPICallLog(t, db, database.APICallLog{ID: "analytics-call-ok", TenantID: adminSession.tenantID, TaskID: "analytics-call-task", ProviderID: "analytics-provider", ModelID: "analytics-model", Status: "SUCCESS", DurationMs: 800, RequestID: "req-ok", HTTPStatus: &statusOK, CreatedAt: createdAt})
	statusLimited := 429
	seedAPICallLog(t, db, database.APICallLog{ID: "analytics-call-failed", TenantID: adminSession.tenantID, TaskID: "analytics-call-task", ProviderID: "analytics-provider", ModelID: "analytics-model", Status: "FAILURE", DurationMs: 1800, RequestID: "req-failed", HTTPStatus: &statusLimited, ErrorCode: "RATE_LIMITED", ErrorMessage: "rate limited", CreatedAt: createdAt.Add(time.Minute)})
	seedAPICallLog(t, db, database.APICallLog{ID: "analytics-call-failed-retry", TenantID: adminSession.tenantID, TaskID: "analytics-call-task", ProviderID: "analytics-provider", ModelID: "analytics-model", Status: "FAILURE", DurationMs: 1600, RequestID: "req-failed-retry", HTTPStatus: &statusLimited, ErrorCode: "QUOTA_EXCEEDED", ErrorMessage: "quota exceeded", CreatedAt: createdAt.Add(2 * time.Minute)})

	requests := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/requests?from=2026-08-10&to=2026-08-10", nil, adminSession.cookies, nil)
	if requests.Code != http.StatusOK {
		t.Fatalf("analytics requests status = %d, want %d: %s", requests.Code, http.StatusOK, requests.Body.String())
	}
	requestsData := decodeData(t, requests)
	summary := objectField(t, requestsData, "summary")
	assertFloatField(t, summary, "callCount", 3)
	assertFloatField(t, summary, "successRate", 1.0/3.0)
	assertFloatField(t, summary, "p95DurationMs", 1800)
	errorGroups := requestsData["errorGroups"].([]any)
	if len(errorGroups) != 1 {
		t.Fatalf("analytics error groups = %#v, want one Chinese business category", errorGroups)
	}
	errorGroup := errorGroups[0].(map[string]any)
	if stringField(t, errorGroup, "errorCode") != "RATE_LIMITED" {
		t.Fatalf("analytics error category = %#v, want RATE_LIMITED", errorGroup)
	}
	assertFloatField(t, errorGroup, "count", 1)

	users := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/users?from=2026-08-10&to=2026-08-10", nil, adminSession.cookies, nil)
	if users.Code != http.StatusOK {
		t.Fatalf("analytics users status = %d, want %d: %s", users.Code, http.StatusOK, users.Body.String())
	}
	if len(recordsField(t, decodeData(t, users))) == 0 {
		t.Fatal("analytics users returned no records")
	}

	export := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/exports/requests?from=2026-08-10&to=2026-08-10", nil, adminSession.cookies, nil)
	if export.Code != http.StatusOK {
		t.Fatalf("analytics export status = %d, want %d: %s", export.Code, http.StatusOK, export.Body.String())
	}
	if contentType := export.Header().Get("Content-Type"); !strings.Contains(contentType, "text/csv") {
		t.Fatalf("analytics export content type = %q", contentType)
	}
	for _, required := range []string{"中转站", "模型调用次数", "调用成功率", "上海中转站"} {
		if !strings.Contains(export.Body.String(), required) {
			t.Fatalf("analytics export missing %q: %s", required, export.Body.String())
		}
	}

	tasksExport := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/exports/tasks?from=2026-08-10&to=2026-08-10", nil, adminSession.cookies, nil)
	if tasksExport.Code != http.StatusOK {
		t.Fatalf("analytics task export status = %d, want %d: %s", tasksExport.Code, http.StatusOK, tasksExport.Body.String())
	}
	for _, required := range []string{"商品主图", "已完成", "2秒", "创建时间（北京时间）"} {
		if !strings.Contains(tasksExport.Body.String(), required) {
			t.Fatalf("analytics task export missing %q: %s", required, tasksExport.Body.String())
		}
	}
	for _, forbidden := range []string{"SUCCEEDED", ",MAIN,"} {
		if strings.Contains(tasksExport.Body.String(), forbidden) {
			t.Fatalf("analytics task export leaked %q: %s", forbidden, tasksExport.Body.String())
		}
	}
}

func TestAdminAnalyticsUsersRequireUserReadAndRedactCostsWithoutUsageRead(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	seedAnalyticsDimensions(t, db, adminSession.tenantID, adminSession.userID)
	createdAt := time.Date(2026, 8, 10, 3, 0, 0, 0, time.UTC)
	seedAnalyticsTask(t, db, analyticsTaskSeed{ID: "analytics-user-only-task", TenantID: adminSession.tenantID, UserID: adminSession.userID, Status: task.StatusSucceeded, StartedAt: createdAt, FinishedAt: createdAt.Add(time.Second), CreatedAt: createdAt})
	seedUsageRecord(t, db, usageSeed{ID: "analytics-user-only-cost", TenantID: adminSession.tenantID, TaskID: "analytics-user-only-task", UserID: adminSession.userID, ProjectID: "analytics-project", ProviderID: "analytics-provider", ModelID: "analytics-model", ImageCount: 1, EstimatedCost: "7.25000000", Currency: "USD", CostStatus: usagecost.StatusCalculated, RawUsageJSON: `{}`, CreatedAt: createdAt})

	var adminRole database.Role
	if err := db.Where("tenant_id = ? AND code = ?", adminSession.tenantID, "admin").First(&adminRole).Error; err != nil {
		t.Fatalf("find admin role: %v", err)
	}
	var usagePermission database.Permission
	if err := db.Where("code = ?", "usage:read").First(&usagePermission).Error; err != nil {
		t.Fatalf("find usage permission: %v", err)
	}
	if err := db.Where("tenant_id = ? AND role_id = ? AND permission_id = ?", adminSession.tenantID, adminRole.ID, usagePermission.ID).Delete(&database.RolePermission{}).Error; err != nil {
		t.Fatalf("remove usage permission from isolated test role: %v", err)
	}

	users := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/users?from=2026-08-10&to=2026-08-10", nil, adminSession.cookies, nil)
	if users.Code != http.StatusOK {
		t.Fatalf("user-only analytics status = %d, want %d: %s", users.Code, http.StatusOK, users.Body.String())
	}
	records := recordsField(t, decodeData(t, users))
	if len(records) == 0 {
		t.Fatal("user-only analytics returned no users")
	}
	if costs, ok := records[0].(map[string]any)["costs"].([]any); !ok || len(costs) != 0 {
		t.Fatalf("user-only analytics costs = %#v, want redacted empty list", records[0].(map[string]any)["costs"])
	}
	if strings.Contains(users.Body.String(), "7.25000000") {
		t.Fatalf("user-only analytics leaked cost: %s", users.Body.String())
	}

	overview := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/overview?from=2026-08-10&to=2026-08-10", nil, adminSession.cookies, nil)
	if overview.Code != http.StatusForbidden {
		t.Fatalf("user-only overview status = %d, want %d", overview.Code, http.StatusForbidden)
	}
	export := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/exports/users?from=2026-08-10&to=2026-08-10", nil, adminSession.cookies, nil)
	if export.Code != http.StatusOK || strings.Contains(export.Body.String(), "7.25000000") {
		t.Fatalf("user-only export should succeed with redacted costs: status=%d body=%s", export.Code, export.Body.String())
	}
}

func TestAdminAnalyticsUserSearchStaysTenantScoped(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	seedUserAdminOtherTenant(t, db)

	response := performJSON(router, http.MethodGet, "/api/v1/admin/analytics/users?search=Tenant%20B", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("tenant-scoped user search status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if records := recordsField(t, decodeData(t, response)); len(records) != 0 {
		t.Fatalf("tenant-scoped user search returned cross-tenant records: %#v", records)
	}
	assertResponseExcludes(t, response.Body.String(), "tenant-b@example.com", "user-tenant-b")
}

func TestAnalyticsExportLabelsUseChineseFallbacks(t *testing.T) {
	tests := map[string]string{
		analyticsDimensionLabel("rawDimension"):  "其他维度",
		analyticsEntityStatusLabel("RAW_STATUS"): "未知状态",
		analyticsLifecycleLabel("RAW_LIFECYCLE"): "状态待确认",
		analyticsTaskStatusLabel("RAW_TASK"):     "未知任务状态",
		analyticsImageTypeLabel("RAW_IMAGE"):     "其他图片类型",
		analyticsCurrencyLabel("ZZZ"):            "其他币种",
	}
	for got, want := range tests {
		if got != want {
			t.Fatalf("analytics export label = %q, want %q", got, want)
		}
	}
}

func TestAdminAnalyticsRejectsInvalidRangeAndUnknownStatus(t *testing.T) {
	router, _, adminSession := newProjectRouteTestRouter(t)
	for _, path := range []string{
		"/api/v1/admin/analytics/overview?from=2024-01-01&to=2026-08-10",
		"/api/v1/admin/analytics/tasks?from=2026-08-10&to=2026-08-10&status=SUCCESS",
		"/api/v1/admin/analytics/requests?from=2026-08-10&to=2026-08-10&status=SUCCEEDED",
	} {
		response := performJSON(router, http.MethodGet, path, nil, adminSession.cookies, nil)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status = %d, want %d: %s", path, response.Code, http.StatusUnprocessableEntity, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "筛选条件无效") {
			t.Fatalf("%s did not return Chinese validation message: %s", path, response.Body.String())
		}
	}
}

type analyticsTaskSeed struct {
	ID           string
	TenantID     string
	UserID       string
	Status       string
	StartedAt    time.Time
	FinishedAt   time.Time
	CreatedAt    time.Time
	ErrorCode    string
	ErrorMessage string
}

func seedAnalyticsTask(t *testing.T, db *gorm.DB, seed analyticsTaskSeed) {
	t.Helper()
	var startedAt *time.Time
	var finishedAt *time.Time
	if !seed.StartedAt.IsZero() {
		startedAt = &seed.StartedAt
	}
	if !seed.FinishedAt.IsZero() {
		finishedAt = &seed.FinishedAt
	}
	if err := db.Create(&database.GenerationTask{
		ID: seed.ID, TenantID: seed.TenantID, ProjectID: "analytics-project", Type: task.TypeImageGeneration,
		ProviderID: "analytics-provider", ModelID: "analytics-model", Status: seed.Status, Prompt: "测试提示词", ImageType: "MAIN",
		ParamsJSON: `{}`, InputAssetIDsJSON: `[]`, Attempt: 1, MaxAttempts: 3, StartedAt: startedAt, FinishedAt: finishedAt,
		CreatedBy: seed.UserID, ErrorCode: seed.ErrorCode, ErrorMessage: seed.ErrorMessage, CreatedAt: seed.CreatedAt, UpdatedAt: seed.CreatedAt,
	}).Error; err != nil {
		t.Fatalf("seed analytics task %s: %v", seed.ID, err)
	}
}

func seedAnalyticsDimensions(t *testing.T, db *gorm.DB, tenantID string, userID string) {
	t.Helper()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if err := db.Create(&database.Project{ID: "analytics-project", TenantID: tenantID, Name: "商品主图项目", Status: "ACTIVE", CreatedBy: userID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed analytics project: %v", err)
	}
	if err := db.Create(&database.AIProvider{ID: "analytics-provider", TenantID: tenantID, Type: "OPENAI_COMPATIBLE", Name: "上海中转站", BaseURL: "https://example.com/v1", EncryptedAPIKey: "encrypted", APIKeyHint: "****test", Status: "ENABLED", TimeoutSeconds: 30, ConcurrencyLimit: 2, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed analytics provider: %v", err)
	}
	if err := db.Create(&database.AIModel{ID: "analytics-model", TenantID: tenantID, ProviderID: "analytics-provider", ModelName: "image-model-v1", DisplayName: "商品图模型", SupportsGenerate: true, MaxOutputCount: 4, SupportedSizesJSON: `[]`, SupportedQualitiesJSON: `[]`, SupportedOutputFormatsJSON: `[]`, PricingJSON: `{}`, Status: "ENABLED", CreatedBy: userID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed analytics model: %v", err)
	}
}
