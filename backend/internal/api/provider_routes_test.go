package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	modelcap "github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"gorm.io/gorm"
)

func TestProviderRoutesCRUDEnableDisableDeleteTestAndAudit(t *testing.T) {
	router, db, fakeProbe, adminSession := newProviderRouteTestRouter(t)

	createResponse := performJSON(router, http.MethodPost, "/api/v1/providers", map[string]any{
		"type":             provider.TypeOpenAICompatible,
		"name":             "Secure Relay",
		"baseUrl":          "https://api.openai.com/v1",
		"apiKey":           "sk-live-secret-1234",
		"status":           provider.StatusEnabled,
		"timeoutSeconds":   10,
		"concurrencyLimit": 2,
	}, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create provider status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	assertProviderResponseHasNoSecret(t, createResponse.Body.String())
	createData := decodeData(t, createResponse)
	providerID := stringField(t, createData, "id")
	if stringField(t, createData, "apiKeyHint") != "****1234" {
		t.Fatalf("apiKeyHint = %q, want ****1234", stringField(t, createData, "apiKeyHint"))
	}
	if createData["apiKeyUpdatedAt"] == nil {
		t.Fatal("create response missing apiKeyUpdatedAt")
	}

	var record database.AIProvider
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, providerID).First(&record).Error; err != nil {
		t.Fatalf("load provider: %v", err)
	}
	if record.EncryptedAPIKey == "" || strings.Contains(record.EncryptedAPIKey, "sk-live-secret-1234") {
		t.Fatalf("provider API key was not encrypted safely: %#v", record)
	}
	if record.APIKeyHint != "****1234" {
		t.Fatalf("stored api key hint = %q", record.APIKeyHint)
	}

	listResponse := performJSON(router, http.MethodGet, "/api/v1/providers?type=OPENAI_COMPATIBLE&status=ENABLED", nil, adminSession.cookies, nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list providers status = %d, want %d: %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	assertProviderResponseHasNoSecret(t, listResponse.Body.String())
	if total, ok := decodeData(t, listResponse)["total"].(float64); !ok || total != 1 {
		t.Fatalf("provider list total = %#v, want 1", decodeData(t, listResponse)["total"])
	}

	detailResponse := performJSON(router, http.MethodGet, "/api/v1/providers/"+providerID, nil, adminSession.cookies, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail provider status = %d, want %d: %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}
	assertProviderResponseHasNoSecret(t, detailResponse.Body.String())

	updateResponse := performJSON(router, http.MethodPatch, "/api/v1/providers/"+providerID, map[string]any{
		"name":   "Secure Relay Rotated",
		"apiKey": "sk-rotated-secret-5678",
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update provider status = %d, want %d: %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}
	assertProviderResponseHasNoSecret(t, updateResponse.Body.String())
	if stringField(t, decodeData(t, updateResponse), "apiKeyHint") != "****5678" {
		t.Fatalf("rotated apiKeyHint response = %s", updateResponse.Body.String())
	}
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, providerID).First(&record).Error; err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	if strings.Contains(record.EncryptedAPIKey, "sk-rotated-secret-5678") || strings.Contains(record.EncryptedAPIKey, "sk-live-secret-1234") {
		t.Fatalf("rotated encrypted key contains plaintext: %#v", record)
	}

	disableResponse := performJSON(router, http.MethodPost, "/api/v1/providers/"+providerID+"/disable", nil, adminSession.cookies, adminSession.csrfHeader())
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable provider status = %d, want %d: %s", disableResponse.Code, http.StatusOK, disableResponse.Body.String())
	}
	if stringField(t, decodeData(t, disableResponse), "status") != provider.StatusDisabled {
		t.Fatalf("disable status response = %s", disableResponse.Body.String())
	}
	enableResponse := performJSON(router, http.MethodPost, "/api/v1/providers/"+providerID+"/enable", nil, adminSession.cookies, adminSession.csrfHeader())
	if enableResponse.Code != http.StatusOK {
		t.Fatalf("enable provider status = %d, want %d: %s", enableResponse.Code, http.StatusOK, enableResponse.Body.String())
	}

	testResponse := performJSON(router, http.MethodPost, "/api/v1/providers/"+providerID+"/test", nil, adminSession.cookies, adminSession.csrfHeader())
	if testResponse.Code != http.StatusOK {
		t.Fatalf("test provider status = %d, want %d: %s", testResponse.Code, http.StatusOK, testResponse.Body.String())
	}
	assertProviderResponseHasNoSecret(t, testResponse.Body.String())
	testData := decodeData(t, testResponse)
	if stringField(t, testData, "status") != provider.TestStatusSuccess {
		t.Fatalf("test status = %q", stringField(t, testData, "status"))
	}
	if fakeProbe.lastAPIKey != "sk-rotated-secret-5678" {
		t.Fatalf("probe did not receive decrypted rotated key")
	}
	var assetCount int64
	if err := db.Model(&database.ImageAsset{}).Count(&assetCount).Error; err != nil {
		t.Fatalf("count assets: %v", err)
	}
	if assetCount != 0 {
		t.Fatalf("provider test must not create assets, count = %d", assetCount)
	}

	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/providers/"+providerID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete provider status = %d, want %d: %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	deletedDetail := performJSON(router, http.MethodGet, "/api/v1/providers/"+providerID, nil, adminSession.cookies, nil)
	if deletedDetail.Code != http.StatusNotFound {
		t.Fatalf("deleted detail status = %d, want %d", deletedDetail.Code, http.StatusNotFound)
	}
	assertProviderCredentialCryptoErased(t, db, adminSession.tenantID, providerID)

	assertProviderOperationLogs(t, db, []string{
		"provider.create",
		"provider.update",
		"provider.disable",
		"provider.enable",
		"provider.test",
		"provider.delete",
	})
}

