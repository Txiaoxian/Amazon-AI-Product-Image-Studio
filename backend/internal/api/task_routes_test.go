package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	assetpkg "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
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

func TestTaskRoutesCreateWithOmittedProviderModelUsesTenantDefaults(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Task Defaults Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "defaults-happy", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
	seedTaskDefaultsSetting(t, db, adminSession.tenantID, providerID, modelID)

	response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":       task.TypeImageGeneration,
		"prompt":     "Use tenant task defaults",
		"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
	}, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusCreated {
		t.Fatalf("default-backed create status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	data := decodeData(t, response)
	taskID := stringField(t, data, "id")
	if stringField(t, data, "providerId") != providerID || stringField(t, data, "modelId") != modelID {
		t.Fatalf("created provider/model = %s/%s, want %s/%s", stringField(t, data, "providerId"), stringField(t, data, "modelId"), providerID, modelID)
	}
	if len(enqueuer.taskIDs) != 1 || enqueuer.taskIDs[0] != taskID {
		t.Fatalf("default-backed enqueue payloads = %#v, want task ID %q", enqueuer.taskIDs, taskID)
	}
	assertTaskOperationLogs(t, db, []string{"task.create"})
	assertNoTaskEventOrOperationLogSecrets(t, db)
}

func TestTaskRoutesRejectMixedMissingProviderModelIDs(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Task Mixed Defaults Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "mixed-missing", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
	seedTaskDefaultsSetting(t, db, adminSession.tenantID, providerID, modelID)

	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "missing provider only",
			body: map[string]any{
				"type":       task.TypeImageGeneration,
				"prompt":     "Missing provider only",
				"modelId":    modelID,
				"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
			},
		},
		{
			name: "missing model only",
			body: map[string]any{
				"type":       task.TypeImageGeneration,
				"prompt":     "Missing model only",
				"providerId": providerID,
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
	assertNoTaskCreateSideEffects(t, db, adminSession.tenantID, projectID, enqueuer)
}

func TestTaskRoutesRejectInvalidRuntimeTaskDefaultsWithoutSideEffects(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, db *gorm.DB, tenantID string) (string, string)
		body  func(projectID string) map[string]any
	}{
		{
			name: "missing taskDefaults",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				return "", ""
			},
		},
		{
			name: "cleared taskDefaults",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				seedClearedTaskDefaultsSetting(t, db, tenantID)
				return "", ""
			},
		},
		{
			name: "disabled default provider",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				return seedTaskProviderModel(t, db, tenantID, "defaults-disabled-provider", provider.StatusDisabled, model.StatusEnabled, true, true, false, false, 1)
			},
		},
		{
			name: "deleted default provider",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				providerID, modelID := seedTaskProviderModel(t, db, tenantID, "defaults-deleted-provider", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
				if err := db.Where("tenant_id = ? AND id = ?", tenantID, providerID).Delete(&database.AIProvider{}).Error; err != nil {
					t.Fatalf("delete default provider: %v", err)
				}
				return providerID, modelID
			},
		},
		{
			name: "cross-tenant default provider",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				_, modelID := seedTaskProviderModel(t, db, tenantID, "defaults-cross-provider-local", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
				seedOtherTenantTask(t, db)
				return "provider-tenant-b", modelID
			},
		},
		{
			name: "disabled default model",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				return seedTaskProviderModel(t, db, tenantID, "defaults-disabled-model", provider.StatusEnabled, model.StatusDisabled, true, true, false, false, 1)
			},
		},
		{
			name: "deleted default model",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				providerID, modelID := seedTaskProviderModel(t, db, tenantID, "defaults-deleted-model", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
				if err := db.Where("tenant_id = ? AND id = ?", tenantID, modelID).Delete(&database.AIModel{}).Error; err != nil {
					t.Fatalf("delete default model: %v", err)
				}
				return providerID, modelID
			},
		},
		{
			name: "cross-tenant default model",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				providerID, _ := seedTaskProviderModel(t, db, tenantID, "defaults-cross-model-local", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
				seedOtherTenantTask(t, db)
				return providerID, "model-tenant-b"
			},
		},
		{
			name: "default model belongs to another provider",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				providerID, _ := seedTaskProviderModel(t, db, tenantID, "defaults-provider-a", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
				_, otherModelID := seedTaskProviderModel(t, db, tenantID, "defaults-provider-b", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
				return providerID, otherModelID
			},
		},
		{
			name: "default model does not support generation",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				return seedTaskProviderModel(t, db, tenantID, "defaults-edit-only", provider.StatusEnabled, model.StatusEnabled, false, true, false, false, 1)
			},
		},
		{
			name: "default-backed unsupported output count",
			setup: func(t *testing.T, db *gorm.DB, tenantID string) (string, string) {
				return seedTaskProviderModel(t, db, tenantID, "defaults-no-n", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)
			},
			body: func(projectID string) map[string]any {
				return map[string]any{
					"type":       task.TypeImageGeneration,
					"prompt":     "Unsupported output count",
					"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png", "outputCount": 2},
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
			projectID := createTaskTestProject(t, router, adminSession, "Invalid Defaults "+tc.name)
			providerID, modelID := tc.setup(t, db, adminSession.tenantID)
			if providerID != "" || modelID != "" {
				seedTaskDefaultsSetting(t, db, adminSession.tenantID, providerID, modelID)
			}
			body := map[string]any{
				"type":       task.TypeImageGeneration,
				"prompt":     "Default-backed request should fail",
				"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
			}
			if tc.body != nil {
				body = tc.body(projectID)
			}
			response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", body, adminSession.cookies, adminSession.csrfHeader())
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("default-backed create status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
			}
			assertNoTaskCreateSideEffects(t, db, adminSession.tenantID, projectID, enqueuer)
			assertResponseExcludes(t, response.Body.String(), "provider-tenant-b", "model-tenant-b", "encrypted", "api_key", "apikey", "secret")
		})
	}
}

