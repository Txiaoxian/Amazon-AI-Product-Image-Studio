package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"gorm.io/gorm"
)

func TestUserAdminCreateUserHashesPasswordAndRedactsResponse(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	sellerRoleID := roleIDByCode(t, db, adminSession.tenantID, "seller")

	response := performJSON(router, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "  New.User@Example.com ",
		"displayName": "New User",
		"password":    "new-user-password-123",
		"roleIds":     []string{sellerRoleID},
	}, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	assertNoUserAdminSecrets(t, response.Body.String())
	data := decodeData(t, response)
	userID := stringField(t, data, "id")
	if stringField(t, data, "email") != "new.user@example.com" {
		t.Fatalf("created email = %q, want normalized lower-case email", stringField(t, data, "email"))
	}

	var user database.User
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, userID).First(&user).Error; err != nil {
		t.Fatalf("load created user: %v", err)
	}
	if user.PasswordHash == "new-user-password-123" {
		t.Fatal("password was stored in plaintext")
	}
	if !auth.CheckPassword(user.PasswordHash, "new-user-password-123") {
		t.Fatal("created password hash does not verify")
	}

	duplicateResponse := performJSON(router, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "new.user@example.com",
		"displayName": "Duplicate",
		"password":    "duplicate-password-123",
	}, adminSession.cookies, adminSession.csrfHeader())
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicate email status = %d, want %d", duplicateResponse.Code, http.StatusConflict)
	}

	weakPasswordResponse := performJSON(router, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "weak@example.com",
		"displayName": "Weak",
		"password":    "short",
	}, adminSession.cookies, adminSession.csrfHeader())
	if weakPasswordResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("weak password status = %d, want %d", weakPasswordResponse.Code, http.StatusUnprocessableEntity)
	}

	assertUserAdminOperationLogs(t, db, []string{"user.create"})
}

func TestUserAdminListPaginationTenantIsolationAndValidation(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	sellerRoleID := roleIDByCode(t, db, adminSession.tenantID, "seller")

	createManagedUser(t, router, adminSession, "list-one@example.com", "List One", "list-one-password-123", []string{sellerRoleID})
	createManagedUser(t, router, adminSession, "list-two@example.com", "List Two", "list-two-password-123", nil)
	seedUserAdminOtherTenant(t, db)

	listResponse := performJSON(router, http.MethodGet, "/api/v1/users?pageNum=1&pageSize=1&q=list&status=ACTIVE", nil, adminSession.cookies, nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list users status = %d, want %d: %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	assertNoUserAdminSecrets(t, listResponse.Body.String())
	page := decodeData(t, listResponse)
	if total := int(page["total"].(float64)); total != 2 {
		t.Fatalf("list total = %d, want 2", total)
	}
	if pageSize := int(page["pageSize"].(float64)); pageSize != 1 {
		t.Fatalf("pageSize = %d, want 1", pageSize)
	}
	for _, record := range recordsFromPage(t, listResponse) {
		if stringField(t, record, "tenantId") != adminSession.tenantID {
			t.Fatalf("list leaked cross-tenant user: %#v", record)
		}
	}
	allListResponse := performJSON(router, http.MethodGet, "/api/v1/users?pageNum=1&pageSize=2&q=list&status=ACTIVE", nil, adminSession.cookies, nil)
	if allListResponse.Code != http.StatusOK {
		t.Fatalf("full list status = %d, want %d: %s", allListResponse.Code, http.StatusOK, allListResponse.Body.String())
	}
	foundRoleInList := false
	for _, record := range recordsFromPage(t, allListResponse) {
		if stringField(t, record, "email") != "list-one@example.com" {
			continue
		}
		roles, ok := record["roles"].([]any)
		if !ok {
			t.Fatalf("list record roles is not an array: %#v", record["roles"])
		}
		foundRoleInList = len(roles) == 1
	}
	if !foundRoleInList {
		t.Fatal("user list did not include assigned role summaries")
	}

	for path, wantStatus := range map[string]int{
		"/api/v1/users?pageNum=0":                     http.StatusUnprocessableEntity,
		"/api/v1/users?pageSize=101":                  http.StatusUnprocessableEntity,
		"/api/v1/users?status=DELETED":                http.StatusUnprocessableEntity,
		"/api/v1/users?q=" + strings.Repeat("x", 129): http.StatusUnprocessableEntity,
	} {
		response := performJSON(router, http.MethodGet, path, nil, adminSession.cookies, nil)
		if response.Code != wantStatus {
			t.Fatalf("%s status = %d, want %d", path, response.Code, wantStatus)
		}
	}
}

func TestUserAdminDetailCrossTenantReturnsNotFound(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	userID := createManagedUser(t, router, adminSession, "detail@example.com", "Detail User", "detail-password-123", nil)
	otherTenantUserID, _ := seedUserAdminOtherTenant(t, db)

	detailResponse := performJSON(router, http.MethodGet, "/api/v1/users/"+userID, nil, adminSession.cookies, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d: %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}
	assertNoUserAdminSecrets(t, detailResponse.Body.String())

	crossTenantResponse := performJSON(router, http.MethodGet, "/api/v1/users/"+otherTenantUserID, nil, adminSession.cookies, nil)
	if crossTenantResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant detail status = %d, want %d", crossTenantResponse.Code, http.StatusNotFound)
	}
}