func TestProviderRoutesEnforceRBACAndTenantScope(t *testing.T) {
	router, db, _, adminSession := newProviderRouteTestRouter(t)

	createResponse := performJSON(router, http.MethodPost, "/api/v1/providers", map[string]any{
		"type":    provider.TypeOpenAI,
		"name":    "Official OpenAI",
		"apiKey":  "sk-admin-secret-0001",
		"baseUrl": "https://api.openai.com/v1",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create provider status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	providerID := stringField(t, decodeData(t, createResponse), "id")

	seedActiveUser(t, db, adminSession.tenantID, "seller-provider", "seller-provider@example.com", "Seller Provider", "seller-provider-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-provider", "seller")
	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller-provider@example.com", "seller-provider-password-123")

	readResponse := performJSON(router, http.MethodGet, "/api/v1/providers/"+providerID, nil, sellerSession.cookies, nil)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("seller read provider status = %d, want %d: %s", readResponse.Code, http.StatusOK, readResponse.Body.String())
	}
	updateResponse := performJSON(router, http.MethodPatch, "/api/v1/providers/"+providerID, map[string]string{"name": "Blocked"}, sellerSession.cookies, sellerSession.csrfHeader())
	if updateResponse.Code != http.StatusForbidden {
		t.Fatalf("seller update provider status = %d, want %d", updateResponse.Code, http.StatusForbidden)
	}
	testResponse := performJSON(router, http.MethodPost, "/api/v1/providers/"+providerID+"/test", nil, sellerSession.cookies, sellerSession.csrfHeader())
	if testResponse.Code != http.StatusForbidden {
		t.Fatalf("seller test provider status = %d, want %d", testResponse.Code, http.StatusForbidden)
	}
	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/providers/"+providerID, nil, sellerSession.cookies, sellerSession.csrfHeader())
	if deleteResponse.Code != http.StatusForbidden {
		t.Fatalf("seller delete provider status = %d, want %d", deleteResponse.Code, http.StatusForbidden)
	}

	seedActiveUser(t, db, adminSession.tenantID, "viewer-provider", "viewer-provider@example.com", "Viewer Provider", "viewer-provider-password-123")
	assignRole(t, db, adminSession.tenantID, "viewer-provider", "viewer")
	viewerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "viewer-provider@example.com", "viewer-provider-password-123")
	viewerListResponse := performJSON(router, http.MethodGet, "/api/v1/providers", nil, viewerSession.cookies, nil)
	if viewerListResponse.Code != http.StatusForbidden {
		t.Fatalf("viewer list provider status = %d, want %d", viewerListResponse.Code, http.StatusForbidden)
	}

	seedOtherTenantProvider(t, db)
	crossTenantResponse := performJSON(router, http.MethodGet, "/api/v1/providers/provider-tenant-b", nil, adminSession.cookies, nil)
	if crossTenantResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant provider status = %d, want %d", crossTenantResponse.Code, http.StatusNotFound)
	}
	crossTenantDeleteResponse := performJSON(router, http.MethodDelete, "/api/v1/providers/provider-tenant-b", nil, adminSession.cookies, adminSession.csrfHeader())
	if crossTenantDeleteResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant provider delete status = %d, want %d", crossTenantDeleteResponse.Code, http.StatusNotFound)
	}
	unknownDeleteResponse := performJSON(router, http.MethodDelete, "/api/v1/providers/provider-missing", nil, adminSession.cookies, adminSession.csrfHeader())
	if unknownDeleteResponse.Code != http.StatusNotFound {
		t.Fatalf("unknown provider delete status = %d, want %d", unknownDeleteResponse.Code, http.StatusNotFound)
	}
}