func TestTaskRoutesRejectDefaultModelWithoutEditCapability(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Invalid Edit Defaults")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "defaults-generate-only", provider.StatusEnabled, model.StatusEnabled, true, false, false, false, 1)
	seedTaskDefaultsSetting(t, db, adminSession.tenantID, providerID, modelID)
	sourceAssetID := seedTaskAsset(t, db, adminSession.tenantID, projectID, adminSession.userID, assetpkg.KindGenerated, "defaults-edit-source")

	response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":              task.TypeImageEdit,
		"prompt":            "Default model cannot edit",
		"editSourceAssetId": sourceAssetID,
		"parameters":        map[string]any{"size": "1024x1024", "outputFormat": "png"},
	}, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("default edit create status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}
	assertNoTaskCreateSideEffects(t, db, adminSession.tenantID, projectID, enqueuer)
	assertResponseExcludes(t, response.Body.String(), providerID, modelID, "encrypted", "api_key", "apikey", "secret")
}

func TestTaskRoutesRejectOmittedProviderModelAfterTaskDefaultsClearedViaSettingsAPI(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Cleared Defaults Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "defaults-clear-api", provider.StatusEnabled, model.StatusEnabled, true, true, false, false, 1)

	setDefaults := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskDefaults": map[string]any{
			"defaultProviderId": providerID,
			"defaultModelId":    modelID,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if setDefaults.Code != http.StatusOK {
		t.Fatalf("set taskDefaults status = %d, want %d: %s", setDefaults.Code, http.StatusOK, setDefaults.Body.String())
	}
	clearDefaults := performJSON(router, http.MethodPatch, "/api/v1/admin/system-settings", map[string]any{
		"taskDefaults": map[string]any{
			"defaultProviderId": nil,
			"defaultModelId":    nil,
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if clearDefaults.Code != http.StatusOK {
		t.Fatalf("clear taskDefaults status = %d, want %d: %s", clearDefaults.Code, http.StatusOK, clearDefaults.Body.String())
	}

	response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":       task.TypeImageGeneration,
		"prompt":     "Defaults were cleared",
		"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
	}, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create after clear status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}
	assertNoTaskCreateSideEffects(t, db, adminSession.tenantID, projectID, enqueuer)
	assertResponseExcludes(t, response.Body.String(), providerID, modelID, "encrypted", "api_key", "apikey", "secret")
}

func TestTaskRoutesAllowGeneratedAndEditedEditSourceAssetsOnlyForEditSource(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Edit Source Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "edit-source", provider.StatusEnabled, model.StatusEnabled, false, true, true, false, 1)
	generatedAssetID := seedTaskAsset(t, db, adminSession.tenantID, projectID, adminSession.userID, assetpkg.KindGenerated, "generated-edit-source")
	editedAssetID := seedTaskAsset(t, db, adminSession.tenantID, projectID, adminSession.userID, assetpkg.KindEdited, "edited-edit-source")

	for _, tc := range []struct {
		name    string
		assetID string
	}{
		{name: "generated edit source", assetID: generatedAssetID},
		{name: "edited edit source", assetID: editedAssetID},
	} {
		response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
			"type":              task.TypeImageEdit,
			"prompt":            "Edit the selected asset",
			"providerId":        providerID,
			"modelId":           modelID,
			"editSourceAssetId": tc.assetID,
			"parameters":        map[string]any{"size": "1024x1024", "outputFormat": "png"},
		}, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusCreated {
			t.Fatalf("%s status = %d, want %d: %s", tc.name, response.Code, http.StatusCreated, response.Body.String())
		}
		data := decodeData(t, response)
		inputAssetIDs := asStringSlice(t, data["inputAssetIds"])
		if len(inputAssetIDs) != 1 || inputAssetIDs[0] != tc.assetID {
			t.Fatalf("%s inputAssetIds = %#v, want [%s]", tc.name, inputAssetIDs, tc.assetID)
		}
	}
	if len(enqueuer.taskIDs) != 2 {
		t.Fatalf("successful edit source requests enqueued %d tasks, want 2", len(enqueuer.taskIDs))
	}
}

