package api

import (
	"net/http"
	"testing"
	"time"

	assetpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"gorm.io/gorm"
)

func TestTaskHistoryRoutesListGeneratedAndEditedPairsWithFilteringAndSafety(t *testing.T) {
	router, db, _, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "History Project")
	now := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)

	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "history-task-generated", "history-asset-generated", assetpkg.KindGenerated, now.Add(2*time.Minute), false)
	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "history-task-edited", "history-asset-edited", assetpkg.KindEdited, now.Add(time.Minute), false)
	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "history-task-reference", "history-asset-reference", assetpkg.KindReference, now.Add(3*time.Minute), false)
	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "history-task-deleted", "history-asset-deleted", assetpkg.KindGenerated, now.Add(4*time.Minute), true)
	seedTaskHistoryAsset(t, db, adminSession.tenantID, projectID, adminSession.userID, "history-asset-orphan", assetpkg.KindGenerated, now.Add(5*time.Minute), false)
	seedTaskHistoryAPICallLog(t, db, adminSession.tenantID, "history-task-generated", "sk-history-secret")

	response := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("history list status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertResponseExcludes(t, response.Body.String(), "history-asset-reference", "history-asset-deleted", "history-asset-orphan", "minio-secret-object-key", "objectKey", "thumbnailObjectKey", "sk-history-secret", "apiCall", "redactedRequest", "redactedResponse", "authorization", "cookie", "base64")

	data := decodeData(t, response)
	assertPageMeta(t, data, 2, 1, 20)
	records := recordsField(t, data)
	assertHistoryRecord(t, records[0].(map[string]any), "history-asset-generated", assetpkg.KindGenerated, "history-task-generated")
	assertHistoryRecord(t, records[1].(map[string]any), "history-asset-edited", assetpkg.KindEdited, "history-task-edited")
}

func TestTaskHistoryRoutesEnforceTenantProjectAndRBACAuthorization(t *testing.T) {
	router, db, _, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "History Auth Project")
	otherProjectID := createTaskTestProject(t, router, adminSession, "History Non Member Project")
	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "auth-task", "auth-asset", assetpkg.KindGenerated, time.Now().UTC(), false)

	unauthenticated := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history", nil, nil, nil)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated history status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}

	seedActiveUser(t, db, adminSession.tenantID, "limited-history", "limited-history@example.com", "Limited History", "limited-history-password-123")
	assignRole(t, db, adminSession.tenantID, "limited-history", "limited")
	addMember(t, router, adminSession, projectID, "limited-history", project.RoleOwner)
	limitedSession := loginProjectRouteUser(t, router, adminSession.tenantID, "limited-history@example.com", "limited-history-password-123")
	limitedResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history", nil, limitedSession.cookies, nil)
	if limitedResponse.Code != http.StatusForbidden {
		t.Fatalf("limited member history status = %d, want %d: %s", limitedResponse.Code, http.StatusForbidden, limitedResponse.Body.String())
	}

	seedActiveUser(t, db, adminSession.tenantID, "seller-history", "seller-history@example.com", "Seller History", "seller-history-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-history", "seller")
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-history@example.com", "seller-history-password-123")
	nonMemberResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+otherProjectID+"/history", nil, sellerSession.cookies, nil)
	if nonMemberResponse.Code != http.StatusForbidden {
		t.Fatalf("non-member history status = %d, want %d: %s", nonMemberResponse.Code, http.StatusForbidden, nonMemberResponse.Body.String())
	}

	seedOtherTenantTask(t, db)
	crossTenantProject := performJSON(router, http.MethodGet, "/api/v1/projects/project-tenant-b/history", nil, adminSession.cookies, nil)
	if crossTenantProject.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant project history status = %d, want %d: %s", crossTenantProject.Code, http.StatusNotFound, crossTenantProject.Body.String())
	}
	assertResponseExcludes(t, crossTenantProject.Body.String(), "tenant-b", "task-tenant-b", "project-tenant-b")
}