func TestProviderDeleteRejectsLinkedEnabledModelAndLeavesRowsUnchanged(t *testing.T) {
	router, db, _, adminSession := newProviderRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-linked-enabled", "Linked Enabled Provider")
	modelID := seedProviderRouteModel(t, db, adminSession.tenantID, adminSession.userID, providerID, "model-linked-enabled", modelcap.StatusEnabled, "linked-enabled-model", "Linked Enabled Model")

	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/providers/"+providerID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusConflict {
		t.Fatalf("delete linked enabled provider status = %d, want %d: %s", deleteResponse.Code, http.StatusConflict, deleteResponse.Body.String())
	}
	if code := errorCode(t, deleteResponse); code != "PROVIDER_HAS_LINKED_MODELS" {
		t.Fatalf("delete linked enabled error code = %q, want PROVIDER_HAS_LINKED_MODELS: %s", code, deleteResponse.Body.String())
	}
	assertResponseExcludes(t, deleteResponse.Body.String(), "linked-enabled-model", "Linked Enabled Model", "tenant-b")
	assertProviderAndModelUnchanged(t, db, adminSession.tenantID, providerID, modelID, modelcap.StatusEnabled)
	assertNoProviderDeleteOperationLog(t, db, providerID)
}

func TestProviderDeleteRejectsLinkedDisabledModelAndLeavesRowsUnchanged(t *testing.T) {
	router, db, _, adminSession := newProviderRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-linked-disabled", "Linked Disabled Provider")
	modelID := seedProviderRouteModel(t, db, adminSession.tenantID, adminSession.userID, providerID, "model-linked-disabled", modelcap.StatusDisabled, "linked-disabled-model", "Linked Disabled Model")

	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/providers/"+providerID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusConflict {
		t.Fatalf("delete linked disabled provider status = %d, want %d: %s", deleteResponse.Code, http.StatusConflict, deleteResponse.Body.String())
	}
	if code := errorCode(t, deleteResponse); code != "PROVIDER_HAS_LINKED_MODELS" {
		t.Fatalf("delete linked disabled error code = %q, want PROVIDER_HAS_LINKED_MODELS: %s", code, deleteResponse.Body.String())
	}
	assertResponseExcludes(t, deleteResponse.Body.String(), "linked-disabled-model", "Linked Disabled Model", "tenant-b")
	assertProviderAndModelUnchanged(t, db, adminSession.tenantID, providerID, modelID, modelcap.StatusDisabled)
	assertNoProviderDeleteOperationLog(t, db, providerID)
}