func TestTaskRoutesRejectNonReferenceAssetsInReferenceAssetIDs(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Reference Kind Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "reference-kind", provider.StatusEnabled, model.StatusEnabled, true, true, true, false, 1)
	generatedAssetID := seedTaskAsset(t, db, adminSession.tenantID, projectID, adminSession.userID, assetpkg.KindGenerated, "generated-reference")
	editedAssetID := seedTaskAsset(t, db, adminSession.tenantID, projectID, adminSession.userID, assetpkg.KindEdited, "edited-reference")

	for _, tc := range []struct {
		name    string
		assetID string
	}{
		{name: "generated reference", assetID: generatedAssetID},
		{name: "edited reference", assetID: editedAssetID},
	} {
		response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
			"type":              task.TypeImageGeneration,
			"prompt":            "Generate with invalid reference",
			"providerId":        providerID,
			"modelId":           modelID,
			"referenceAssetIds": []string{tc.assetID},
			"parameters":        map[string]any{"size": "1024x1024", "outputFormat": "png"},
		}, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status = %d, want %d: %s", tc.name, response.Code, http.StatusUnprocessableEntity, response.Body.String())
		}
	}
	if len(enqueuer.taskIDs) != 0 {
		t.Fatalf("rejected reference requests must not enqueue tasks: %#v", enqueuer.taskIDs)
	}
}

func TestTaskRoutesRejectEditSourceAssetsOutsideCurrentProjectAndTenant(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Scoped Edit Source Project")
	otherProjectID := createTaskTestProject(t, router, adminSession, "Other Edit Source Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "edit-source-scope", provider.StatusEnabled, model.StatusEnabled, false, true, true, false, 1)
	crossProjectAssetID := seedTaskAsset(t, db, adminSession.tenantID, otherProjectID, adminSession.userID, assetpkg.KindGenerated, "cross-project-edit-source")
	crossTenantAssetID := seedTaskCrossTenantAsset(t, db, assetpkg.KindGenerated, "cross-tenant-edit-source")

	for _, tc := range []struct {
		name    string
		assetID string
	}{
		{name: "cross-project edit source", assetID: crossProjectAssetID},
		{name: "cross-tenant edit source", assetID: crossTenantAssetID},
	} {
		response := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
			"type":              task.TypeImageEdit,
			"prompt":            "Edit an asset outside current scope",
			"providerId":        providerID,
			"modelId":           modelID,
			"editSourceAssetId": tc.assetID,
			"parameters":        map[string]any{"size": "1024x1024", "outputFormat": "png"},
		}, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status = %d, want %d: %s", tc.name, response.Code, http.StatusUnprocessableEntity, response.Body.String())
		}
	}
	if len(enqueuer.taskIDs) != 0 {
		t.Fatalf("rejected scoped edit source requests must not enqueue tasks: %#v", enqueuer.taskIDs)
	}
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