func TestTaskHistoryRoutesTenantScopedOutputLinksDoNotLeakForeignIDs(t *testing.T) {
	router, db, _, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "History Tenant Scope Project")
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)

	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "visible-task", "visible-asset", assetpkg.KindGenerated, now, false)
	seedOtherTenantTask(t, db)
	seedTaskHistoryAsset(t, db, adminSession.tenantID, projectID, adminSession.userID, "dirty-cross-tenant-task-asset", assetpkg.KindGenerated, now.Add(time.Minute), false)
	seedTaskHistoryOutput(t, db, adminSession.tenantID, "task-tenant-b", "dirty-cross-tenant-task-asset", 1)
	seedTaskHistoryOutput(t, db, adminSession.tenantID, "visible-task", "foreign-asset-id", 2)

	response := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history", nil, adminSession.cookies, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("history tenant scope status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	assertResponseExcludes(t, response.Body.String(), "task-tenant-b", "dirty-cross-tenant-task-asset", "foreign-asset-id", "tenant-b")

	data := decodeData(t, response)
	assertPageMeta(t, data, 1, 1, 20)
	records := recordsField(t, data)
	assertHistoryRecord(t, records[0].(map[string]any), "visible-asset", assetpkg.KindGenerated, "visible-task")
	taskData := objectField(t, records[0].(map[string]any), "task")
	outputIDs := asStringSlice(t, taskData["outputAssetIds"])
	if len(outputIDs) != 1 || outputIDs[0] != "visible-asset" {
		t.Fatalf("history task outputAssetIds = %#v, want only visible-asset", outputIDs)
	}
}

func TestTaskHistoryRoutesKindFilterAndPaginationAreDeterministic(t *testing.T) {
	router, db, _, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "History Pagination Project")
	sameTime := time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-a", "asset-a", assetpkg.KindGenerated, sameTime, false)
	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-c", "asset-c", assetpkg.KindEdited, sameTime, false)
	seedTaskHistoryRecord(t, db, adminSession.tenantID, projectID, adminSession.userID, "task-b", "asset-b", assetpkg.KindGenerated, sameTime, false)

	pageOne := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history?pageNum=1&pageSize=2", nil, adminSession.cookies, nil)
	if pageOne.Code != http.StatusOK {
		t.Fatalf("history page one status = %d, want %d: %s", pageOne.Code, http.StatusOK, pageOne.Body.String())
	}
	pageOneData := decodeData(t, pageOne)
	assertPageMeta(t, pageOneData, 3, 1, 2)
	pageOneRecords := recordsField(t, pageOneData)
	assertHistoryRecord(t, pageOneRecords[0].(map[string]any), "asset-c", assetpkg.KindEdited, "task-c")
	assertHistoryRecord(t, pageOneRecords[1].(map[string]any), "asset-b", assetpkg.KindGenerated, "task-b")

	pageTwo := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history?pageNum=2&pageSize=2", nil, adminSession.cookies, nil)
	if pageTwo.Code != http.StatusOK {
		t.Fatalf("history page two status = %d, want %d: %s", pageTwo.Code, http.StatusOK, pageTwo.Body.String())
	}
	pageTwoRecords := recordsField(t, decodeData(t, pageTwo))
	assertHistoryRecord(t, pageTwoRecords[0].(map[string]any), "asset-a", assetpkg.KindGenerated, "task-a")

	generatedOnly := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history?kind=GENERATED", nil, adminSession.cookies, nil)
	if generatedOnly.Code != http.StatusOK {
		t.Fatalf("generated history status = %d, want %d: %s", generatedOnly.Code, http.StatusOK, generatedOnly.Body.String())
	}
	generatedRecords := recordsField(t, decodeData(t, generatedOnly))
	assertHistoryRecord(t, generatedRecords[0].(map[string]any), "asset-b", assetpkg.KindGenerated, "task-b")
	assertHistoryRecord(t, generatedRecords[1].(map[string]any), "asset-a", assetpkg.KindGenerated, "task-a")

	editedOnly := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history?kind=EDITED", nil, adminSession.cookies, nil)
	if editedOnly.Code != http.StatusOK {
		t.Fatalf("edited history status = %d, want %d: %s", editedOnly.Code, http.StatusOK, editedOnly.Body.String())
	}
	editedRecords := recordsField(t, decodeData(t, editedOnly))
	assertHistoryRecord(t, editedRecords[0].(map[string]any), "asset-c", assetpkg.KindEdited, "task-c")

	capped := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/history?pageSize=1000", nil, adminSession.cookies, nil)
	if capped.Code != http.StatusOK {
		t.Fatalf("capped history status = %d, want %d: %s", capped.Code, http.StatusOK, capped.Body.String())
	}
	assertPageMeta(t, decodeData(t, capped), 3, 1, 100)
}