func TestProviderDeleteSucceedsAfterLinkedModelIsSoftDeleted(t *testing.T) {
	router, db, _, adminSession := newProviderRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-after-model-delete", "After Model Delete Provider")
	modelID := seedProviderRouteModel(t, db, adminSession.tenantID, adminSession.userID, providerID, "model-before-provider-delete", modelcap.StatusEnabled, "model-before-provider-delete", "Model Before Provider Delete")

	modelDelete := performJSON(router, http.MethodDelete, "/api/v1/models/"+modelID, nil, adminSession.cookies, adminSession.csrfHeader())
	if modelDelete.Code != http.StatusOK {
		t.Fatalf("delete linked model status = %d, want %d: %s", modelDelete.Code, http.StatusOK, modelDelete.Body.String())
	}
	var deletedModel database.AIModel
	if err := db.Unscoped().Where("tenant_id = ? AND id = ?", adminSession.tenantID, modelID).First(&deletedModel).Error; err != nil {
		t.Fatalf("load soft-deleted model: %v", err)
	}
	if !deletedModel.DeletedAt.Valid {
		t.Fatalf("linked model was not soft-deleted: %#v", deletedModel)
	}

	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/providers/"+providerID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete provider after model soft delete status = %d, want %d: %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	deletedDetail := performJSON(router, http.MethodGet, "/api/v1/providers/"+providerID, nil, adminSession.cookies, nil)
	if deletedDetail.Code != http.StatusNotFound {
		t.Fatalf("deleted provider detail status = %d, want %d", deletedDetail.Code, http.StatusNotFound)
	}
	assertProviderDeleteOperationLogExcludes(t, db, providerID, "model-before-provider-delete", "Model Before Provider Delete")
	assertProviderCredentialCryptoErased(t, db, adminSession.tenantID, providerID)
}

func TestProviderDeleteIgnoresCrossTenantLinkedModels(t *testing.T) {
	router, db, _, adminSession := newProviderRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-cross-tenant-linked", "Cross Tenant Provider")
	seedCrossTenantModelReferencingProvider(t, db, providerID)

	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/providers/"+providerID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete provider with cross-tenant linked model status = %d, want %d: %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	assertResponseExcludes(t, deleteResponse.Body.String(), "tenant-b", "cross-tenant-linked-model", "Cross Tenant Linked Model")

	var crossTenantModel database.AIModel
	if err := db.Where("tenant_id = ? AND id = ?", "tenant-b", "model-cross-tenant-provider-id").First(&crossTenantModel).Error; err != nil {
		t.Fatalf("load cross-tenant linked model: %v", err)
	}
	if crossTenantModel.DeletedAt.Valid || crossTenantModel.Status != modelcap.StatusEnabled {
		t.Fatalf("cross-tenant model was mutated: %#v", crossTenantModel)
	}
}

func TestProviderDisableRejectsLinkedEnabledModelsAndLeavesRowsUnchanged(t *testing.T) {
	router, db, _, adminSession := newProviderRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-disable-linked", "Disable Linked Provider")
	modelID := seedProviderRouteModel(t, db, adminSession.tenantID, adminSession.userID, providerID, "model-disable-linked", modelcap.StatusEnabled, "disable-linked-model", "Disable Linked Model")

	disableResponse := performJSON(router, http.MethodPost, "/api/v1/providers/"+providerID+"/disable", nil, adminSession.cookies, adminSession.csrfHeader())
	if disableResponse.Code != http.StatusConflict {
		t.Fatalf("disable linked provider status = %d, want %d: %s", disableResponse.Code, http.StatusConflict, disableResponse.Body.String())
	}
	if code := errorCode(t, disableResponse); code != "PROVIDER_HAS_ENABLED_MODELS" {
		t.Fatalf("disable linked provider error code = %q, want PROVIDER_HAS_ENABLED_MODELS: %s", code, disableResponse.Body.String())
	}
	assertResponseExcludes(t, disableResponse.Body.String(), "disable-linked-model", "Disable Linked Model", providerID)

	patchDisableResponse := performJSON(router, http.MethodPatch, "/api/v1/providers/"+providerID, map[string]any{
		"status": provider.StatusDisabled,
		"name":   "Should Not Persist",
	}, adminSession.cookies, adminSession.csrfHeader())
	if patchDisableResponse.Code != http.StatusConflict {
		t.Fatalf("patch disable linked provider status = %d, want %d: %s", patchDisableResponse.Code, http.StatusConflict, patchDisableResponse.Body.String())
	}
	if code := errorCode(t, patchDisableResponse); code != "PROVIDER_HAS_ENABLED_MODELS" {
		t.Fatalf("patch disable linked provider error code = %q, want PROVIDER_HAS_ENABLED_MODELS: %s", code, patchDisableResponse.Body.String())
	}

	var providerRecord database.AIProvider
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, providerID).First(&providerRecord).Error; err != nil {
		t.Fatalf("load provider after rejected disable: %v", err)
	}
	if providerRecord.Status != provider.StatusEnabled || providerRecord.Name != "Disable Linked Provider" {
		t.Fatalf("provider changed after rejected disable: %#v", providerRecord)
	}

	var linkedModel database.AIModel
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, modelID).First(&linkedModel).Error; err != nil {
		t.Fatalf("load linked model after rejected disable: %v", err)
	}
	if linkedModel.DeletedAt.Valid || linkedModel.Status != modelcap.StatusEnabled {
		t.Fatalf("linked model changed after rejected disable: %#v", linkedModel)
	}
	assertNoProviderOperationLog(t, db, providerID, "provider.disable")
	assertNoProviderOperationLog(t, db, providerID, "provider.update")
}