func TestUserAdminUpdateOnlyAllowsSafeFields(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	userID := createManagedUser(t, router, adminSession, "update@example.com", "Update User", "update-password-123", nil)

	updateResponse := performJSON(router, http.MethodPatch, "/api/v1/users/"+userID, map[string]any{
		"displayName": "Updated User",
		"status":      "ACTIVE",
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("safe update status = %d, want %d: %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}
	if stringField(t, decodeData(t, updateResponse), "displayName") != "Updated User" {
		t.Fatalf("displayName was not updated: %s", updateResponse.Body.String())
	}

	var before database.User
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, userID).First(&before).Error; err != nil {
		t.Fatalf("load user before unsafe update: %v", err)
	}
	unsafeResponse := performJSON(router, http.MethodPatch, "/api/v1/users/"+userID, map[string]any{
		"tenantId":     "tenant-b",
		"passwordHash": "attacker-controlled",
		"lastLoginAt":  "2026-01-01T00:00:00Z",
		"createdAt":    "2026-01-01T00:00:00Z",
	}, adminSession.cookies, adminSession.csrfHeader())
	if unsafeResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unsafe update status = %d, want %d", unsafeResponse.Code, http.StatusUnprocessableEntity)
	}
	var after database.User
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, userID).First(&after).Error; err != nil {
		t.Fatalf("load user after unsafe update: %v", err)
	}
	if after.TenantID != before.TenantID || after.PasswordHash != before.PasswordHash || !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("unsafe fields changed: before=%#v after=%#v", before, after)
	}

	assertUserAdminOperationLogs(t, db, []string{"user.update"})
}

func TestUserAdminDisableEnableWritesAuditAndInvalidatesOldSession(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	userID := createManagedUser(t, router, adminSession, "toggle@example.com", "Toggle User", "toggle-password-123", nil)
	userSession := loginProjectRouteUser(t, router, adminSession.tenantID, "toggle@example.com", "toggle-password-123")

	disableResponse := performJSON(router, http.MethodPost, "/api/v1/users/"+userID+"/disable", nil, adminSession.cookies, adminSession.csrfHeader())
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want %d: %s", disableResponse.Code, http.StatusOK, disableResponse.Body.String())
	}
	if stringField(t, decodeData(t, disableResponse), "status") != "DISABLED" {
		t.Fatalf("disabled response status mismatch: %s", disableResponse.Body.String())
	}

	oldSessionResponse := performJSON(router, http.MethodGet, "/api/v1/me", nil, userSession.cookies, nil)
	if oldSessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("disabled old session /me status = %d, want %d", oldSessionResponse.Code, http.StatusUnauthorized)
	}

	enableResponse := performJSON(router, http.MethodPost, "/api/v1/users/"+userID+"/enable", nil, adminSession.cookies, adminSession.csrfHeader())
	if enableResponse.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want %d: %s", enableResponse.Code, http.StatusOK, enableResponse.Body.String())
	}
	if stringField(t, decodeData(t, enableResponse), "status") != "ACTIVE" {
		t.Fatalf("enabled response status mismatch: %s", enableResponse.Body.String())
	}

	assertUserAdminOperationLogs(t, db, []string{"user.disable", "user.enable"})
}

