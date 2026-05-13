package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"gorm.io/gorm"
)

func TestTaskRoutesCreateListDetailCancelRetryAndAudit(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Task Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "happy", provider.StatusEnabled, model.StatusEnabled, true, true, true, true, 4)
	assetID := seedTaskReferenceAsset(t, db, adminSession.tenantID, projectID, adminSession.userID, "reference-a")

	createResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":              task.TypeImageGeneration,
		"prompt":            "Generate a clean marketplace image with hidden words Authorization Cookie base64 sk-secret",
		"providerId":        providerID,
		"modelId":           modelID,
		"imageType":         "MAIN",
		"referenceAssetIds": []string{assetID},
		"parameters": map[string]any{
			"size":         "1024x1024",
			"quality":      "high",
			"outputFormat": "png",
			"outputCount":  2,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	createData := decodeData(t, createResponse)
	taskID := stringField(t, createData, "id")
	if stringField(t, createData, "status") != task.StatusQueued {
		t.Fatalf("created task status = %q, want QUEUED", stringField(t, createData, "status"))
	}
	if len(enqueuer.taskIDs) != 1 || enqueuer.taskIDs[0] != taskID {
		t.Fatalf("enqueue payloads = %#v, want task ID only %q", enqueuer.taskIDs, taskID)
	}
	assertNoTaskEventOrOperationLogSecrets(t, db)

	listResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/tasks?status=QUEUED&type=IMAGE_GENERATION", nil, adminSession.cookies, nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list tasks status = %d, want %d: %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	listData := decodeData(t, listResponse)
	if total, ok := listData["total"].(float64); !ok || total != 1 {
		t.Fatalf("task list total = %#v, want 1", listData["total"])
	}

	detailResponse := performJSON(router, http.MethodGet, "/api/v1/tasks/"+taskID, nil, adminSession.cookies, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("task detail status = %d, want %d: %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}
	if stringField(t, decodeData(t, detailResponse), "projectId") != projectID {
		t.Fatalf("detail projectId mismatch: %s", detailResponse.Body.String())
	}

	cancelResponse := performJSON(router, http.MethodPost, "/api/v1/tasks/"+taskID+"/cancel", nil, adminSession.cookies, adminSession.csrfHeader())
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("cancel task status = %d, want %d: %s", cancelResponse.Code, http.StatusOK, cancelResponse.Body.String())
	}
	if stringField(t, decodeData(t, cancelResponse), "status") != task.StatusCancelled {
		t.Fatalf("cancelled status response = %s", cancelResponse.Body.String())
	}

	retryResponse := performJSON(router, http.MethodPost, "/api/v1/tasks/"+taskID+"/retry", nil, adminSession.cookies, adminSession.csrfHeader())
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("retry task status = %d, want %d: %s", retryResponse.Code, http.StatusOK, retryResponse.Body.String())
	}
	retryData := decodeData(t, retryResponse)
	if stringField(t, retryData, "status") != task.StatusQueued {
		t.Fatalf("retry status = %q, want QUEUED", stringField(t, retryData, "status"))
	}
	if retryData["attempt"].(float64) != 2 {
		t.Fatalf("retry attempt = %#v, want 2", retryData["attempt"])
	}
	if len(enqueuer.taskIDs) != 2 || enqueuer.taskIDs[1] != taskID {
		t.Fatalf("retry enqueue payloads = %#v, want second task ID %q", enqueuer.taskIDs, taskID)
	}

	assertTaskEvents(t, db, taskID, []string{task.EventTaskQueued, task.EventTaskCancelled, task.EventTaskRetried, task.EventTaskQueued})
	assertTaskEventsHaveStableReplayCursor(t, db, taskID)
	assertTaskOperationLogs(t, db, []string{"task.create", "task.cancel", "task.retry"})
	assertNoTaskEventOrOperationLogSecrets(t, db)
}