func TestProviderDisableAllowsOnlyDisabledLinkedModels(t *testing.T) {
	router, db, _, adminSession := newProviderRouteTestRouter(t)
	providerID := seedModelRouteProvider(t, db, adminSession.tenantID, adminSession.userID, "provider-disable-disabled-model", "Disable Disabled Model Provider")
	modelID := seedProviderRouteModel(t, db, adminSession.tenantID, adminSession.userID, providerID, "model-disable-disabled-model", modelcap.StatusDisabled, "disable-disabled-model", "Disable Disabled Model")

	disableResponse := performJSON(router, http.MethodPost, "/api/v1/providers/"+providerID+"/disable", nil, adminSession.cookies, adminSession.csrfHeader())
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable provider with disabled linked model status = %d, want %d: %s", disableResponse.Code, http.StatusOK, disableResponse.Body.String())
	}
	if stringField(t, decodeData(t, disableResponse), "status") != provider.StatusDisabled {
		t.Fatalf("disable provider status response = %s", disableResponse.Body.String())
	}

	var linkedModel database.AIModel
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, modelID).First(&linkedModel).Error; err != nil {
		t.Fatalf("load linked disabled model after provider disable: %v", err)
	}
	if linkedModel.DeletedAt.Valid || linkedModel.Status != modelcap.StatusDisabled {
		t.Fatalf("linked disabled model was mutated by provider disable: %#v", linkedModel)
	}
}