func TestUserAdminDisableSelfIsRejected(t *testing.T) {
	router, _, adminSession := newProjectRouteTestRouter(t)

	response := performJSON(router, http.MethodPost, "/api/v1/users/"+adminSession.userID+"/disable", nil, adminSession.cookies, adminSession.csrfHeader())
	if response.Code != http.StatusConflict {
		t.Fatalf("disable self status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestUserAdminLastActiveAdminProtections(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	managerRoleID := seedUserAdminRole(t, db, adminSession.tenantID, "role-identity-manager", "identity-manager", auth.RoleStatusActive, []string{
		"user:read",
		"user:disable",
		"role:read",
		"role:manage",
	})
	seedActiveUser(t, db, adminSession.tenantID, "identity-manager-user", "identity-manager@example.com", "Identity Manager", "identity-manager-password-123")
	assignUserAdminRole(t, db, adminSession.tenantID, "identity-manager-user", managerRoleID, "manager")
	managerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "identity-manager@example.com", "identity-manager-password-123")

	disableLastAdminResponse := performJSON(router, http.MethodPost, "/api/v1/users/"+adminSession.userID+"/disable", nil, managerSession.cookies, managerSession.csrfHeader())
	if disableLastAdminResponse.Code != http.StatusConflict {
		t.Fatalf("disable last active admin status = %d, want %d", disableLastAdminResponse.Code, http.StatusConflict)
	}

	removeLastAdminRoleResponse := performJSON(router, http.MethodPost, "/api/v1/users/"+adminSession.userID+"/roles", map[string]any{
		"roleIds": []string{},
	}, managerSession.cookies, managerSession.csrfHeader())
	if removeLastAdminRoleResponse.Code != http.StatusConflict {
		t.Fatalf("remove last admin role status = %d, want %d", removeLastAdminRoleResponse.Code, http.StatusConflict)
	}

	var admin database.User
	if err := db.Where("tenant_id = ? AND id = ?", adminSession.tenantID, adminSession.userID).First(&admin).Error; err != nil {
		t.Fatalf("load admin after protection checks: %v", err)
	}
	if admin.Status != auth.UserStatusActive {
		t.Fatalf("admin status = %q, want ACTIVE", admin.Status)
	}
	if !userHasRoleCode(t, db, adminSession.tenantID, adminSession.userID, "admin") {
		t.Fatal("admin role was removed despite last-admin protection")
	}
}

