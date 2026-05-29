package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	modelcap "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"gorm.io/gorm"
)

func TestModelRoutesCRUDEnableDisableDeleteAndAudit(t *testing.T) {
	router, db, adminSession := newModelRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-model-a", "Studio Provider")

	createResponse := performJSON(router, http.MethodPost, "/api/v1/models", validModelPayload(providerID), adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create model status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	assertModelResponseHasNoSecrets(t, createResponse.Body.String())
	createData := decodeData(t, createResponse)
	modelID := stringField(t, createData, "id")
	if stringField(t, createData, "tenantId") != adminSession.tenantID {
		t.Fatalf("created model tenantId = %q, want %q", stringField(t, createData, "tenantId"), adminSession.tenantID)
	}
	if stringField(t, createData, "providerName") != "Studio Provider" {
		t.Fatalf("created model providerName = %q", stringField(t, createData, "providerName"))
	}
	if maxOutputCount, ok := createData["maxOutputCount"].(float64); !ok || maxOutputCount != 4 {
		t.Fatalf("created maxOutputCount = %#v, want 4", createData["maxOutputCount"])
	}

	var record database.AIModel
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, modelID).First(&record).Error; err != nil {
		t.Fatalf("load model: %v", err)
	}
	if record.ProviderID != providerID || record.TenantID != adminSession.tenantID {
		t.Fatalf("model stored without expected tenant/provider scope: %#v", record)
	}
	if !json.Valid([]byte(record.PricingJSON)) || !json.Valid([]byte(record.SupportedSizesJSON)) {
		t.Fatalf("model capability JSON was not stored as valid JSON: %#v", record)
	}

	listResponse := performJSON(router, http.MethodGet, "/api/v1/models?providerId="+providerID+"&status=ENABLED&capability=generate", nil, adminSession.cookies, nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list models status = %d, want %d: %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	assertModelResponseHasNoSecrets(t, listResponse.Body.String())
	if total, ok := decodeData(t, listResponse)["total"].(float64); !ok || total != 1 {
		t.Fatalf("model list total = %#v, want 1", decodeData(t, listResponse)["total"])
	}

	detailResponse := performJSON(router, http.MethodGet, "/api/v1/models/"+modelID, nil, adminSession.cookies, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail model status = %d, want %d: %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}
	assertModelResponseHasNoSecrets(t, detailResponse.Body.String())

	updateResponse := performJSON(router, http.MethodPatch, "/api/v1/models/"+modelID, map[string]any{
		"displayName":            "GPT Image Capability Updated",
		"supportsN":              false,
		"maxOutputCount":         1,
		"supportedOutputFormats": []string{"webp"},
		"pricing": map[string]any{
			"currency": "USD",
			"unitPrices": map[string]float64{
				"image": 0.05,
			},
		},
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update model status = %d, want %d: %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}
	updateData := decodeData(t, updateResponse)
	if stringField(t, updateData, "displayName") != "GPT Image Capability Updated" {
		t.Fatalf("updated displayName = %q", stringField(t, updateData, "displayName"))
	}
	if supportsN, ok := updateData["supportsN"].(bool); !ok || supportsN {
		t.Fatalf("updated supportsN = %#v, want false", updateData["supportsN"])
	}

	disableResponse := performJSON(router, http.MethodPost, "/api/v1/models/"+modelID+"/disable", nil, adminSession.cookies, adminSession.csrfHeader())
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable model status = %d, want %d: %s", disableResponse.Code, http.StatusOK, disableResponse.Body.String())
	}
	if stringField(t, decodeData(t, disableResponse), "status") != modelcap.StatusDisabled {
		t.Fatalf("disable status response = %s", disableResponse.Body.String())
	}
	enableResponse := performJSON(router, http.MethodPost, "/api/v1/models/"+modelID+"/enable", nil, adminSession.cookies, adminSession.csrfHeader())
	if enableResponse.Code != http.StatusOK {
		t.Fatalf("enable model status = %d, want %d: %s", enableResponse.Code, http.StatusOK, enableResponse.Body.String())
	}
	if stringField(t, decodeData(t, enableResponse), "status") != modelcap.StatusEnabled {
		t.Fatalf("enable status response = %s", enableResponse.Body.String())
	}

	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/models/"+modelID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete model status = %d, want %d: %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	deletedDetail := performJSON(router, http.MethodGet, "/api/v1/models/"+modelID, nil, adminSession.cookies, nil)
	if deletedDetail.Code != http.StatusNotFound {
		t.Fatalf("deleted detail status = %d, want %d", deletedDetail.Code, http.StatusNotFound)
	}

	assertModelOperationLogs(t, db, []string{
		"model.create",
		"model.update",
		"model.disable",
		"model.enable",
		"model.delete",
	})
}

func TestModelRoutesEnforceRBACProviderTenantAndModelTenant(t *testing.T) {
	router, db, adminSession := newModelRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-rbac", "RBAC Provider")

	createResponse := performJSON(router, http.MethodPost, "/api/v1/models", validModelPayload(providerID), adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create model status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	modelID := stringField(t, decodeData(t, createResponse), "id")

	seedActiveUser(t, db, adminSession.tenantID, "seller-model", "seller-model@example.com", "Seller Model", "seller-model-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-model", "seller")
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-model@example.com", "seller-model-password-123")

	readResponse := performJSON(router, http.MethodGet, "/api/v1/models/"+modelID, nil, sellerSession.cookies, nil)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("seller read model status = %d, want %d: %s", readResponse.Code, http.StatusOK, readResponse.Body.String())
	}
	updateResponse := performJSON(router, http.MethodPatch, "/api/v1/models/"+modelID, map[string]string{"displayName": "Blocked"}, sellerSession.cookies, sellerSession.csrfHeader())
	if updateResponse.Code != http.StatusForbidden {
		t.Fatalf("seller update model status = %d, want %d", updateResponse.Code, http.StatusForbidden)
	}

	seedActiveUser(t, db, adminSession.tenantID, "model-manager", "model-manager@example.com", "Model Manager", "model-manager-password-123")
	assignModelManageOnlyRole(t, db, adminSession.tenantID, "model-manager")
	managerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "model-manager@example.com", "model-manager-password-123")
	managerReadResponse := performJSON(router, http.MethodGet, "/api/v1/models/"+modelID, nil, managerSession.cookies, nil)
	if managerReadResponse.Code != http.StatusOK {
		t.Fatalf("model:manage user read model status = %d, want %d: %s", managerReadResponse.Code, http.StatusOK, managerReadResponse.Body.String())
	}
	managerUpdateResponse := performJSON(router, http.MethodPatch, "/api/v1/models/"+modelID, map[string]string{"displayName": "Manager Updated"}, managerSession.cookies, managerSession.csrfHeader())
	if managerUpdateResponse.Code != http.StatusOK {
		t.Fatalf("model:manage user update model status = %d, want %d: %s", managerUpdateResponse.Code, http.StatusOK, managerUpdateResponse.Body.String())
	}

	seedActiveUser(t, db, adminSession.tenantID, "viewer-model", "viewer-model@example.com", "Viewer Model", "viewer-model-password-123")
	assignRole(t, db, adminSession.tenantID, "viewer-model", "viewer")
	viewerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "viewer-model@example.com", "viewer-model-password-123")
	viewerListResponse := performJSON(router, http.MethodGet, "/api/v1/models", nil, viewerSession.cookies, nil)
	if viewerListResponse.Code != http.StatusForbidden {
		t.Fatalf("viewer list model status = %d, want %d", viewerListResponse.Code, http.StatusForbidden)
	}

	seedOtherTenantModelData(t, db)
	crossTenantDetail := performJSON(router, http.MethodGet, "/api/v1/models/model-tenant-b", nil, adminSession.cookies, nil)
	if crossTenantDetail.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant model status = %d, want %d", crossTenantDetail.Code, http.StatusNotFound)
	}
	crossTenantProviderCreate := validModelPayload("provider-tenant-b")
	crossTenantProviderResponse := performJSON(router, http.MethodPost, "/api/v1/models", crossTenantProviderCreate, adminSession.cookies, adminSession.csrfHeader())
	if crossTenantProviderResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cross-tenant provider binding status = %d, want %d: %s", crossTenantProviderResponse.Code, http.StatusUnprocessableEntity, crossTenantProviderResponse.Body.String())
	}
}