func TestTaskRoutesRejectUnauthorizedCancelRetryWithoutSideEffects(t *testing.T) {
	router, db, enqueuer, adminSession := newTaskRouteTestRouter(t)
	projectID := createTaskTestProject(t, router, adminSession, "Task Object Permission Project")
	providerID, modelID := seedTaskProviderModel(t, db, adminSession.tenantID, "object-permission", provider.StatusEnabled, model.StatusEnabled, true, false, false, false, 1)

	createResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"type":       task.TypeImageGeneration,
		"prompt":     "Object-level permission test",
		"providerId": providerID,
		"modelId":    modelID,
		"parameters": map[string]any{"size": "1024x1024", "outputFormat": "png"},
	}, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	taskID := stringField(t, decodeData(t, createResponse), "id")

	seedActiveUser(t, db, adminSession.tenantID, "viewer-task", "viewer-task@example.com", "Viewer Task", "viewer-task-password-123")
	assignRole(t, db, adminSession.tenantID, "viewer-task", "viewer")
	addMember(t, router, adminSession, projectID, "viewer-task", project.RoleViewer)
	viewerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "viewer-task@example.com", "viewer-task-password-123")

	for _, path := range []string{"/api/v1/tasks/" + taskID + "/cancel", "/api/v1/tasks/" + taskID + "/retry"} {
		response := performJSON(router, http.MethodPost, path, nil, viewerSession.cookies, viewerSession.csrfHeader())
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d, want %d: %s", path, response.Code, http.StatusForbidden, response.Body.String())
		}
		var record database.GenerationTask
		if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, taskID).First(&record).Error; err != nil {
			t.Fatalf("reload task after forbidden action: %v", err)
		}
		if record.Status != task.StatusQueued || record.Attempt != 1 {
			t.Fatalf("forbidden action changed task state: %#v", record)
		}
	}
	if len(enqueuer.taskIDs) != 1 {
		t.Fatalf("forbidden cancel/retry enqueued extra tasks: %#v", enqueuer.taskIDs)
	}

	seedOtherTenantTask(t, db)
	for _, path := range []string{"/api/v1/tasks/task-tenant-b/cancel", "/api/v1/tasks/task-tenant-b/retry"} {
		response := performJSON(router, http.MethodPost, path, nil, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s cross-tenant status = %d, want %d: %s", path, response.Code, http.StatusNotFound, response.Body.String())
		}
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

func seedTaskDefaultsSetting(t *testing.T, db *gorm.DB, tenantID string, providerID string, modelID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&database.SystemSetting{
		ID:        "setting-task-defaults",
		TenantID:  tenantID,
		Key:       settings.KeyTaskDefaults,
		ValueJSON: `{"defaultProviderId":"` + providerID + `","defaultModelId":"` + modelID + `"}`,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed task defaults setting: %v", err)
	}
}

func seedClearedTaskDefaultsSetting(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Create(&database.SystemSetting{
		ID:        "setting-task-defaults",
		TenantID:  tenantID,
		Key:       settings.KeyTaskDefaults,
		ValueJSON: `{"defaultProviderId":null,"defaultModelId":null}`,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed cleared task defaults setting: %v", err)
	}
}

func seedTaskReferenceAsset(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string, suffix string) string {
	return seedTaskAsset(t, db, tenantID, projectID, userID, assetpkg.KindReference, suffix)
}

func seedTaskAsset(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string, kind string, suffix string) string {
	t.Helper()
	now := time.Now().UTC()
	assetID := "asset-" + suffix
	if err := db.Create(&database.ImageAsset{
		ID:        assetID,
		TenantID:  tenantID,
		ProjectID: projectID,
		Kind:      kind,
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
		t.Fatalf("seed %s asset %s: %v", kind, suffix, err)
	}
	return assetID
}

func seedTaskCrossTenantAsset(t *testing.T, db *gorm.DB, kind string, suffix string) string {
	t.Helper()
	now := time.Now().UTC()
	tenantID := "tenant-asset-scope"
	userID := "user-asset-scope"
	projectID := "project-asset-scope"
	if err := db.Create(&database.Tenant{ID: tenantID, Name: "Asset Scope Tenant", Status: auth.TenantStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed asset scope tenant: %v", err)
	}
	if err := db.Create(&database.User{ID: userID, TenantID: tenantID, Email: "asset-scope@example.com", DisplayName: "Asset Scope", PasswordHash: "hash", Status: auth.UserStatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed asset scope user: %v", err)
	}
	if err := db.Create(&database.Project{ID: projectID, TenantID: tenantID, Name: "Asset Scope Project", Status: project.StatusActive, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed asset scope project: %v", err)
	}
	return seedTaskAsset(t, db, tenantID, projectID, userID, kind, suffix)
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

func assertNoTaskCreateSideEffects(t *testing.T, db *gorm.DB, tenantID string, projectID string, enqueuer *fakeTaskEnqueuer) {
	t.Helper()
	if len(enqueuer.taskIDs) != 0 {
		t.Fatalf("rejected request enqueued tasks: %#v", enqueuer.taskIDs)
	}
	var taskCount int64
	if err := db.Model(&database.GenerationTask{}).Where("tenant_id = ? AND project_id = ?", tenantID, projectID).Count(&taskCount).Error; err != nil {
		t.Fatalf("count tasks after rejected request: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("rejected request created %d tasks, want 0", taskCount)
	}
	var logCount int64
	if err := db.Model(&database.OperationLog{}).Where("tenant_id = ? AND action = ?", tenantID, "task.create").Count(&logCount).Error; err != nil {
		t.Fatalf("count task.create logs after rejected request: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("rejected request wrote %d task.create logs, want 0", logCount)
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