func TestUserAdminRoleAssignmentValidationAndRollback(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	userID := createManagedUser(t, router, adminSession, "roles@example.com", "Roles User", "roles-password-123", nil)
	sellerRoleID := roleIDByCode(t, db, adminSession.tenantID, "seller")
	viewerRoleID := roleIDByCode(t, db, adminSession.tenantID, "viewer")
	disabledRoleID := seedUserAdminRole(t, db, adminSession.tenantID, "role-disabled", "disabled-role", "DISABLED", nil)
	_, crossTenantRoleID := seedUserAdminOtherTenant(t, db)

	assignResponse := performJSON(router, http.MethodPost, "/api/v1/users/"+userID+"/roles", map[string]any{
		"roleIds": []string{sellerRoleID, viewerRoleID},
	}, adminSession.cookies, adminSession.csrfHeader())
	if assignResponse.Code != http.StatusOK {
		t.Fatalf("assign roles status = %d, want %d: %s", assignResponse.Code, http.StatusOK, assignResponse.Body.String())
	}
	if countUserRoles(t, db, adminSession.tenantID, userID) != 2 {
		t.Fatal("role assignment did not write both roles")
	}

	for name, roleIDs := range map[string][]string{
		"disabled":     {disabledRoleID},
		"cross-tenant": {crossTenantRoleID},
		"nonexistent":  {"missing-role-id"},
	} {
		response := performJSON(router, http.MethodPost, "/api/v1/users/"+userID+"/roles", map[string]any{
			"roleIds": roleIDs,
		}, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s role assignment status = %d, want %d", name, response.Code, http.StatusUnprocessableEntity)
		}
		if countUserRoles(t, db, adminSession.tenantID, userID) != 2 {
			t.Fatalf("%s invalid role assignment did not roll back", name)
		}
	}

	createWithInvalidRole := performJSON(router, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "invalid-role-create@example.com",
		"displayName": "Invalid Role Create",
		"password":    "invalid-role-password-123",
		"roleIds":     []string{sellerRoleID, "missing-role-id"},
	}, adminSession.cookies, adminSession.csrfHeader())
	if createWithInvalidRole.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create with partial invalid roles status = %d, want %d", createWithInvalidRole.Code, http.StatusUnprocessableEntity)
	}
	if userEmailExists(t, db, adminSession.tenantID, "invalid-role-create@example.com") {
		t.Fatal("user was created despite invalid role set")
	}

	createWithCrossTenantRole := performJSON(router, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "cross-role-create@example.com",
		"displayName": "Cross Role Create",
		"password":    "cross-role-password-123",
		"roleIds":     []string{crossTenantRoleID},
	}, adminSession.cookies, adminSession.csrfHeader())
	if createWithCrossTenantRole.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create with cross tenant role status = %d, want %d", createWithCrossTenantRole.Code, http.StatusUnprocessableEntity)
	}
	if userEmailExists(t, db, adminSession.tenantID, "cross-role-create@example.com") {
		t.Fatal("user was created despite cross-tenant role")
	}

	assertUserAdminOperationLogs(t, db, []string{"user.roles.replace"})
}

