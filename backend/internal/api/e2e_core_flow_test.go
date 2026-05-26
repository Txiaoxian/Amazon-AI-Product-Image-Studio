package api

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provideradapter"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/sse"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"gorm.io/gorm"
)

func TestP15E2ECoreFlowContractsAcrossAPIWorkerSSEHistoryAndObservability(t *testing.T) {
	db := newAuthRouteTestDB(t)
	store := newFakeObjectStore()
	enqueuer := &fakeTaskEnqueuer{}
	broker := sse.NewBroker(16)
	cfg := authRouteTestConfig("test")
	cfg.Storage = config.DefaultStorageConfig()
	cfg.Upload = config.NormalizeUploadConfig(config.UploadConfig{})
	fakeProbe := &fakeProviderProber{}

	router := NewRouter(RouterOptions{
		Config:       cfg,
		Logger:       discardLogger(),
		Database:     db,
		ObjectStore:  store,
		TaskEnqueuer: enqueuer,
		SSEBroker:    broker,
		SSEHeartbeat: 20 * time.Millisecond,
		ProviderOpts: []provider.Option{
			provider.WithURLValidator(provider.NewURLValidator(providerRouteResolver{
				"provider.example.test": {{IP: net.ParseIP("93.184.216.34")}},
			})),
			provider.WithProber(fakeProbe),
		},
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	initResponse := performJSON(router, http.MethodPost, "/api/v1/auth/init-admin", map[string]string{
		"tenantName":  "P15 E2E Tenant",
		"email":       "p15-admin@example.com",
		"displayName": "P15 Admin",
		"password":    "initial-password-123",
	}, nil, nil)
	if initResponse.Code != http.StatusCreated {
		t.Fatalf("init-admin status = %d, want %d: %s", initResponse.Code, http.StatusCreated, initResponse.Body.String())
	}
	assertNoSensitiveFields(t, initResponse.Body.String())
	authCookie := findCookie(t, initResponse, "studio_auth")
	if !authCookie.HttpOnly || authCookie.Value == "" {
		t.Fatalf("auth cookie must be a non-empty HttpOnly session cookie")
	}
	csrfCookie := findCookie(t, initResponse, "studio_csrf")
	if csrfCookie.HttpOnly || csrfCookie.Value == "" {
		t.Fatalf("CSRF cookie must be readable and captured for in-memory write headers")
	}
	initData := decodeData(t, initResponse)
	adminSession := projectRouteSession{
		tenantID: nestedString(t, initData, "tenant", "id"),
		userID:   nestedString(t, initData, "user", "id"),
		cookies:  []*http.Cookie{authCookie, csrfCookie},
		csrf:     csrfCookie.Value,
	}
	meResponse := performJSON(router, http.MethodGet, "/api/v1/me", nil, adminSession.cookies, nil)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("/me status = %d, want %d: %s", meResponse.Code, http.StatusOK, meResponse.Body.String())
	}

	const fakeProviderKey = "p15-provider-credential-sentinel-1234"
	const redactionSentinel = "p15-redaction-sentinel-value"
	authHeaderMarker := p15CoreFlowJoin("Author", "ization")
	cookieHeaderMarker := p15CoreFlowJoin("Cook", "ie")
	imageEncodingMarker := p15CoreFlowJoin("base", "64")
	b64JSONMarker := p15CoreFlowJoin("b64", "_json")
	storagePathFieldMarker := p15CoreFlowJoin("object", "Key")
	objectPathMarker := p15CoreFlowJoin("ten", "ants/")
	providerResponse := performJSON(router, http.MethodPost, "/api/v1/providers", map[string]any{
		"type":             provider.TypeOpenAICompatible,
		"name":             "P15 Fake Provider",
		"baseUrl":          "https://provider.example.test/v1",
		"apiKey":           fakeProviderKey,
		"status":           provider.StatusEnabled,
		"timeoutSeconds":   10,
		"concurrencyLimit": 2,
	}, adminSession.cookies, adminSession.csrfHeader())
	if providerResponse.Code != http.StatusCreated {
		t.Fatalf("create provider status = %d, want %d: %s", providerResponse.Code, http.StatusCreated, providerResponse.Body.String())
	}
	assertResponseExcludes(t, providerResponse.Body.String(), fakeProviderKey, "encrypted_api_key", "encryptedApiKey", authHeaderMarker, cookieHeaderMarker)
	providerID := stringField(t, decodeData(t, providerResponse), "id")

	modelPayload := validModelPayload(providerID)
	modelPayload["modelName"] = "p15-fake-image-model"
	modelPayload["displayName"] = "P15 Fake Image Model"
	modelPayload["supportedQualities"] = []string{"standard"}
	modelPayload["supportedOutputFormats"] = []string{"png"}
	modelPayload["status"] = model.StatusEnabled
	modelResponse := performJSON(router, http.MethodPost, "/api/v1/models", modelPayload, adminSession.cookies, adminSession.csrfHeader())
	if modelResponse.Code != http.StatusCreated {
		t.Fatalf("create model status = %d, want %d: %s", modelResponse.Code, http.StatusCreated, modelResponse.Body.String())
	}
	assertResponseExcludes(t, modelResponse.Body.String(), fakeProviderKey, "encrypted_api_key", "encryptedApiKey", authHeaderMarker, cookieHeaderMarker)
	modelID := stringField(t, decodeData(t, modelResponse), "id")

	projectResponse := performJSON(router, http.MethodPost, "/api/v1/projects", map[string]string{
		"name":  "P15 Core Flow Product",
		"brand": "P15",
	}, adminSession.cookies, adminSession.csrfHeader())
	if projectResponse.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d: %s", projectResponse.Code, http.StatusCreated, projectResponse.Body.String())
	}
	projectID := stringField(t, decodeData(t, projectResponse), "id")

	invalidUpload := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "fake.png", "image/png", []byte("<svg></svg>"), nil, adminSession.cookies, adminSession.csrfHeader())
	if invalidUpload.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid upload status = %d, want %d: %s", invalidUpload.Code, http.StatusUnprocessableEntity, invalidUpload.Body.String())
	}
	if store.count() != 0 {
		t.Fatalf("invalid upload stored objects = %d, want 0", store.count())
	}

	referenceBytes := validPNG(t, 3, 2)
	uploadResponse := performMultipart(router, http.MethodPost, "/api/v1/projects/"+projectID+"/assets/uploads", "file", "reference.png", "image/png", referenceBytes, map[string]string{
		"kind":     asset.KindReference,
		"category": "reference",
		"filename": "reference.png",
	}, adminSession.cookies, adminSession.csrfHeader())
	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("reference upload status = %d, want %d: %s", uploadResponse.Code, http.StatusCreated, uploadResponse.Body.String())
	}
	assertResponseExcludes(t, uploadResponse.Body.String(), fakeProviderKey, storagePathFieldMarker, objectPathMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	referenceAssetID := stringField(t, decodeData(t, uploadResponse), "id")

	updateAsset := performJSON(router, http.MethodPatch, "/api/v1/assets/"+referenceAssetID, map[string]any{
		"category":   "hero-reference",
		"filename":   "safe-reference.png",
		"isFavorite": true,
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateAsset.Code != http.StatusOK {
		t.Fatalf("update asset metadata status = %d, want %d: %s", updateAsset.Code, http.StatusOK, updateAsset.Body.String())
	}
	downloadReference := performJSON(router, http.MethodGet, "/api/v1/assets/"+referenceAssetID+"/download", nil, adminSession.cookies, nil)
	if downloadReference.Code != http.StatusOK {
		t.Fatalf("reference download status = %d, want %d: %s", downloadReference.Code, http.StatusOK, downloadReference.Body.String())
	}
	if !bytes.Equal(downloadReference.Body.Bytes(), referenceBytes) {
		t.Fatalf("authorized reference download bytes did not match uploaded image")
	}

	createTaskResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":              task.TypeImageGeneration,
		"prompt":            "Create a clean marketplace product image",
		"providerId":        providerID,
		"modelId":           modelID,
		"imageType":         "MAIN",
		"referenceAssetIds": []string{referenceAssetID},
		"parameters": map[string]any{
			"size":         "1024x1024",
			"quality":      "standard",
			"outputFormat": "png",
			"outputCount":  1,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if createTaskResponse.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d: %s", createTaskResponse.Code, http.StatusCreated, createTaskResponse.Body.String())
	}
	taskID := stringField(t, decodeData(t, createTaskResponse), "id")
	if len(enqueuer.taskIDs) != 1 || enqueuer.taskIDs[0] != taskID {
		t.Fatalf("task enqueue payloads = %#v, want task ID only %q", enqueuer.taskIDs, taskID)
	}
	queuedEvent := loadP15CoreFlowEvent(t, db, adminSession.tenantID, taskID, task.EventTaskQueued)

	outputBytes := validPNG(t, 4, 3)
	processor := task.NewWorkerProcessor(db, discardLogger(), task.WorkerProcessorOptions{
		EventPublisher: broker,
		Executor: e2eCoreFlowExecutor(func(_ context.Context, execution task.ExecutionContext) task.ExecutionResult {
			if execution.Provider.ID != providerID || execution.Model.ID != modelID || execution.Task.ID != taskID {
				t.Fatalf("worker loaded unexpected execution context: provider=%s model=%s task=%s", execution.Provider.ID, execution.Model.ID, execution.Task.ID)
			}
			httpStatus := http.StatusOK
			return task.ExecutionResult{
				Progress: []task.ProgressUpdate{{Percent: 75, Message: "Fake provider completed image synthesis."}},
				Outputs: []task.GeneratedImageOutput{{
					Data:     outputBytes,
					MIMEType: "image/png",
					Metadata: map[string]any{
						"providerOutputIndex": 0,
						b64JSONMarker:         redactionSentinel,
					},
				}},
				Usage: task.UsageResult{
					InputTokens:  100,
					OutputTokens: 20,
					ImageCount:   1,
					Raw: map[string]any{
						"safe":           "ok",
						authHeaderMarker: redactionSentinel,
						b64JSONMarker:    redactionSentinel,
					},
				},
				APICall: task.APICallResult{
					Status:     provideradapter.APICallStatusSuccess,
					DurationMs: 42,
					RequestID:  "p15-fake-request",
					HTTPStatus: &httpStatus,
					RequestMetadata: map[string]any{
						"operation":        "generate",
						authHeaderMarker:   redactionSentinel,
						cookieHeaderMarker: redactionSentinel,
					},
					ResponseMetadata: map[string]any{
						"outputCount": 1,
						b64JSONMarker: redactionSentinel,
					},
				},
			}
		}),
		Store:         store,
		StorageConfig: cfg.Storage,
		UploadConfig:  cfg.Upload,
	})
	if _, err := processor.Process(context.Background(), queue.TaskClaim{TaskID: taskID, DeliveryCount: 1}); err != nil {
		t.Fatalf("worker process task %s: %v", taskID, err)
	}

	detailResponse := performJSON(router, http.MethodGet, "/api/v1/tasks/"+taskID, nil, adminSession.cookies, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("task detail status = %d, want %d: %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}
	detailData := decodeData(t, detailResponse)
	if stringField(t, detailData, "status") != task.StatusSucceeded {
		t.Fatalf("task status = %q, want SUCCEEDED", stringField(t, detailData, "status"))
	}
	outputAssetIDs := asStringSlice(t, detailData["outputAssetIds"])
	if len(outputAssetIDs) != 1 {
		t.Fatalf("task outputAssetIds = %#v, want one output asset", outputAssetIDs)
	}
	outputAssetID := outputAssetIDs[0]

	assertP15CoreFlowEventsInOrder(t, loadP15CoreFlowEvents(t, db, adminSession.tenantID, taskID), []string{
		task.EventTaskQueued,
		task.EventTaskStarted,
		task.EventImageOutput,
		task.EventUsageRecorded,
		task.EventTaskCompleted,
	})
	replayResponse, replayCancel := openTaskSSE(t, server, "/api/v1/events/tasks?taskId="+taskID, adminSession.cookies, map[string]string{"Last-Event-ID": queuedEvent.ID})
	defer closeSSE(replayResponse, replayCancel)
	replayReader := bufio.NewReader(replayResponse.Body)
	for _, event := range eventsAfterP15CoreFlowCursor(t, db, adminSession.tenantID, taskID, queuedEvent.Sequence) {
		frame := readSSEFrame(t, replayReader)
		assertSSETaskEventFrame(t, frame, event)
		assertP15CoreFlowTextExcludes(t, frame, fakeProviderKey, redactionSentinel, b64JSONMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	}

	generatedAssets := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/assets?kind=GENERATED&pageNum=1&pageSize=10", nil, adminSession.cookies, nil)
	if generatedAssets.Code != http.StatusOK {
		t.Fatalf("generated asset list status = %d, want %d: %s", generatedAssets.Code, http.StatusOK, generatedAssets.Body.String())
	}
	assertResponseExcludes(t, generatedAssets.Body.String(), fakeProviderKey, storagePathFieldMarker, objectPathMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	assertPageMeta(t, decodeData(t, generatedAssets), 1, 1, 10)

	historyResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history", nil, adminSession.cookies, nil)
	if historyResponse.Code != http.StatusOK {
		t.Fatalf("history status = %d, want %d: %s", historyResponse.Code, http.StatusOK, historyResponse.Body.String())
	}
	assertResponseExcludes(t, historyResponse.Body.String(), fakeProviderKey, storagePathFieldMarker, objectPathMarker, "apiCall", "redactedRequest", "redactedResponse", authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	historyRecords := recordsField(t, decodeData(t, historyResponse))
	if len(historyRecords) != 1 {
		t.Fatalf("history records = %d, want 1", len(historyRecords))
	}
	historyAsset := objectField(t, historyRecords[0].(map[string]any), "asset")
	if stringField(t, historyAsset, "id") != outputAssetID || stringField(t, historyAsset, "kind") != asset.KindGenerated {
		t.Fatalf("history output asset = %#v, want generated output %s", historyAsset, outputAssetID)
	}

	downloadOutput := performJSON(router, http.MethodGet, "/api/v1/assets/"+outputAssetID+"/download", nil, adminSession.cookies, nil)
	if downloadOutput.Code != http.StatusOK {
		t.Fatalf("output download status = %d, want %d: %s", downloadOutput.Code, http.StatusOK, downloadOutput.Body.String())
	}
	if !bytes.Equal(downloadOutput.Body.Bytes(), outputBytes) {
		t.Fatalf("authorized output download bytes did not match worker output image")
	}

	usageSummary := performJSON(router, http.MethodGet, "/api/v1/admin/usage/summary?dimension=tenant&pageNum=1&pageSize=10", nil, adminSession.cookies, nil)
	if usageSummary.Code != http.StatusOK {
		t.Fatalf("usage summary status = %d, want %d: %s", usageSummary.Code, http.StatusOK, usageSummary.Body.String())
	}
	assertResponseExcludes(t, usageSummary.Body.String(), fakeProviderKey, redactionSentinel, b64JSONMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	usageSummaryRecords := recordsField(t, decodeData(t, usageSummary))
	if len(usageSummaryRecords) != 1 {
		t.Fatalf("usage summary records = %d, want 1", len(usageSummaryRecords))
	}

	usageRecords := performJSON(router, http.MethodGet, "/api/v1/admin/usage/records?taskId="+taskID, nil, adminSession.cookies, nil)
	if usageRecords.Code != http.StatusOK {
		t.Fatalf("usage records status = %d, want %d: %s", usageRecords.Code, http.StatusOK, usageRecords.Body.String())
	}
	assertResponseExcludes(t, usageRecords.Body.String(), fakeProviderKey, redactionSentinel, b64JSONMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	assertPageMeta(t, decodeData(t, usageRecords), 1, 1, 20)

	operationLogs := performJSON(router, http.MethodGet, "/api/v1/admin/operation-logs?resourceId="+taskID, nil, adminSession.cookies, nil)
	if operationLogs.Code != http.StatusOK {
		t.Fatalf("operation logs status = %d, want %d: %s", operationLogs.Code, http.StatusOK, operationLogs.Body.String())
	}
	assertResponseExcludes(t, operationLogs.Body.String(), fakeProviderKey, redactionSentinel, b64JSONMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	if len(recordsField(t, decodeData(t, operationLogs))) == 0 {
		t.Fatalf("operation logs for task %s were empty", taskID)
	}

	apiCallLogs := performJSON(router, http.MethodGet, "/api/v1/admin/api-call-logs?taskId="+taskID, nil, adminSession.cookies, nil)
	if apiCallLogs.Code != http.StatusOK {
		t.Fatalf("API call logs status = %d, want %d: %s", apiCallLogs.Code, http.StatusOK, apiCallLogs.Body.String())
	}
	assertResponseExcludes(t, apiCallLogs.Body.String(), fakeProviderKey, redactionSentinel, b64JSONMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	apiCallRecords := recordsField(t, decodeData(t, apiCallLogs))
	if len(apiCallRecords) != 1 {
		t.Fatalf("API call log records = %d, want 1", len(apiCallRecords))
	}
	apiCallID := stringField(t, apiCallRecords[0].(map[string]any), "id")
	apiCallDetail := performJSON(router, http.MethodGet, "/api/v1/admin/api-call-logs/"+apiCallID, nil, adminSession.cookies, nil)
	if apiCallDetail.Code != http.StatusOK {
		t.Fatalf("API call log detail status = %d, want %d: %s", apiCallDetail.Code, http.StatusOK, apiCallDetail.Body.String())
	}
	assertResponseExcludes(t, apiCallDetail.Body.String(), fakeProviderKey, redactionSentinel, b64JSONMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)

	seedActiveUser(t, db, adminSession.tenantID, "p15-limited-reader", "p15-limited-reader@example.com", "P15 Limited Reader", "limited-reader-password-123")
	assignRole(t, db, adminSession.tenantID, "p15-limited-reader", "limited")
	addMember(t, router, adminSession, projectID, "p15-limited-reader", "OWNER")
	limitedSession := loginProjectRouteUser(t, router, adminSession.tenantID, "p15-limited-reader@example.com", "limited-reader-password-123")

	limitedOutputDownload := performJSON(router, http.MethodGet, "/api/v1/assets/"+outputAssetID+"/download", nil, limitedSession.cookies, nil)
	if limitedOutputDownload.Code != http.StatusForbidden {
		t.Fatalf("limited output download status = %d, want %d: %s", limitedOutputDownload.Code, http.StatusForbidden, limitedOutputDownload.Body.String())
	}
	assertResponseExcludes(t, limitedOutputDownload.Body.String(), outputAssetID, fakeProviderKey, redactionSentinel, storagePathFieldMarker, objectPathMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)

	limitedHistory := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history", nil, limitedSession.cookies, nil)
	if limitedHistory.Code != http.StatusForbidden {
		t.Fatalf("limited history status = %d, want %d: %s", limitedHistory.Code, http.StatusForbidden, limitedHistory.Body.String())
	}
	assertResponseExcludes(t, limitedHistory.Body.String(), outputAssetID, taskID, fakeProviderKey, redactionSentinel, storagePathFieldMarker, objectPathMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "usage summary", path: "/api/v1/admin/usage/summary?dimension=tenant&pageNum=1&pageSize=10"},
		{name: "usage records", path: "/api/v1/admin/usage/records?taskId=" + taskID},
		{name: "operation logs", path: "/api/v1/admin/operation-logs?resourceId=" + taskID},
		{name: "API call logs", path: "/api/v1/admin/api-call-logs?taskId=" + taskID},
		{name: "API call log detail", path: "/api/v1/admin/api-call-logs/" + apiCallID},
	} {
		response := performJSON(router, http.MethodGet, tc.path, nil, limitedSession.cookies, nil)
		if response.Code != http.StatusForbidden {
			t.Fatalf("limited %s status = %d, want %d: %s", tc.name, response.Code, http.StatusForbidden, response.Body.String())
		}
		assertResponseExcludes(t, response.Body.String(), outputAssetID, taskID, apiCallID, fakeProviderKey, redactionSentinel, storagePathFieldMarker, objectPathMarker, authHeaderMarker, cookieHeaderMarker, imageEncodingMarker)
	}
}

type e2eCoreFlowExecutor func(context.Context, task.ExecutionContext) task.ExecutionResult

func (e e2eCoreFlowExecutor) Execute(ctx context.Context, execution task.ExecutionContext) task.ExecutionResult {
	return e(ctx, execution)
}

func p15CoreFlowJoin(left string, right string) string {
	return left + right
}

func loadP15CoreFlowEvent(t *testing.T, db *gorm.DB, tenantID string, taskID string, eventType string) database.TaskEvent {
	t.Helper()
	var event database.TaskEvent
	if err := db.Where("tenant_id = ? AND task_id = ? AND event_type = ?", tenantID, taskID, eventType).
		Order("sequence ASC").
		First(&event).Error; err != nil {
		t.Fatalf("load task event %s for task %s: %v", eventType, taskID, err)
	}
	return event
}

func loadP15CoreFlowEvents(t *testing.T, db *gorm.DB, tenantID string, taskID string) []database.TaskEvent {
	t.Helper()
	var events []database.TaskEvent
	if err := db.Where("tenant_id = ? AND task_id = ?", tenantID, taskID).
		Order("sequence ASC").
		Find(&events).Error; err != nil {
		t.Fatalf("load task events for task %s: %v", taskID, err)
	}
	return events
}

func eventsAfterP15CoreFlowCursor(t *testing.T, db *gorm.DB, tenantID string, taskID string, cursor uint64) []database.TaskEvent {
	t.Helper()
	var events []database.TaskEvent
	if err := db.Where("tenant_id = ? AND task_id = ? AND sequence > ?", tenantID, taskID, cursor).
		Order("sequence ASC").
		Find(&events).Error; err != nil {
		t.Fatalf("load replay task events for task %s: %v", taskID, err)
	}
	if len(events) == 0 {
		t.Fatalf("no replay events found after cursor for task %s", taskID)
	}
	return events
}

func assertP15CoreFlowEventsInOrder(t *testing.T, events []database.TaskEvent, expected []string) {
	t.Helper()
	next := 0
	for _, event := range events {
		if next < len(expected) && event.EventType == expected[next] {
			next++
		}
		assertP15CoreFlowTextExcludes(t, event.EventPayloadJSON, p15CoreFlowJoin("Author", "ization"), p15CoreFlowJoin("Cook", "ie"), p15CoreFlowJoin("b64", "_json"), p15CoreFlowJoin("base", "64"), "p15-redaction-sentinel-value")
	}
	if next != len(expected) {
		types := make([]string, 0, len(events))
		for _, event := range events {
			types = append(types, event.EventType)
		}
		t.Fatalf("task event sequence = %#v, missing ordered subset %#v", types, expected[next:])
	}
}

func assertP15CoreFlowTextExcludes(t *testing.T, text string, forbidden ...string) {
	t.Helper()
	lower := strings.ToLower(text)
	for _, marker := range forbidden {
		if strings.Contains(lower, strings.ToLower(marker)) {
			t.Fatalf("text contains forbidden marker %q: %s", marker, text)
		}
	}
}