func TestTaskRoutesEnforceTenantAndProjectAuthorization(t *testing.T) {
	router, db, _, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Restricted Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "authz", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)

	seedActiveUser(t, db, adminSession.tenantID, "seller-task", "seller-task@example.com", "Seller Task", "seller-task-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-task", "seller")
	addViewerResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "seller-task",
		"role":   project.RoleViewer,
	}, adminSession.cookies, adminSession.csrfHeader())
	if addViewerResponse.Code != http.StatusCreated {
		t.Fatalf("add seller viewer status = %d: %s", addViewerResponse.Code, addViewerResponse.Body.String())
	}
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-task@example.com", "seller-task-password-123")
	viewerCreateResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":       task.TypeImageGeneration,
		"prompt":     "Viewer cannot create",
		"providerId": providerID,
		"modelId":    modelID,
		"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
	}, sellerSession.cookies, sellerSession.csrfHeader())
	if viewerCreateResponse.Code != http.StatusForbidden {
		t.Fatalf("viewer task create status = %d, want %d", viewerCreateResponse.Code, http.StatusForbidden)
	}

	seedOtherTenantTask(t, db)
	crossTenantDetail := performJSON(router, http.MethodGet, "/api/v1/tasks/task-tenant-b", nil, adminSession.cookies, nil)
	if crossTenantDetail.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant task detail status = %d, want %d", crossTenantDetail.Code, http.StatusNotFound)
	}

	crossTenantProviderResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":       task.TypeImageGeneration,
		"prompt":     "Cannot use another tenant provider",
		"providerId": "provider-tenant-b",
		"modelId":    modelID,
		"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
	}, adminSession.cookies, adminSession.csrfHeader())
	if crossTenantProviderResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cross-tenant provider create status = %d, want %d", crossTenantProviderResponse.Code, http.StatusUnprocessableEntity)
	}
}

func TestTaskRoutesValidateProviderModelCapabilitiesAndServerOwnedFields(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Capability Project")
	disabledProviderID, disabledProviderModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "disabled-provider", provider.StatusDisabled, model.StatusEnabled, true, false, false, false, 1)
	enabledProviderID, disabledModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "disabled-model", provider.StatusEnabled, model.StatusDisabled, true, false, false, false, 1)
	_, editOnlyModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "edit-only", provider.StatusEnabled, model.StatusEnabled, false, true, false, false, 1)
	_, noNModelID := seedTaskProviderModel(t, db, adminSession.tenantID, "no-n", provider.StatusEnabled, model.StatusEnabled, true, false, false, false, 1)

	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "disabled provider",
			body: map[string]any{
				"type":       task.TypeImageGeneration,
				"prompt":     "Blocked",
				"providerId": disabledProviderID,
				"modelId":    disabledProviderModelID,
				"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
			},
		},
		{
			name: "disabled model",
			body: map[string]any{
				"type":       task.TypeImageGeneration,
				"prompt":     "Blocked",
				"providerId": enabledProviderID,
				"modelId":    disabledModelID,
				"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
			},
		},
		{
			name: "generation unsupported",
			body: map[string]any{
				"type":       task.TypeImageGeneration,
				"prompt":     "Blocked",
				"providerId": enabledProviderID,
				"modelId":    editOnlyModelID,
				"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
			},
		},
		{
			name: "unsupported output count",
			body: map[string]any{
				"type":       task.TypeImageGeneration,
				"prompt":     "Blocked",
				"providerId": enabledProviderID,
				"modelId":    noNModelID,
				"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png", "outputCount": 2},
			},
		},
		{
			name: "server owned field",
			body: map[string]any{
				"type":       task.TypeImageGeneration,
				"prompt":     "Blocked",
				"providerId": enabledProviderID,
				"modelId":    noNModelID,
				"tenantId":   adminSession.tenantID,
				"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
			},
		},
	}

	for _, tc := range cases {
		response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", tc.body, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status = %d, want %d: %s", tc.name, response.Code, http.StatusUnprocessableEntity, response.Body.String())
		}
	}
	if len(enqueuer.taskIDs) != 0 {
		t.Fatalf("rejected requests must not enqueue tasks: %#v", enqueuer.taskIDs)
	}
}

func TestTaskRoutesHandleEnqueueFailureWithoutLeavingQueuedZombie(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Enqueue Failure Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "enqueue", provider.StatusEnabled, model.StatusEnabled, true, false, false, false, 1)
	enqueuer.err = errors.New("redis unavailable")

	response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":       task.TypeImageGeneration,
		"prompt":     "Queue failure must become failed state",
		"providerId": providerID,
		"modelId":    modelID,
		"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
	}, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("enqueue failure status = %d, want %d: %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "redis") {
		t.Fatalf("enqueue failure response leaked backend detail: %s", response.Body.String())
	}
	if len(enqueuer.taskIDs) != 1 {
		t.Fatalf("enqueue attempts = %#v, want one task ID", enqueuer.taskIDs)
	}

	var record database.GenerationTask
	if err := db.Where("tenant_id = ? AND project_id = ?", adminSession.tenantID, projectID).First(&record).Error; err != nil {
		t.Fatalf("load failed queued task: %v", err)
	}
	if record.Status != task.StatusFailed || record.ErrorCode != "ENQUEUE_FAILED" {
		t.Fatalf("enqueue failure task = %#v, want FAILED/ENQUEUE_FAILED", record)
	}
	assertTaskEvents(t, db, record.ID, []string{task.EventTaskQueued, task.EventTaskFailed})
}