func TestUserAdminRolesPermissionsAndUserEndpointsRequireRBAC(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	adminRolesResponse := performJSON(router, http.MethodGet, "/api/v1/roles", nil, adminSession.cookies, nil)
	if adminRolesResponse.Code != http.StatusOK {
		t.Fatalf("admin roles status = %d, want %d: %s", adminRolesResponse.Code, http.StatusOK, adminRolesResponse.Body.String())
	}
	adminPermissionsResponse := performJSON(router, http.MethodGet, "/api/v1/permissions", nil, adminSession.cookies, nil)
	if adminPermissionsResponse.Code != http.StatusOK {
		t.Fatalf("admin permissions status = %d, want %d: %s", adminPermissionsResponse.Code, http.StatusOK, adminPermissionsResponse.Body.String())
	}
	assertNoUserAdminSecrets(t, adminRolesResponse.Body.String())
	assertNoUserAdminSecrets(t, adminPermissionsResponse.Body.String())

	roleReaderRoleID := seedUserAdminRole(t, db, adminSession.tenantID, "role-role-reader", "role-reader", auth.RoleStatusActive, []string{"role:read"})
	seedActiveUser(t, db, adminSession.tenantID, "role-reader-user", "role-reader@example.com", "Role Reader", "role-reader-password-123")
	assignUserAdminRole(t, db, adminSession.tenantID, "role-reader-user", roleReaderRoleID, "role-reader")
	roleReaderSession := loginProjectRouteUser(t, router, adminSession.tenantID, "role-reader@example.com", "role-reader-password-123")
	if response := performJSON(router, http.MethodGet, "/api/v1/roles", nil, roleReaderSession.cookies, nil); response.Code != http.StatusOK {
		t.Fatalf("role:read roles status = %d, want %d", response.Code, http.StatusOK)
	}
	if response := performJSON(router, http.MethodGet, "/api/v1/permissions", nil, roleReaderSession.cookies, nil); response.Code != http.StatusOK {
		t.Fatalf("role:read permissions status = %d, want %d", response.Code, http.StatusOK)
	}
	if response := performJSON(router, http.MethodGet, "/api/v1/users", nil, roleReaderSession.cookies, nil); response.Code != http.StatusForbidden {
		t.Fatalf("role reader user list status = %d, want %d", response.Code, http.StatusForbidden)
	}

	limitedRoleID := seedUserAdminRole(t, db, adminSession.tenantID, "role-no-useradmin", "no-useradmin", auth.RoleStatusActive, nil)
	seedActiveUser(t, db, adminSession.tenantID, "no-useradmin-user", "no-useradmin@example.com", "No Useradmin", "no-useradmin-password-123")
	assignUserAdminRole(t, db, adminSession.tenantID, "no-useradmin-user", limitedRoleID, "no-useradmin")
	limitedSession := loginProjectRouteUser(t, router, adminSession.tenantID, "no-useradmin@example.com", "no-useradmin-password-123")
	for path, method := range map[string]string{
		"/api/v1/users":       http.MethodGet,
		"/api/v1/roles":       http.MethodGet,
		"/api/v1/permissions": http.MethodGet,
	} {
		response := performJSON(router, method, path, nil, limitedSession.cookies, nil)
		if response.Code != http.StatusForbidden {
			t.Fatalf("limited %s status = %d, want %d", path, response.Code, http.StatusForbidden)
		}
	}
	limitedCreateResponse := performJSON(router, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "blocked-create@example.com",
		"displayName": "Blocked Create",
		"password":    "blocked-create-password-123",
	}, limitedSession.cookies, limitedSession.csrfHeader())
	if limitedCreateResponse.Code != http.StatusForbidden {
		t.Fatalf("limited create user status = %d, want %d", limitedCreateResponse.Code, http.StatusForbidden)
	}
	limitedRoleWriteResponse := performJSON(router, http.MethodPost, "/api/v1/users/"+adminSession.userID+"/roles", map[string]any{
		"roleIds": []string{},
	}, limitedSession.cookies, limitedSession.csrfHeader())
	if limitedRoleWriteResponse.Code != http.StatusForbidden {
		t.Fatalf("limited role assignment status = %d, want %d", limitedRoleWriteResponse.Code, http.StatusForbidden)
	}

	unauthenticatedResponse := performJSON(router, http.MethodGet, "/api/v1/users", nil, nil, nil)
	if unauthenticatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated users status = %d, want %d", unauthenticatedResponse.Code, http.StatusUnauthorized)
	}
}

func createManagedUser(t *testing.T, router http.Handler, session projectRouteSession, email string, displayName string, password string, roleIDs []string) string {
	t.Helper()
	response := performJSON(router, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       email,
		"displayName": displayName,
		"password":    password,
		"roleIds":     roleIDs,
	}, session.cookies, session.csrfHeader())
	if response.Code != http.StatusCreated {
		t.Fatalf("create managed user %s status = %d, want %d: %s", email, response.Code, http.StatusCreated, response.Body.String())
	}
	return stringField(t, decodeData(t, response), "id")
}

func roleIDByCode(t *testing.T, db *gorm.DB, tenantID string, code string) string {
	t.Helper()
	var role database.Role
	if err := db.Where("tenant_id = ? AND code = ?", tenantID, code).First(&role).Error; err != nil {
		t.Fatalf("find role %s: %v", code, err)
	}
	return role.ID
}

func seedUserAdminRole(t *testing.T, db *gorm.DB, tenantID string, roleID string, code string, status string, permissionCodes []string) string {
	t.Helper()
	now := time.Now().UTC()
	role := database.Role{
		ID:          roleID,
		TenantID:    tenantID,
		Code:        code,
		Name:        code,
		Description: code + " test role",
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("seed role %s: %v", code, err)
	}
	for _, code := range permissionCodes {
		var permission database.Permission
		if err := db.Where("code = ?", code).First(&permission).Error; err != nil {
			t.Fatalf("find permission %s: %v", code, err)
		}
		if err := db.Create(&database.RolePermission{
			ID:           "role-permission-" + roleID + "-" + strings.ReplaceAll(code, ":", "-"),
			TenantID:     tenantID,
			RoleID:       role.ID,
			PermissionID: permission.ID,
			CreatedAt:    now,
		}).Error; err != nil {
			t.Fatalf("seed role permission %s: %v", code, err)
		}
	}
	return role.ID
}