func TestProviderRoutesRejectSSRFBaseURLs(t *testing.T) {
	router, _, _, adminSession := newProviderRouteTestRouter(t)

	createResponse := performJSON(router, http.MethodPost, "/api/v1/providers", map[string]any{
		"type":    provider.TypeOpenAICompatible,
		"name":    "Blocked Local",
		"baseUrl": "https://127.0.0.1/v1",
		"apiKey":  "sk-blocked-secret-0001",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create blocked provider status = %d, want %d: %s", createResponse.Code, http.StatusUnprocessableEntity, createResponse.Body.String())
	}

	createGoodResponse := performJSON(router, http.MethodPost, "/api/v1/providers", map[string]any{
		"type":    provider.TypeOpenAICompatible,
		"name":    "Allowed Public",
		"baseUrl": "https://api.openai.com/v1",
		"apiKey":  "sk-allowed-secret-0001",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createGoodResponse.Code != http.StatusCreated {
		t.Fatalf("create allowed provider status = %d, want %d: %s", createGoodResponse.Code, http.StatusCreated, createGoodResponse.Body.String())
	}
	providerID := stringField(t, decodeData(t, createGoodResponse), "id")

	updateBlockedResponse := performJSON(router, http.MethodPatch, "/api/v1/providers/"+providerID, map[string]string{
		"baseUrl": "https://mysql/v1",
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateBlockedResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update blocked provider status = %d, want %d: %s", updateBlockedResponse.Code, http.StatusUnprocessableEntity, updateBlockedResponse.Body.String())
	}
}

func TestProviderRoutesRevalidateBaseURLOnSaveUpdateAndTest(t *testing.T) {
	resolver := &mutableProviderRouteResolver{}
	resolver.set("api.openai.com", "93.184.216.34")
	resolver.set("rebind.example.com", "93.184.216.34")
	resolver.set("private.example.com", "10.0.0.5")
	router, db, fakeProbe, adminSession := newProviderRouteTestRouterWithResolver(t, resolver)

	createBlocked := performJSON(router, http.MethodPost, "/api/v1/providers", map[string]any{
		"type":    provider.TypeOpenAICompatible,
		"name":    "Private DNS",
		"baseUrl": "https://private.example.com/v1",
		"apiKey":  "fake-secret-for-ssrf-test",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createBlocked.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create private DNS status = %d, want %d: %s", createBlocked.Code, http.StatusUnprocessableEntity, createBlocked.Body.String())
	}

	createGood := performJSON(router, http.MethodPost, "/api/v1/providers", map[string]any{
		"type":    provider.TypeOpenAICompatible,
		"name":    "Rebinding Relay",
		"baseUrl": "https://rebind.example.com/v1",
		"apiKey":  "fake-secret-for-ssrf-test",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createGood.Code != http.StatusCreated {
		t.Fatalf("create public provider status = %d, want %d: %s", createGood.Code, http.StatusCreated, createGood.Body.String())
	}
	providerID := stringField(t, decodeData(t, createGood), "id")

	updateBlocked := performJSON(router, http.MethodPatch, "/api/v1/providers/"+providerID, map[string]string{
		"baseUrl": "https://private.example.com/v1",
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateBlocked.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update private DNS status = %d, want %d: %s", updateBlocked.Code, http.StatusUnprocessableEntity, updateBlocked.Body.String())
	}

	resolver.set("rebind.example.com", "127.0.0.1")
	testBlocked := performJSON(router, http.MethodPost, "/api/v1/providers/"+providerID+"/test", nil, adminSession.cookies, adminSession.csrfHeader())
	if testBlocked.Code != http.StatusUnprocessableEntity {
		t.Fatalf("test rebinding provider status = %d, want %d: %s", testBlocked.Code, http.StatusUnprocessableEntity, testBlocked.Body.String())
	}
	if fakeProbe.lastAPIKey != "" {
		t.Fatalf("probe was called after runtime URL revalidation failed")
	}
	assertResponseExcludes(t, testBlocked.Body.String(), "fake-secret-for-ssrf-test", "rebind.example.com", "127.0.0.1")

	var record database.AIProvider
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, providerID).First(&record).Error; err != nil {
		t.Fatalf("load revalidated provider: %v", err)
	}
	if record.LastTestStatus != provider.TestStatusFailure || record.LastTestError == "" {
		t.Fatalf("blocked test did not persist sanitized failure status: %#v", record)
	}
	assertProviderOperationLogs(t, db, []string{"provider.create", "provider.test"})
}

func TestProviderTestResponseAndAuditAreRedactedOnProbeFailure(t *testing.T) {
	router, db, fakeProbe, adminSession := newProviderRouteTestRouter(t)
	fakeProbe.result = provider.ProbeResult{
		Status:    provider.TestStatusFailure,
		CheckedAt: time.Now().UTC(),
		Message:   "raw provider said invalid api key sk-raw-secret",
	}

	createResponse := performJSON(router, http.MethodPost, "/api/v1/providers", map[string]any{
		"type":    provider.TypeOpenAICompatible,
		"name":    "Failing Relay",
		"baseUrl": "https://api.openai.com/v1",
		"apiKey":  "sk-failing-secret-0001",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create provider status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	providerID := stringField(t, decodeData(t, createResponse), "id")

	testResponse := performJSON(router, http.MethodPost, "/api/v1/providers/"+providerID+"/test", nil, adminSession.cookies, adminSession.csrfHeader())
	if testResponse.Code != http.StatusOK {
		t.Fatalf("test provider status = %d, want %d: %s", testResponse.Code, http.StatusOK, testResponse.Body.String())
	}
	body := strings.ToLower(testResponse.Body.String())
	for _, forbidden := range []string{"sk-failing-secret", "sk-raw-secret", "api key", "authorization", "cookie"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("provider test response contains %q: %s", forbidden, testResponse.Body.String())
		}
	}

	assertProviderOperationLogs(t, db, []string{"provider.test"})
}

type providerRouteSession = projectRouteSession

type fakeProviderProber struct {
	result     provider.ProbeResult
	lastAPIKey string
}

func (p *fakeProviderProber) Test(_ context.Context, config provider.ProbeConfig) (provider.ProbeResult, error) {
	p.lastAPIKey = config.APIKey
	if p.result.Status != "" {
		return p.result, nil
	}
	status := http.StatusOK
	return provider.ProbeResult{
		Status:     provider.TestStatusSuccess,
		DurationMs: 12,
		CheckedAt:  time.Now().UTC(),
		HTTPStatus: &status,
		RequestID:  "req-provider-test",
		Message:    "Provider test succeeded.",
	}, nil
}

type providerRouteResolver map[string][]net.IPAddr

func (r providerRouteResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if addresses, ok := r[host]; ok {
		return addresses, nil
	}
	return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
}

type mutableProviderRouteResolver struct {
	addresses map[string][]net.IPAddr
}

func (r *mutableProviderRouteResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if addresses, ok := r.addresses[host]; ok {
		return addresses, nil
	}
	return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
}

func (r *mutableProviderRouteResolver) set(host string, ips ...string) {
	if r.addresses == nil {
		r.addresses = map[string][]net.IPAddr{}
	}
	addresses := make([]net.IPAddr, 0, len(ips))
	for _, raw := range ips {
		addresses = append(addresses, net.IPAddr{IP: net.ParseIP(raw)})
	}
	r.addresses[host] = addresses
}

func newProviderRouteTestRouter(t *testing.T) (http.Handler, *gorm.DB, *fakeProviderProber, providerRouteSession) {
	t.Helper()

	return newProviderRouteTestRouterWithResolver(t, providerRouteResolver{
		"api.openai.com": {{IP: net.ParseIP("93.184.216.34")}},
	})
}

func newProviderRouteTestRouterWithResolver(t *testing.T, resolver provider.Resolver) (http.Handler, *gorm.DB, *fakeProviderProber, providerRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	fakeProbe := &fakeProviderProber{}
	router := NewRouter(RouterOptions{
		Config:   authRouteTestConfig("test"),
		Logger:   discardLogger(),
		Database: db,
		ProviderOpts: []provider.Option{
			provider.WithURLValidator(provider.NewURLValidator(resolver)),
			provider.WithProber(fakeProbe),
		},
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
	return router, db, fakeProbe, providerRouteSession{
		tenantID: nestedString(t, data, "tenant", "id"),
		userID:   nestedString(t, data, "user", "id"),
		cookies:  []*http.Cookie{authCookie, csrfCookie},
		csrf:     csrfCookie.Value,
	}
}

func seedOtherTenantProvider(t *testing.T, db *gorm.DB) {
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
		Email:        "tenant-b-provider@example.com",
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
}

func seedProviderRouteModel(t *testing.T, db *gorm.DB, tenantID string, userID string, providerID string, modelID string, status string, modelName string, displayName string) string {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&database.AIModel{
		ID:                         modelID,
		TenantID:                   tenantID,
		ProviderID:                 providerID,
		ModelName:                  modelName,
		DisplayName:                displayName,
		SupportsGenerate:           true,
		SupportsEdit:               true,
		SupportsMultiReference:     true,
		SupportsN:                  true,
		MaxOutputCount:             4,
		SupportedSizesJSON:         `["1024x1024"]`,
		SupportedQualitiesJSON:     `["standard"]`,
		SupportedOutputFormatsJSON: `["png"]`,
		PricingJSON:                `{"currency":"USD","unitPrices":{"image":0.04}}`,
		Status:                     status,
		CreatedBy:                  userID,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}).Error; err != nil {
		t.Fatalf("seed provider route model: %v", err)
	}
	return modelID
}

func seedCrossTenantModelReferencingProvider(t *testing.T, db *gorm.DB, providerID string) {
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
		ID:           "user-tenant-b-linked-provider",
		TenantID:     "tenant-b",
		Email:        "tenant-b-linked-provider@example.com",
		DisplayName:  "Tenant B Linked Provider User",
		PasswordHash: "hash",
		Status:       "ACTIVE",
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B linked provider user: %v", err)
	}
	if err := db.Create(&database.AIModel{
		ID:                         "model-cross-tenant-provider-id",
		TenantID:                   "tenant-b",
		ProviderID:                 providerID,
		ModelName:                  "cross-tenant-linked-model",
		DisplayName:                "Cross Tenant Linked Model",
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
		CreatedBy:                  "user-tenant-b-linked-provider",
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}).Error; err != nil {
		t.Fatalf("seed cross-tenant linked model: %v", err)
	}
}

func assertProviderAndModelUnchanged(t *testing.T, db *gorm.DB, tenantID string, providerID string, modelID string, expectedModelStatus string) {
	t.Helper()

	var providerRecord database.AIProvider
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, providerID).First(&providerRecord).Error; err != nil {
		t.Fatalf("load provider after conflict: %v", err)
	}
	if providerRecord.DeletedAt.Valid {
		t.Fatalf("provider was deleted after conflict: %#v", providerRecord)
	}

	var modelRecord database.AIModel
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, modelID).First(&modelRecord).Error; err != nil {
		t.Fatalf("load model after conflict: %v", err)
	}
	if modelRecord.DeletedAt.Valid || modelRecord.Status != expectedModelStatus {
		t.Fatalf("linked model changed after conflict: %#v", modelRecord)
	}
}

func assertNoProviderDeleteOperationLog(t *testing.T, db *gorm.DB, providerID string) {
	t.Helper()

	assertNoProviderOperationLog(t, db, providerID, "provider.delete")
}

func assertNoProviderOperationLog(t *testing.T, db *gorm.DB, providerID string, action string) {
	t.Helper()

	var count int64
	if err := db.Model(&database.OperationLog{}).
		Where("resource_type = ? AND resource_id = ? AND action = ?", "provider", providerID, action).
		Count(&count).Error; err != nil {
		t.Fatalf("count provider %s logs: %v", action, err)
	}
	if count != 0 {
		t.Fatalf("%s operation log count = %d, want 0", action, count)
	}
}

func assertProviderDeleteOperationLogExcludes(t *testing.T, db *gorm.DB, providerID string, forbidden ...string) {
	t.Helper()

	var log database.OperationLog
	if err := db.Where("resource_type = ? AND resource_id = ? AND action = ?", "provider", providerID, "provider.delete").First(&log).Error; err != nil {
		t.Fatalf("load provider delete operation log: %v", err)
	}
	assertResponseExcludes(t, log.MetadataJSON, forbidden...)
}

func assertProviderCredentialCryptoErased(t *testing.T, db *gorm.DB, tenantID string, providerID string) {
	t.Helper()

	var record database.AIProvider
	if err := db.Unscoped().Where("tenant_id = ? AND id = ?", tenantID, providerID).First(&record).Error; err != nil {
		t.Fatalf("load soft-deleted provider: %v", err)
	}
	if !record.DeletedAt.Valid {
		t.Fatalf("provider %q was not soft-deleted", providerID)
	}
	if record.EncryptedAPIKey != "" || record.APIKeyHint != "" || record.APIKeyUpdatedAt != nil {
		t.Fatalf("provider %q credential metadata was not erased", providerID)
	}
}

func errorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return payload.Error.Code
}

func assertProviderResponseHasNoSecret(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, forbidden := range []string{"sk-live-secret", "sk-rotated-secret", "sk-admin-secret", "encrypted_api_key", "encryptedapikey", "ciphertext", "authorization", "cookie"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("provider response contains %q: %s", forbidden, body)
		}
	}
}

func assertProviderOperationLogs(t *testing.T, db *gorm.DB, expectedActions []string) {
	t.Helper()

	var logs []database.OperationLog
	if err := db.Order("created_at ASC").Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}

	seen := map[string]bool{}
	for _, log := range logs {
		seen[log.Action] = true
		metadata := strings.ToLower(log.MetadataJSON)
		for _, forbidden := range []string{"sk-live-secret", "sk-rotated-secret", "sk-admin-secret", "sk-failing-secret", "sk-raw-secret", "api_key", "apikey", "api key", "authorization", "cookie", "password", "jwt", "bearer"} {
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