type fakeTaskEnqueuer struct {
	taskIDs []string
	err     error
}

func (e *fakeTaskEnqueuer) EnqueueTask(_ context.Context, taskID string) error {
	e.taskIDs = append(e.taskIDs, taskID)
	return e.err
}

func newTaskRouteTestRouter(t *testing.T) (http.Handler, *gorm.DB, *fakeTaskEnqueuer, projectRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	enqueuer := &fakeTaskEnqueuer{}
	router := NewRouter(RouterOptions{
		Config:       authRouteTestConfig("test"),
		Logger:       discardLogger(),
		Database:     db,
		TaskEnqueuer: enqueuer,
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
	return router, db, enqueuer, projectRouteSession{
		tenantID: nestedString(t, data, "tenant", "id"),
		userID:   nestedString(t, data, "user", "id"),
		cookies:  []*http.Cookie{authCookie, csrfCookie},
		csrf:     csrfCookie.Value,
	}
}

func createTaskTestProject(t *testing.T, router http.Handler, session projectRouteSession, name string) string {
	t.Helper()
	response := performJSON(router, http.MethodPost, "/api/v1/projects", map[string]string{"name": name}, session.cookies, session.csrfHeader())
	if response.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	return stringField(t, decodeData(t, response), "id")
}

func seedTaskProviderModel(t *testing.T, db *gorm.DB, tenantID string, suffix string, providerStatus string, modelStatus string, supportsGenerate bool, supportsEdit bool, supportsMultiReference bool, supportsN bool, maxOutputCount int) (string, string) {
	t.Helper()
	now := time.Now().UTC()
	providerID := "provider-" + suffix
	modelID := "model-" + suffix
	if err := db.Create(&database.AIProvider{
		ID:               providerID,
		TenantID:         tenantID,
		Type:             provider.TypeOpenAICompatible,
		Name:             "Provider " + suffix,
		BaseURL:          "https://api.openai.com/v1",
		EncryptedAPIKey:  "encrypted",
		APIKeyHint:       "****test",
		Status:           providerStatus,
		TimeoutSeconds:   10,
		ConcurrencyLimit: 2,
		CreatedBy:        "seed",
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("seed provider %s: %v", suffix, err)
	}
	if maxOutputCount == 0 {
		maxOutputCount = 1
	}
	if err := db.Create(&database.AIModel{
		ID:                         modelID,
		TenantID:                   tenantID,
		ProviderID:                 providerID,
		ModelName:                  "model-" + suffix,
		DisplayName:                "Model " + suffix,
		SupportsGenerate:           supportsGenerate,
		SupportsEdit:               supportsEdit,
		SupportsMultiReference:     supportsMultiReference,
		SupportsN:                  supportsN,
		MaxOutputCount:             maxOutputCount,
		SupportedSizesJSON:         `["1024x1024","1536x1024"]`,
		SupportedQualitiesJSON:     `["standard","high"]`,
		SupportedOutputFormatsJSON: `["png","jpeg","webp"]`,
		PricingJSON:                `{"currency":"USD","unitPrices":{}}`,
		Status:                     modelStatus,
		CreatedBy:                  "seed",
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}).Error; err != nil {
		t.Fatalf("seed model %s: %v", suffix, err)
	}
	return providerID, modelID
}

func seedTaskReferenceAsset(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string, suffix string) string {
	t.Helper()
	now := time.Now().UTC()
	assetID := "asset-" + suffix
	if err := db.Create(&database.ImageAsset{
		ID:        assetID,
		TenantID:  tenantID,
		ProjectID: projectID,
		Kind:      "REFERENCE",
		Category:  "MAIN",
		Filename:  suffix + ".png",
		ObjectKey: "tenant/project/" + suffix + ".png",
		MimeType:  "image/png",
		SizeBytes: 1024,
		Width:     512,
		Height:    512,
		SHA256:    strings.Repeat("a", 64),
		CreatedBy: userID,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed reference asset %s: %v", suffix, err)
	}
	return assetID
}

func seedOtherTenantTask(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&database.Tenant{ID: "tenant-b", Name: "Tenant B", Status: auth.TenantStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant B: %v", err)
	}
	if err := db.Create(&database.User{ID: "user-tenant-b", TenantID: "tenant-b", Email: "tenant-b@example.com", DisplayName: "Tenant B", PasswordHash: "hash", Status: auth.UserStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant B user: %v", err)
	}
	if err := db.Create(&database.Project{ID: "project-tenant-b", TenantID: "tenant-b", Name: "Tenant B Project", Status: project.StatusActive, CreatedBy: "user-tenant-b", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant B project: %v", err)
	}
	if err := db.Create(&database.AIProvider{ID: "provider-tenant-b", TenantID: "tenant-b", Type: provider.TypeOpenAICompatible, Name: "Tenant B Provider", BaseURL: "https://api.openai.com/v1", EncryptedAPIKey: "encrypted", APIKeyHint: "****test", Status: provider.StatusEnabled, TimeoutSeconds: 10, ConcurrencyLimit: 1, CreatedBy: "user-tenant-b", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant B provider: %v", err)
	}
	if err := db.Create(&database.AIModel{ID: "model-tenant-b", TenantID: "tenant-b", ProviderID: "provider-tenant-b", ModelName: "tenant-b-model", DisplayName: "Tenant B Model", SupportsGenerate: true, MaxOutputCount: 1, SupportedSizesJSON: `["1024x1024"]`, SupportedQualitiesJSON: `["high"]`, SupportedOutputFormatsJSON: `["png"]`, PricingJSON: `{"currency":"USD","unitPrices":{}}`, Status: model.StatusEnabled, CreatedBy: "user-tenant-b", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant B model: %v", err)
	}
	if err := db.Create(&database.GenerationTask{ID: "task-tenant-b", TenantID: "tenant-b", ProjectID: "project-tenant-b", Type: task.TypeImageGeneration, ProviderID: "provider-tenant-b", ModelID: "model-tenant-b", Status: task.StatusQueued, Prompt: "hidden", ParamsJSON: `{}`, InputAssetIDsJSON: `[]`, Attempt: 1, MaxAttempts: 3, CreatedBy: "user-tenant-b", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant B task: %v", err)
	}
}

func assertTaskEvents(t *testing.T, db *gorm.DB, taskID string, expected []string) {
	t.Helper()
	var events []database.TaskEvent
	if err := db.Where("task_id = ?", taskID).Order("sequence ASC").Find(&events).Error; err != nil {
		t.Fatalf("load task events: %v", err)
	}
	if len(events) != len(expected) {
		t.Fatalf("task event count = %d, want %d: %#v", len(events), len(expected), events)
	}
	for index, event := range events {
		if event.EventType != expected[index] {
			t.Fatalf("event %d type = %q, want %q; events = %#v", index, event.EventType, expected[index], events)
		}
	}
}

func assertTaskEventsHaveStableReplayCursor(t *testing.T, db *gorm.DB, taskID string) {
	t.Helper()
	var events []database.TaskEvent
	if err := db.Where("task_id = ?", taskID).Order("sequence ASC").Find(&events).Error; err != nil {
		t.Fatalf("load task events by sequence: %v", err)
	}
	for index, event := range events {
		if event.Sequence == 0 {
			t.Fatalf("event %d missing replay sequence: %#v", index, event)
		}
		if event.ID != task.EventIDFromSequence(event.Sequence) {
			t.Fatalf("event %d id = %q, want %q", index, event.ID, task.EventIDFromSequence(event.Sequence))
		}
		if index > 0 && event.Sequence <= events[index-1].Sequence {
			t.Fatalf("event sequence did not increase: previous=%d current=%d", events[index-1].Sequence, event.Sequence)
		}
	}
}

func assertTaskOperationLogs(t *testing.T, db *gorm.DB, expectedActions []string) {
	t.Helper()
	var logs []database.OperationLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}
	seen := map[string]bool{}
	for _, log := range logs {
		seen[log.Action] = true
	}
	for _, action := range expectedActions {
		if !seen[action] {
			t.Fatalf("missing task operation log %s; logs = %#v", action, logs)
		}
	}
}

func assertNoTaskEventOrOperationLogSecrets(t *testing.T, db *gorm.DB) {
	t.Helper()
	forbidden := []string{"authorization", "cookie", "api_key", "apikey", "secret", "base64", "sk-secret"}
	var events []database.TaskEvent
	if err := db.Find(&events).Error; err != nil {
		t.Fatalf("load task events: %v", err)
	}
	for _, event := range events {
		body := strings.ToLower(event.EventPayloadJSON)
		for _, word := range forbidden {
			if strings.Contains(body, word) {
				t.Fatalf("task event payload contains %q: %#v", word, event)
			}
		}
	}
	var logs []database.OperationLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}
	for _, log := range logs {
		metadata := strings.ToLower(log.MetadataJSON)
		for _, word := range forbidden {
			if strings.Contains(metadata, word) {
				t.Fatalf("operation log metadata contains %q: %#v", word, log)
			}
		}
	}
}