func assignUserAdminRole(t *testing.T, db *gorm.DB, tenantID string, userID string, roleID string, suffix string) {
	t.Helper()
	if err := db.Create(&database.UserRole{
		ID:        "user-admin-role-" + userID + "-" + suffix,
		TenantID:  tenantID,
		UserID:    userID,
		RoleID:    roleID,
		CreatedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("assign user admin role %s to %s: %v", roleID, userID, err)
	}
}

func seedUserAdminOtherTenant(t *testing.T, db *gorm.DB) (string, string) {
	t.Helper()
	now := time.Now().UTC()
	tenantID := "tenant-b"
	if err := db.Create(&database.Tenant{
		ID:        tenantID,
		Name:      "Tenant B",
		Status:    auth.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B: %v", err)
	}
	hash, err := auth.HashPassword("tenant-b-password-123")
	if err != nil {
		t.Fatalf("hash tenant B password: %v", err)
	}
	userID := "user-tenant-b"
	if err := db.Create(&database.User{
		ID:           userID,
		TenantID:     tenantID,
		Email:        "tenant-b@example.com",
		DisplayName:  "Tenant B User",
		PasswordHash: hash,
		Status:       auth.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B user: %v", err)
	}
	roleID := "role-tenant-b"
	if err := db.Create(&database.Role{
		ID:          roleID,
		TenantID:    tenantID,
		Code:        "tenant-b-role",
		Name:        "Tenant B Role",
		Description: "Cross tenant role",
		Status:      auth.RoleStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B role: %v", err)
	}
	return userID, roleID
}

func recordsFromPage(t *testing.T, response *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var payload struct {
		Data struct {
			Records []map[string]any `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode page records: %v", err)
	}
	return payload.Data.Records
}

func countUserRoles(t *testing.T, db *gorm.DB, tenantID string, userID string) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&database.UserRole{}).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Count(&count).Error; err != nil {
		t.Fatalf("count user roles: %v", err)
	}
	return count
}

func userEmailExists(t *testing.T, db *gorm.DB, tenantID string, email string) bool {
	t.Helper()
	var count int64
	if err := db.Model(&database.User{}).Where("tenant_id = ? AND email = ?", tenantID, email).Count(&count).Error; err != nil {
		t.Fatalf("count users by email: %v", err)
	}
	return count > 0
}

func userHasRoleCode(t *testing.T, db *gorm.DB, tenantID string, userID string, roleCode string) bool {
	t.Helper()
	var count int64
	if err := db.Table("user_roles").
		Joins("JOIN roles ON roles.tenant_id = user_roles.tenant_id AND roles.id = user_roles.role_id").
		Where("user_roles.tenant_id = ? AND user_roles.user_id = ? AND roles.code = ?", tenantID, userID, roleCode).
		Count(&count).Error; err != nil {
		t.Fatalf("count user role code: %v", err)
	}
	return count > 0
}

func assertNoUserAdminSecrets(t *testing.T, body string) {
	t.Helper()
	assertNoSensitiveFields(t, body)
	lower := strings.ToLower(body)
	for _, marker := range []string{"password", "passwordhash", "token", "cookie", "authorization"} {
		if strings.Contains(lower, marker) {
			t.Fatalf("response contains sensitive marker %q: %s", marker, body)
		}
	}
}

func assertUserAdminOperationLogs(t *testing.T, db *gorm.DB, expectedActions []string) {
	t.Helper()
	var logs []database.OperationLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}
	seen := map[string]bool{}
	for _, log := range logs {
		seen[log.Action] = true
		metadata := strings.ToLower(log.MetadataJSON)
		for _, forbidden := range []string{"password", "passwordhash", "token", "cookie", "authorization", "api_key", "apikey", "jwt"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %#v", forbidden, log)
			}
		}
	}
	for _, action := range expectedActions {
		if !seen[action] {
			t.Fatalf("missing operation log action %s; logs = %#v", action, logs)
		}
	}
}