func TestModelRoutesRejectUnavailableProviderForCreateUpdateAndEnable(t *testing.T) {
	router, db, adminSession := newModelRouteTestRouter(t)
	enabledProviderID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-available", "Available Provider")
	disabledProviderID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-disabled", "Disabled Provider")
	if err := db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id = ?", adminSession.tenantID, disabledProviderID).
		Update("status", provider.StatusDisabled).Error; err != nil {
		t.Fatalf("disable seeded provider: %v", err)
	}
	deletedProviderID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-deleted", "Deleted Provider")
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, deletedProviderID).Delete(&database.AIProvider{}).Error; err != nil {
		t.Fatalf("soft delete seeded provider: %v", err)
	}

	disabledCreate := performJSON(router, http.MethodPost, "/api/v1/models", validModelPayload(disabledProviderID), adminSession.cookies, adminSession.csrfHeader())
	if disabledCreate.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create with disabled provider status = %d, want %d: %s", disabledCreate.Code, http.StatusUnprocessableEntity, disabledCreate.Body.String())
	}
	assertResponseExcludes(t, disabledCreate.Body.String(), disabledProviderID, "Disabled Provider")

	deletedCreate := performJSON(router, http.MethodPost, "/api/v1/models", validModelPayload(deletedProviderID), adminSession.cookies, adminSession.csrfHeader())
	if deletedCreate.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create with deleted provider status = %d, want %d: %s", deletedCreate.Code, http.StatusUnprocessableEntity, deletedCreate.Body.String())
	}
	assertResponseExcludes(t, deletedCreate.Body.String(), deletedProviderID, "Deleted Provider")

	createResponse := performJSON(router, http.MethodPost, "/api/v1/models", validModelPayload(enabledProviderID), adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create available model status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	modelID := stringField(t, decodeData(t, createResponse), "id")

	migrateResponse := performJSON(router, http.MethodPatch, "/api/v1/models/"+modelID, map[string]any{
		"providerId": disabledProviderID,
	}, adminSession.cookies, adminSession.csrfHeader())
	if migrateResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("migrate to disabled provider status = %d, want %d: %s", migrateResponse.Code, http.StatusUnprocessableEntity, migrateResponse.Body.String())
	}
	assertResponseExcludes(t, migrateResponse.Body.String(), disabledProviderID, "Disabled Provider")

	if err := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND id = ?", adminSession.tenantID, modelID).
		Update("status", modelcap.StatusDisabled).Error; err != nil {
		t.Fatalf("disable model directly: %v", err)
	}
	if err := db.Model(&database.AIProvider{}).
		Where("tenant_id = ? AND id = ?", adminSession.tenantID, enabledProviderID).
		Update("status", provider.StatusDisabled).Error; err != nil {
		t.Fatalf("disable model provider directly: %v", err)
	}

	enableResponse := performJSON(router, http.MethodPost, "/api/v1/models/"+modelID+"/enable", nil, adminSession.cookies, adminSession.csrfHeader())
	if enableResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("enable model with disabled provider status = %d, want %d: %s", enableResponse.Code, http.StatusUnprocessableEntity, enableResponse.Body.String())
	}
	assertResponseExcludes(t, enableResponse.Body.String(), enabledProviderID, "Available Provider")

	patchEnableResponse := performJSON(router, http.MethodPatch, "/api/v1/models/"+modelID, map[string]any{
		"status": modelcap.StatusEnabled,
	}, adminSession.cookies, adminSession.csrfHeader())
	if patchEnableResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("patch enable model with disabled provider status = %d, want %d: %s", patchEnableResponse.Code, http.StatusUnprocessableEntity, patchEnableResponse.Body.String())
	}
	assertResponseExcludes(t, patchEnableResponse.Body.String(), enabledProviderID, "Available Provider")

	var record database.AIModel
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, modelID).First(&record).Error; err != nil {
		t.Fatalf("load model after rejected writes: %v", err)
	}
	if record.ProviderID != enabledProviderID || record.Status != modelcap.StatusDisabled {
		t.Fatalf("model changed after rejected provider writes: %#v", record)
	}
	assertNoModelOperationLog(t, db, modelID, "model.update")
	assertNoModelOperationLog(t, db, modelID, "model.enable")
}