func TestTaskHistoryRoutesRejectInvalidQuery(t *testing.T) {
	router, _, _, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "History Invalid Query Project")

	for _, path := range []string{
		"/api/v1/projects/" + projectID + "/history?pageNum=0",
		"/api/v1/projects/" + projectID + "/history?pageNum=abc",
		"/api/v1/projects/" + projectID + "/history?pageSize=0",
		"/api/v1/projects/" + projectID + "/history?pageSize=abc",
		"/api/v1/projects/" + projectID + "/history?kind=REFERENCE",
	} {
		response := performJSON(router, http.MethodGet, path, nil, adminSession.cookies, nil)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status = %d, want %d: %s", path, response.Code, http.StatusUnprocessableEntity, response.Body.String())
		}
	}
}

func seedTaskHistoryRecord(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string, taskID string, assetID string, kind string, assetCreatedAt time.Time, deleted bool) {
	t.Helper()
	seedTaskHistoryTask(t, db, tenantID, projectID, userID, taskID, assetCreatedAt.Add(-time.Minute))
	seedTaskHistoryAsset(t, db, tenantID, projectID, userID, assetID, kind, assetCreatedAt, deleted)
	seedTaskHistoryOutput(t, db, tenantID, taskID, assetID, 1)
}

func seedTaskHistoryTask(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string, taskID string, createdAt time.Time) {
	t.Helper()
	if err := db.Create(&database.GenerationTask{
		ID:                taskID,
		TenantID:          tenantID,
		ProjectID:         projectID,
		Type:              task.TypeImageGeneration,
		ProviderID:        "provider-history",
		ModelID:           "model-history",
		Status:            task.StatusSucceeded,
		Prompt:            "History prompt",
		ImageType:         "MAIN",
		ParamsJSON:        `{}`,
		InputAssetIDsJSON: `[]`,
		Attempt:           1,
		MaxAttempts:       3,
		CreatedBy:         userID,
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt,
	}).Error; err != nil {
		t.Fatalf("seed history task %s: %v", taskID, err)
	}
}

func seedTaskHistoryAsset(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string, assetID string, kind string, createdAt time.Time, deleted bool) {
	t.Helper()
	record := database.ImageAsset{
		ID:        assetID,
		TenantID:  tenantID,
		ProjectID: projectID,
		Kind:      kind,
		Category:  "MAIN",
		Filename:  assetID + ".png",
		ObjectKey: "minio-secret-object-key/" + assetID,
		MimeType:  "image/png",
		SizeBytes: 2048,
		Width:     1024,
		Height:    1024,
		SHA256:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CreatedBy: userID,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	if deleted {
		record.DeletedAt = gorm.DeletedAt{Time: createdAt.Add(time.Minute), Valid: true}
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed history asset %s: %v", assetID, err)
	}
}

func seedTaskHistoryOutput(t *testing.T, db *gorm.DB, tenantID string, taskID string, assetID string, outputIndex int) {
	t.Helper()
	if err := db.Create(&database.TaskOutput{
		ID:          "output-" + tenantID + "-" + taskID + "-" + assetID,
		TenantID:    tenantID,
		TaskID:      taskID,
		AssetID:     assetID,
		OutputIndex: outputIndex,
		CreatedAt:   time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("seed history output %s/%s: %v", taskID, assetID, err)
	}
}

func seedTaskHistoryAPICallLog(t *testing.T, db *gorm.DB, tenantID string, taskID string, secret string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&database.APICallLog{
		ID:                   "api-call-history",
		TenantID:             tenantID,
		TaskID:               taskID,
		ProviderID:           "provider-history",
		ModelID:              "model-history",
		Status:               "SUCCESS",
		DurationMs:           100,
		RequestID:            "request-history",
		ErrorCode:            "",
		ErrorMessage:         "",
		RedactedRequestJSON:  `{"authorization":"` + secret + `"}`,
		RedactedResponseJSON: `{"b64_json":"base64"}`,
		CreatedAt:            now,
	}).Error; err != nil {
		t.Fatalf("seed history api call log: %v", err)
	}
}

func assertHistoryRecord(t *testing.T, record map[string]any, assetID string, kind string, taskID string) {
	t.Helper()
	assetData := objectField(t, record, "asset")
	taskData := objectField(t, record, "task")
	if stringField(t, assetData, "id") != assetID {
		t.Fatalf("history asset id = %q, want %q", stringField(t, assetData, "id"), assetID)
	}
	if stringField(t, assetData, "kind") != kind {
		t.Fatalf("history asset kind = %q, want %q", stringField(t, assetData, "kind"), kind)
	}
	if stringField(t, taskData, "id") != taskID {
		t.Fatalf("history task id = %q, want %q", stringField(t, taskData, "id"), taskID)
	}
}