func TestModelRoutesRejectDuplicateActiveModelNamePerProvider(t *testing.T) {
	router, db, adminSession := newModelRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-duplicate-model-name", "Duplicate Name Provider")

	firstPayload := validModelPayload(providerID)
	firstPayload["modelName"] = "duplicate-image-model"
	firstPayload["displayName"] = "Duplicate Image Model"
	first := performJSON(router, http.MethodPost, "/api/v1/models", firstPayload, adminSession.cookies, adminSession.csrfHeader())
	if first.Code != http.StatusCreated {
		t.Fatalf("create first model status = %d, want %d: %s", first.Code, http.StatusCreated, first.Body.String())
	}
	firstModelID := stringField(t, decodeData(t, first), "id")

	duplicateCreate := performJSON(router, http.MethodPost, "/api/v1/models", firstPayload, adminSession.cookies, adminSession.csrfHeader())
	if duplicateCreate.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate create status = %d, want %d: %s", duplicateCreate.Code, http.StatusUnprocessableEntity, duplicateCreate.Body.String())
	}
	assertResponseExcludes(t, duplicateCreate.Body.String(), providerID, "duplicate-image-model", "Duplicate Image Model")
	assertActiveModelNameCount(t, db, adminSession.tenantID, providerID, "duplicate-image-model", 1)

	secondPayload := validModelPayload(providerID)
	secondPayload["modelName"] = "other-image-model"
	secondPayload["displayName"] = "Other Image Model"
	second := performJSON(router, http.MethodPost, "/api/v1/models", secondPayload, adminSession.cookies, adminSession.csrfHeader())
	if second.Code != http.StatusCreated {
		t.Fatalf("create second model status = %d, want %d: %s", second.Code, http.StatusCreated, second.Body.String())
	}
	secondModelID := stringField(t, decodeData(t, second), "id")

	duplicateUpdate := performJSON(router, http.MethodPatch, "/api/v1/models/"+secondModelID, map[string]any{
		"modelName": "duplicate-image-model",
	}, adminSession.cookies, adminSession.csrfHeader())
	if duplicateUpdate.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate update status = %d, want %d: %s", duplicateUpdate.Code, http.StatusUnprocessableEntity, duplicateUpdate.Body.String())
	}
	assertActiveModelNameCount(t, db, adminSession.tenantID, providerID, "duplicate-image-model", 1)
	assertNoModelOperationLog(t, db, secondModelID, "model.update")

	deleteFirst := performJSON(router, http.MethodDelete, "/api/v1/models/"+firstModelID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteFirst.Code != http.StatusOK {
		t.Fatalf("delete first model status = %d, want %d: %s", deleteFirst.Code, http.StatusOK, deleteFirst.Body.String())
	}

	allowedAfterDelete := performJSON(router, http.MethodPatch, "/api/v1/models/"+secondModelID, map[string]any{
		"modelName": "duplicate-image-model",
	}, adminSession.cookies, adminSession.csrfHeader())
	if allowedAfterDelete.Code != http.StatusOK {
		t.Fatalf("rename after first soft-delete status = %d, want %d: %s", allowedAfterDelete.Code, http.StatusOK, allowedAfterDelete.Body.String())
	}
	assertActiveModelNameCount(t, db, adminSession.tenantID, providerID, "duplicate-image-model", 1)
}

func TestModelRoutesRejectInvalidCapabilityRequests(t *testing.T) {
	router, db, adminSession := newModelRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-validation", "Validation Provider")

	noCapability := validModelPayload(providerID)
	noCapability["supportsGenerate"] = false
	noCapability["supportsEdit"] = false
	response := performJSON(router, http.MethodPost, "/api/v1/models", noCapability, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing capability create status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}

	invalidN := validModelPayload(providerID)
	invalidN["supportsN"] = false
	invalidN["maxOutputCount"] = 2
	response = performJSON(router, http.MethodPost, "/api/v1/models", invalidN, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid supportsN create status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}

	badCurrency := validModelPayload(providerID)
	badCurrency["pricing"] = map[string]any{
		"currency":   "US",
		"unitPrices": map[string]float64{"image": 0.04},
	}
	response = performJSON(router, http.MethodPost, "/api/v1/models", badCurrency, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid pricing currency status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}

	tooManyPrices := validModelPayload(providerID)
	unitPrices := map[string]float64{}
	for i := 0; i < 21; i++ {
		unitPrices[fmt.Sprintf("unit-%02d", i)] = 0.01
	}
	tooManyPrices["pricing"] = map[string]any{
		"currency":   "USD",
		"unitPrices": unitPrices,
	}
	response = performJSON(router, http.MethodPost, "/api/v1/models", tooManyPrices, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unbounded pricing status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}

	tooManySizes := validModelPayload(providerID)
	sizes := make([]string, 65)
	for i := range sizes {
		sizes[i] = fmt.Sprintf("%dx%d", 64+i, 64+i)
	}
	tooManySizes["supportedSizes"] = sizes
	response = performJSON(router, http.MethodPost, "/api/v1/models", tooManySizes, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("too many sizes status = %d, want %d: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}

	createResponse := performJSON(router, http.MethodPost, "/api/v1/models", validModelPayload(providerID), adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create model status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	modelID := stringField(t, decodeData(t, createResponse), "id")
	updateInvalid := performJSON(router, http.MethodPatch, "/api/v1/models/"+modelID, map[string]any{
		"supportsN": false,
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateInvalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid supportsN update status = %d, want %d: %s", updateInvalid.Code, http.StatusUnprocessableEntity, updateInvalid.Body.String())
	}
}

func TestModelRoutesEnabledCapabilityList(t *testing.T) {
	router, db, adminSession := newModelRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-list", "List Provider")

	generatePayload := validModelPayload(providerID)
	generatePayload["modelName"] = "image-generate"
	generatePayload["displayName"] = "Image Generate"
	createResponse := performJSON(router, http.MethodPost, "/api/v1/models", generatePayload, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create generate model status = %d: %s", createResponse.Code, createResponse.Body.String())
	}

	disabledPayload := validModelPayload(providerID)
	disabledPayload["modelName"] = "image-disabled"
	disabledPayload["displayName"] = "Image Disabled"
	disabledPayload["status"] = modelcap.StatusDisabled
	createResponse = performJSON(router, http.MethodPost, "/api/v1/models", disabledPayload, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create disabled model status = %d: %s", createResponse.Code, createResponse.Body.String())
	}

	editPayload := validModelPayload(providerID)
	editPayload["modelName"] = "image-edit"
	editPayload["displayName"] = "Image Edit"
	editPayload["supportsGenerate"] = false
	editPayload["supportsEdit"] = true
	editPayload["supportsN"] = false
	editPayload["maxOutputCount"] = 1
	createResponse = performJSON(router, http.MethodPost, "/api/v1/models", editPayload, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create edit model status = %d: %s", createResponse.Code, createResponse.Body.String())
	}

	listResponse := performJSON(router, http.MethodGet, "/api/v1/models?enabled=true&capability=edit", nil, adminSession.cookies, nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("enabled edit model list status = %d, want %d: %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	data := decodeData(t, listResponse)
	if total, ok := data["total"].(float64); !ok || total != 2 {
		t.Fatalf("enabled edit model total = %#v, want 2", data["total"])
	}
	records, ok := data["records"].([]any)
	if !ok || len(records) != 2 {
		t.Fatalf("enabled edit records = %#v, want 2 records", data["records"])
	}
	body := listResponse.Body.String()
	if strings.Contains(body, "image-disabled") {
		t.Fatalf("enabled capability list included disabled model: %s", body)
	}
}

type modelRouteSession = projectRouteSession

func newModelRouteTestRouter(t *testing.T) (http.Handler, *gorm.DB, modelRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	router := NewRouter(RouterOptions{
		Config:   authRouteTestConfig("test"),
		Logger:   discardLogger(),
		Database: db,
	})

	initResponse := performJSON(router, http.MethodPost, "/api/v1/auth/init-admin", map[string]string{
		"tenantName":  "Studio Tenant",
		"email":       "admin-model@example.com",
		"displayName": "Admin Model",
		"password":    "initial-password-123",
	}, nil, nil)
	if initResponse.Code != http.StatusCreated {
		t.Fatalf("init admin status = %d, want %d: %s", initResponse.Code, http.StatusCreated, initResponse.Body.String())
	}
	data := decodeData(t, initResponse)
	authCookie := findCookie(t, initResponse, "studio_auth")
	csrfCookie := findCookie(t, initResponse, "studio_csrf")
	return router, db, modelRouteSession{
		tenantID: nestedString(t, data, "tenant", "id"),
		userID:   nestedString(t, data, "user", "id"),
		cookies:  []*http.Cookie{authCookie, csrfCookie},
		csrf:     csrfCookie.Value,
	}
}

func validModelPayload(providerID string) map[string]any {
	return map[string]any{
		"providerId":             providerID,
		"modelName":              "gpt-image-capability",
		"displayName":            "GPT Image Capability",
		"supportsGenerate":       true,
		"supportsEdit":           true,
		"supportsMultiReference": true,
		"supportsN":              true,
		"maxOutputCount":         4,
		"supportedSizes":         []string{"1024x1024", "1536x1024"},
		"supportedQualities":     []string{"standard", "hd"},
		"supportedOutputFormats": []string{"png", "jpeg"},
		"pricing": map[string]any{
			"currency": "USD",
			"unitPrices": map[string]float64{
				"image":        0.04,
				"inputToken1K": 0.01,
			},
		},
		"status": modelcap.StatusEnabled,
	}
}

func seedModelRouteProvider(t *testing.T, db *gorm.DB, tenantID string, userID string, providerID string, name string) string {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&database.AIProvider{
		ID:               providerID,
		TenantID:         tenantID,
		Type:             provider.TypeOpenAICompatible,
		Name:             name,
		BaseURL:          "https://api.openai.com/v1",
		EncryptedAPIKey:  "v1:test-key-v1:ciphertext",
		APIKeyHint:       "****0000",
		Status:           provider.StatusEnabled,
		TimeoutSeconds:   10,
		ConcurrencyLimit: 0,
		CreatedBy:        userID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("seed model route provider: %v", err)
	}
	return providerID
}

func seedOtherTenantModelData(t *testing.T, db *gorm.DB) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&database.Tenant{
		ID:        "tenant-b",
		Name:      "Tenant B",
		Status:    "ACTIVE",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B: %v", err)
	}
	if err := db.Create(&database.User{
		ID:           "user-tenant-b",
		TenantID:     "tenant-b",
		Email:        "tenant-b-model@example.com",
		DisplayName:  "Tenant B User",
		PasswordHash: "hash",
		Status:       "ACTIVE",
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B user: %v", err)
	}
	if err := db.Create(&database.AIProvider{
		ID:               "provider-tenant-b",
		TenantID:         "tenant-b",
		Type:             provider.TypeOpenAICompatible,
		Name:             "Tenant B Provider",
		BaseURL:          "https://api.openai.com/v1",
		EncryptedAPIKey:  "v1:test-key-v1:ciphertext",
		APIKeyHint:       "****0000",
		Status:           provider.StatusEnabled,
		TimeoutSeconds:   10,
		ConcurrencyLimit: 0,
		CreatedBy:        "user-tenant-b",
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B provider: %v", err)
	}
	if err := db.Create(&database.AIModel{
		ID:                         "model-tenant-b",
		TenantID:                   "tenant-b",
		ProviderID:                 "provider-tenant-b",
		ModelName:                  "tenant-b-model",
		DisplayName:                "Tenant B Model",
		SupportsGenerate:           true,
		SupportsEdit:               true,
		SupportsMultiReference:     true,
		SupportsN:                  true,
		MaxOutputCount:             4,
		SupportedSizesJSON:         `["1024x1024"]`,
		SupportedQualitiesJSON:     `["standard"]`,
		SupportedOutputFormatsJSON: `["png"]`,
		PricingJSON:                `{"currency":"USD","unitPrices":{"image":0.04}}`,
		Status:                     modelcap.StatusEnabled,
		CreatedBy:                  "user-tenant-b",
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B model: %v", err)
	}
}

func assignModelManageOnlyRole(t *testing.T, db *gorm.DB, tenantID string, userID string) {
	t.Helper()

	var permission database.Permission
	if err := db.Where("code = ?", modelcap.PermissionManage).First(&permission).Error; err != nil {
		t.Fatalf("find model manage permission: %v", err)
	}
	now := time.Now().UTC()
	role := database.Role{
		ID:          "role-model-manager",
		TenantID:    tenantID,
		Code:        "model-manager",
		Name:        "Model Manager",
		Description: "Model manage permission only",
		Status:      "ACTIVE",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("seed model manager role: %v", err)
	}
	if err := db.Create(&database.RolePermission{
		ID:           "role-permission-model-manager",
		TenantID:     tenantID,
		RoleID:       role.ID,
		PermissionID: permission.ID,
		CreatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed model manager permission: %v", err)
	}
	if err := db.Create(&database.UserRole{
		ID:        "user-role-" + userID + "-model-manager",
		TenantID:  tenantID,
		UserID:    userID,
		RoleID:    role.ID,
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("assign model manager role: %v", err)
	}
}

func assertModelResponseHasNoSecrets(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, forbidden := range []string{"api_key", "apikey", "encrypted", "ciphertext", "authorization", "cookie", "password", "jwt", "base64"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("model response contains %q: %s", forbidden, body)
		}
	}
}

func assertModelOperationLogs(t *testing.T, db *gorm.DB, expectedActions []string) {
	t.Helper()

	var logs []database.OperationLog
	if err := db.Order("created_at ASC").Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}

	seen := map[string]bool{}
	for _, log := range logs {
		seen[log.Action] = true
		metadata := strings.ToLower(log.MetadataJSON)
		for _, forbidden := range []string{"api_key", "apikey", "authorization", "cookie", "password", "jwt", "bearer", "ciphertext", "base64"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %#v", forbidden, log)
			}
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(log.MetadataJSON), &decoded); err != nil {
			t.Fatalf("operation log metadata is not JSON: %v", err)
		}
	}
	for _, action := range expectedActions {
		if !seen[action] {
			t.Fatalf("missing operation log action %s; logs = %#v", action, logs)
		}
	}
}

func assertNoModelOperationLog(t *testing.T, db *gorm.DB, modelID string, action string) {
	t.Helper()

	var count int64
	if err := db.Model(&database.OperationLog{}).
		Where("resource_type = ? AND resource_id = ? AND action = ?", "model", modelID, action).
		Count(&count).Error; err != nil {
		t.Fatalf("count %s operation logs: %v", action, err)
	}
	if count != 0 {
		t.Fatalf("%s operation log count = %d, want 0", action, count)
	}
}

func assertActiveModelNameCount(t *testing.T, db *gorm.DB, tenantID string, providerID string, modelName string, expected int64) {
	t.Helper()

	var count int64
	if err := db.Model(&database.AIModel{}).
		Where("tenant_id = ? AND provider_id = ? AND model_name = ? AND deleted_at IS NULL", tenantID, providerID, modelName).
		Count(&count).Error; err != nil {
		t.Fatalf("count active model name: %v", err)
	}
	if count != expected {
		t.Fatalf("active model name count = %d, want %d", count, expected)
	}
}
