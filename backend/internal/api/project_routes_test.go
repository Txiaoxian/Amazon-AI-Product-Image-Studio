package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"gorm.io/gorm"
)

func TestProjectRoutesCRUDMembersSoftDeleteAndAudit(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)

	createResponse := performJSON(router, http.MethodPost, "/api/v1/projects", map[string]string{
		"name":   "Premium Mug",
		"brand":  "Studio",
		"asin":   "B000TEST01",
		"site":   "US",
		"notes":  "Launch candidate",
		"status": "ACTIVE",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	projectData := decodeData(t, createResponse)
	projectID := stringField(t, projectData, "id")
	if stringField(t, projectData, "tenantId") != adminSession.tenantID {
		t.Fatalf("created project tenantId = %q, want %q", stringField(t, projectData, "tenantId"), adminSession.tenantID)
	}

	var owner database.ProjectMember
	if err := db.Where("tenant_id = ? AND project_id = ? AND user_id = ?", adminSession.tenantID, projectID, adminSession.userID).First(&owner).Error; err != nil {
		t.Fatalf("load owner project member: %v", err)
	}
	if owner.Role != project.RoleOwner {
		t.Fatalf("creator member role = %q, want OWNER", owner.Role)
	}

	seedActiveUser(t, db, adminSession.tenantID, "member-user", "member@example.com", "Member User", "member-password-123")
	memberResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "member-user",
		"role":   "EDITOR",
	}, adminSession.cookies, adminSession.csrfHeader())
	if memberResponse.Code != http.StatusCreated {
		t.Fatalf("add member status = %d, want %d: %s", memberResponse.Code, http.StatusCreated, memberResponse.Body.String())
	}
	memberData := decodeData(t, memberResponse)
	if stringField(t, memberData, "role") != project.RoleEditor {
		t.Fatalf("created member role = %q, want EDITOR", stringField(t, memberData, "role"))
	}

	listMembersResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/members", nil, adminSession.cookies, nil)
	if listMembersResponse.Code != http.StatusOK {
		t.Fatalf("list members status = %d, want %d: %s", listMembersResponse.Code, http.StatusOK, listMembersResponse.Body.String())
	}
	if len(decodeDataArray(t, listMembersResponse)) != 2 {
		t.Fatalf("member list should include owner and added member: %s", listMembersResponse.Body.String())
	}
	seedActiveUser(t, db, adminSession.tenantID, "candidate-user", "candidate@example.com", "Candidate User", "candidate-password-123")
	candidatesResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID+"/member-candidates", nil, adminSession.cookies, nil)
	if candidatesResponse.Code != http.StatusOK {
		t.Fatalf("list member candidates status = %d, want %d: %s", candidatesResponse.Code, http.StatusOK, candidatesResponse.Body.String())
	}
	candidates := decodeDataArray(t, candidatesResponse)
	if len(candidates) != 1 {
		t.Fatalf("candidate list length = %d, want 1: %s", len(candidates), candidatesResponse.Body.String())
	}
	candidate, ok := candidates[0].(map[string]any)
	if !ok {
		t.Fatalf("candidate response type = %T, want object", candidates[0])
	}
	if stringField(t, candidate, "userId") != "candidate-user" || stringField(t, candidate, "userName") != "Candidate User" {
		t.Fatalf("candidate display fields not returned as expected: %#v", candidate)
	}

	updateMemberResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+projectID+"/members/member-user", map[string]string{
		"role": "VIEWER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateMemberResponse.Code != http.StatusOK {
		t.Fatalf("update member status = %d, want %d: %s", updateMemberResponse.Code, http.StatusOK, updateMemberResponse.Body.String())
	}
	if stringField(t, decodeData(t, updateMemberResponse), "role") != project.RoleViewer {
		t.Fatalf("updated member role = %q, want VIEWER", stringField(t, decodeData(t, updateMemberResponse), "role"))
	}

	removeMemberResponse := performJSON(router, http.MethodDelete, "/api/v1/projects/"+projectID+"/members/member-user", nil, adminSession.cookies, adminSession.csrfHeader())
	if removeMemberResponse.Code != http.StatusOK {
		t.Fatalf("remove member status = %d, want %d: %s", removeMemberResponse.Code, http.StatusOK, removeMemberResponse.Body.String())
	}

	seedOtherTenantProject(t, db)
	crossTenantResponse := performJSON(router, http.MethodGet, "/api/v1/projects/project-tenant-b", nil, adminSession.cookies, nil)
	if crossTenantResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant detail status = %d, want %d", crossTenantResponse.Code, http.StatusNotFound)
	}

	updateResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+projectID, map[string]string{
		"name":   "Premium Mug Updated",
		"status": "ARCHIVED",
	}, adminSession.cookies, adminSession.csrfHeader())
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update project status = %d, want %d: %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}
	if stringField(t, decodeData(t, updateResponse), "status") != project.StatusArchived {
		t.Fatalf("updated project status = %q, want ARCHIVED", stringField(t, decodeData(t, updateResponse), "status"))
	}

	detailResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID, nil, adminSession.cookies, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("project detail status = %d, want %d: %s", detailResponse.Code, http.StatusOK, detailResponse.Body.String())
	}

	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/projects/"+projectID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete project status = %d, want %d: %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}

	deletedDetailResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID, nil, adminSession.cookies, nil)
	if deletedDetailResponse.Code != http.StatusNotFound {
		t.Fatalf("deleted detail status = %d, want %d", deletedDetailResponse.Code, http.StatusNotFound)
	}
	listResponse := performJSON(router, http.MethodGet, "/api/v1/projects", nil, adminSession.cookies, nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list projects status = %d, want %d: %s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	listData := decodeData(t, listResponse)
	if total, ok := listData["total"].(float64); !ok || total != 0 {
		t.Fatalf("list total = %#v, want 0", listData["total"])
	}

	assertProjectOperationLogs(t, db, []string{
		"project.create",
		"project.update",
		"project.delete",
		"project_member.create",
		"project_member.update",
		"project_member.delete",
	})
}

func TestProjectRoutesRequireRBACAndProjectRoleForNormalUsers(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)

	createResponse := performJSON(router, http.MethodPost, "/api/v1/projects", map[string]string{
		"name": "Editable Project",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	projectID := stringField(t, decodeData(t, createResponse), "id")

	createSecondResponse := performJSON(router, http.MethodPost, "/api/v1/projects", map[string]string{
		"name": "Admin Only Project",
	}, adminSession.cookies, adminSession.csrfHeader())
	if createSecondResponse.Code != http.StatusCreated {
		t.Fatalf("create second project status = %d, want %d", createSecondResponse.Code, http.StatusCreated)
	}
	secondProjectID := stringField(t, decodeData(t, createSecondResponse), "id")

	seedActiveUser(t, db, adminSession.tenantID, "seller-user", "seller@example.com", "Seller User", "seller-password-123")
	assignRole(t, db, adminSession.tenantID, "seller-user", "seller")
	addSellerResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "seller-user",
		"role":   "VIEWER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if addSellerResponse.Code != http.StatusCreated {
		t.Fatalf("add seller member status = %d, want %d: %s", addSellerResponse.Code, http.StatusCreated, addSellerResponse.Body.String())
	}

	sellerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "seller@example.com", "seller-password-123")
	viewResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID, nil, sellerSession.cookies, nil)
	if viewResponse.Code != http.StatusOK {
		t.Fatalf("seller viewer detail status = %d, want %d: %s", viewResponse.Code, http.StatusOK, viewResponse.Body.String())
	}

	viewerUpdateResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+projectID, map[string]string{
		"name": "Viewer Cannot Update",
	}, sellerSession.cookies, sellerSession.csrfHeader())
	if viewerUpdateResponse.Code != http.StatusForbidden {
		t.Fatalf("seller viewer update status = %d, want %d", viewerUpdateResponse.Code, http.StatusForbidden)
	}

	nonMemberResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+secondProjectID, nil, sellerSession.cookies, nil)
	if nonMemberResponse.Code != http.StatusForbidden {
		t.Fatalf("seller non-member detail status = %d, want %d", nonMemberResponse.Code, http.StatusForbidden)
	}

	promoteResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+projectID+"/members/seller-user", map[string]string{
		"role": "EDITOR",
	}, adminSession.cookies, adminSession.csrfHeader())
	if promoteResponse.Code != http.StatusOK {
		t.Fatalf("promote seller status = %d, want %d: %s", promoteResponse.Code, http.StatusOK, promoteResponse.Body.String())
	}
	editorUpdateResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+projectID, map[string]string{
		"name": "Editor Can Update",
	}, sellerSession.cookies, sellerSession.csrfHeader())
	if editorUpdateResponse.Code != http.StatusOK {
		t.Fatalf("seller editor update status = %d, want %d: %s", editorUpdateResponse.Code, http.StatusOK, editorUpdateResponse.Body.String())
	}

	seedActiveUser(t, db, adminSession.tenantID, "limited-user", "limited@example.com", "Limited User", "limited-password-123")
	assignRole(t, db, adminSession.tenantID, "limited-user", "limited")
	addLimitedResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "limited-user",
		"role":   "OWNER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if addLimitedResponse.Code != http.StatusCreated {
		t.Fatalf("add limited member status = %d, want %d: %s", addLimitedResponse.Code, http.StatusCreated, addLimitedResponse.Body.String())
	}
	limitedSession := loginProjectRouteUser(t, router, adminSession.tenantID, "limited@example.com", "limited-password-123")
	limitedReadResponse := performJSON(router, http.MethodGet, "/api/v1/projects/"+projectID, nil, limitedSession.cookies, nil)
	if limitedReadResponse.Code != http.StatusForbidden {
		t.Fatalf("limited member without RBAC status = %d, want %d", limitedReadResponse.Code, http.StatusForbidden)
	}
}

func TestProjectMemberRoutesRejectRemovingOrDowngradingLastOwner(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	projectID := createProjectForTest(t, router, adminSession, "Last Owner Guard")

	deleteLogsBefore := countProjectOperationLogs(t, db, "project_member.delete")
	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/projects/"+projectID+"/members/"+adminSession.userID, nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusConflict {
		t.Fatalf("delete last owner status = %d, want %d: %s", deleteResponse.Code, http.StatusConflict, deleteResponse.Body.String())
	}
	assertProjectMemberRole(t, db, adminSession.tenantID, projectID, adminSession.userID, project.RoleOwner)
	if got := countProjectOperationLogs(t, db, "project_member.delete"); got != deleteLogsBefore {
		t.Fatalf("blocked last owner delete wrote operation log count = %d, want %d", got, deleteLogsBefore)
	}

	updateLogsBefore := countProjectOperationLogs(t, db, "project_member.update")
	downgradeResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+projectID+"/members/"+adminSession.userID, map[string]string{
		"role": "EDITOR",
	}, adminSession.cookies, adminSession.csrfHeader())
	if downgradeResponse.Code != http.StatusConflict {
		t.Fatalf("downgrade last owner status = %d, want %d: %s", downgradeResponse.Code, http.StatusConflict, downgradeResponse.Body.String())
	}
	assertProjectMemberRole(t, db, adminSession.tenantID, projectID, adminSession.userID, project.RoleOwner)
	if got := countProjectOperationLogs(t, db, "project_member.update"); got != updateLogsBefore {
		t.Fatalf("blocked last owner downgrade wrote operation log count = %d, want %d", got, updateLogsBefore)
	}

	noopResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+projectID+"/members/"+adminSession.userID, map[string]string{
		"role": "OWNER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if noopResponse.Code != http.StatusOK {
		t.Fatalf("last owner update to OWNER status = %d, want %d: %s", noopResponse.Code, http.StatusOK, noopResponse.Body.String())
	}
	assertProjectMemberRole(t, db, adminSession.tenantID, projectID, adminSession.userID, project.RoleOwner)
}

func TestProjectMemberRoutesAllowOwnerTransferWhenAnotherOwnerRemains(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)

	deleteProjectID := createProjectForTest(t, router, adminSession, "Delete One Owner")
	seedActiveUser(t, db, adminSession.tenantID, "owner-delete-user", "owner-delete@example.com", "Owner Delete", "owner-delete-password-123")
	addOwnerResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+deleteProjectID+"/members", map[string]string{
		"userId": "owner-delete-user",
		"role":   "OWNER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if addOwnerResponse.Code != http.StatusCreated {
		t.Fatalf("add owner for delete status = %d, want %d: %s", addOwnerResponse.Code, http.StatusCreated, addOwnerResponse.Body.String())
	}
	deleteLogsBefore := countProjectOperationLogs(t, db, "project_member.delete")
	deleteResponse := performJSON(router, http.MethodDelete, "/api/v1/projects/"+deleteProjectID+"/members/owner-delete-user", nil, adminSession.cookies, adminSession.csrfHeader())
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete one of two owners status = %d, want %d: %s", deleteResponse.Code, http.StatusOK, deleteResponse.Body.String())
	}
	assertProjectMemberMissing(t, db, adminSession.tenantID, deleteProjectID, "owner-delete-user")
	assertProjectMemberRole(t, db, adminSession.tenantID, deleteProjectID, adminSession.userID, project.RoleOwner)
	if got := countProjectOperationLogs(t, db, "project_member.delete"); got != deleteLogsBefore+1 {
		t.Fatalf("successful owner delete operation log count = %d, want %d", got, deleteLogsBefore+1)
	}

	downgradeProjectID := createProjectForTest(t, router, adminSession, "Downgrade One Owner")
	seedActiveUser(t, db, adminSession.tenantID, "owner-downgrade-user", "owner-downgrade@example.com", "Owner Downgrade", "owner-downgrade-password-123")
	addDowngradeOwnerResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+downgradeProjectID+"/members", map[string]string{
		"userId": "owner-downgrade-user",
		"role":   "OWNER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if addDowngradeOwnerResponse.Code != http.StatusCreated {
		t.Fatalf("add owner for downgrade status = %d, want %d: %s", addDowngradeOwnerResponse.Code, http.StatusCreated, addDowngradeOwnerResponse.Body.String())
	}
	updateLogsBefore := countProjectOperationLogs(t, db, "project_member.update")
	downgradeResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+downgradeProjectID+"/members/owner-downgrade-user", map[string]string{
		"role": "VIEWER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if downgradeResponse.Code != http.StatusOK {
		t.Fatalf("downgrade one of two owners status = %d, want %d: %s", downgradeResponse.Code, http.StatusOK, downgradeResponse.Body.String())
	}
	assertProjectMemberRole(t, db, adminSession.tenantID, downgradeProjectID, "owner-downgrade-user", project.RoleViewer)
	assertProjectMemberRole(t, db, adminSession.tenantID, downgradeProjectID, adminSession.userID, project.RoleOwner)
	if got := countProjectOperationLogs(t, db, "project_member.update"); got != updateLogsBefore+1 {
		t.Fatalf("successful owner downgrade operation log count = %d, want %d", got, updateLogsBefore+1)
	}

	transferProjectID := createProjectForTest(t, router, adminSession, "Transfer Owner")
	seedActiveUser(t, db, adminSession.tenantID, "new-owner-user", "new-owner@example.com", "New Owner", "new-owner-password-123")
	addNewOwnerResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+transferProjectID+"/members", map[string]string{
		"userId": "new-owner-user",
		"role":   "OWNER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if addNewOwnerResponse.Code != http.StatusCreated {
		t.Fatalf("add second owner status = %d, want %d: %s", addNewOwnerResponse.Code, http.StatusCreated, addNewOwnerResponse.Body.String())
	}
	removeOriginalResponse := performJSON(router, http.MethodDelete, "/api/v1/projects/"+transferProjectID+"/members/"+adminSession.userID, nil, adminSession.cookies, adminSession.csrfHeader())
	if removeOriginalResponse.Code != http.StatusOK {
		t.Fatalf("delete original owner after transfer status = %d, want %d: %s", removeOriginalResponse.Code, http.StatusOK, removeOriginalResponse.Body.String())
	}
	assertProjectMemberMissing(t, db, adminSession.tenantID, transferProjectID, adminSession.userID)
	assertProjectMemberRole(t, db, adminSession.tenantID, transferProjectID, "new-owner-user", project.RoleOwner)
}

func TestProjectMemberRoutesValidationTenantAndRBACRegressions(t *testing.T) {
	router, db, adminSession := newProjectRouteTestRouter(t)
	projectID := createProjectForTest(t, router, adminSession, "Member Guard Regressions")

	seedActiveUser(t, db, adminSession.tenantID, "duplicate-user", "duplicate@example.com", "Duplicate User", "duplicate-password-123")
	addResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "duplicate-user",
		"role":   "EDITOR",
	}, adminSession.cookies, adminSession.csrfHeader())
	if addResponse.Code != http.StatusCreated {
		t.Fatalf("add duplicate base member status = %d, want %d: %s", addResponse.Code, http.StatusCreated, addResponse.Body.String())
	}
	duplicateResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "duplicate-user",
		"role":   "VIEWER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicate member status = %d, want %d: %s", duplicateResponse.Code, http.StatusConflict, duplicateResponse.Body.String())
	}

	invalidRoleResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "duplicate-user",
		"role":   "ADMIN",
	}, adminSession.cookies, adminSession.csrfHeader())
	if invalidRoleResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid role status = %d, want %d: %s", invalidRoleResponse.Code, http.StatusUnprocessableEntity, invalidRoleResponse.Body.String())
	}

	missingUserResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "missing-user",
		"role":   "VIEWER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if missingUserResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing target user status = %d, want %d: %s", missingUserResponse.Code, http.StatusUnprocessableEntity, missingUserResponse.Body.String())
	}

	seedInactiveUser(t, db, adminSession.tenantID, "inactive-user", "inactive@example.com", "Inactive User")
	inactiveUserResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "inactive-user",
		"role":   "VIEWER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if inactiveUserResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("inactive target user status = %d, want %d: %s", inactiveUserResponse.Code, http.StatusUnprocessableEntity, inactiveUserResponse.Body.String())
	}

	seedOtherTenantProject(t, db)
	crossTenantCases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{
			name:   "add",
			method: http.MethodPost,
			path:   "/api/v1/projects/project-tenant-b/members",
			body: map[string]string{
				"userId": adminSession.userID,
				"role":   "VIEWER",
			},
		},
		{
			name:   "update",
			method: http.MethodPatch,
			path:   "/api/v1/projects/project-tenant-b/members/user-tenant-b",
			body:   map[string]string{"role": "VIEWER"},
		},
		{
			name:   "delete",
			method: http.MethodDelete,
			path:   "/api/v1/projects/project-tenant-b/members/user-tenant-b",
		},
	}
	for _, tc := range crossTenantCases {
		response := performJSON(router, tc.method, tc.path, tc.body, adminSession.cookies, adminSession.csrfHeader())
		if response.Code != http.StatusNotFound {
			t.Fatalf("cross-tenant member %s status = %d, want %d: %s", tc.name, response.Code, http.StatusNotFound, response.Body.String())
		}
		for _, forbidden := range []string{"tenant-b", "project-tenant-b", "user-tenant-b"} {
			if strings.Contains(response.Body.String(), forbidden) {
				t.Fatalf("cross-tenant member %s leaked %q: %s", tc.name, forbidden, response.Body.String())
			}
		}
	}

	seedActiveUser(t, db, adminSession.tenantID, "manager-user", "manager@example.com", "Manager User", "manager-password-123")
	assignRoleWithPermissions(t, db, adminSession.tenantID, "manager-user", "member-manager", []string{"project:read", "project:member:manage"})
	addManagerResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "manager-user",
		"role":   "VIEWER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if addManagerResponse.Code != http.StatusCreated {
		t.Fatalf("add manager member status = %d, want %d: %s", addManagerResponse.Code, http.StatusCreated, addManagerResponse.Body.String())
	}
	managerSession := loginProjectRouteUser(t, router, adminSession.tenantID, "manager@example.com", "manager-password-123")
	seedActiveUser(t, db, adminSession.tenantID, "manager-target-user", "manager-target@example.com", "Manager Target", "manager-target-password-123")
	nonOwnerManageResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "manager-target-user",
		"role":   "VIEWER",
	}, managerSession.cookies, managerSession.csrfHeader())
	if nonOwnerManageResponse.Code != http.StatusForbidden {
		t.Fatalf("member manager non-owner add status = %d, want %d: %s", nonOwnerManageResponse.Code, http.StatusForbidden, nonOwnerManageResponse.Body.String())
	}
	promoteManagerResponse := performJSON(router, http.MethodPatch, "/api/v1/projects/"+projectID+"/members/manager-user", map[string]string{
		"role": "OWNER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if promoteManagerResponse.Code != http.StatusOK {
		t.Fatalf("promote manager status = %d, want %d: %s", promoteManagerResponse.Code, http.StatusOK, promoteManagerResponse.Body.String())
	}
	ownerWithRBACManageResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "manager-target-user",
		"role":   "VIEWER",
	}, managerSession.cookies, managerSession.csrfHeader())
	if ownerWithRBACManageResponse.Code != http.StatusCreated {
		t.Fatalf("member manager owner add status = %d, want %d: %s", ownerWithRBACManageResponse.Code, http.StatusCreated, ownerWithRBACManageResponse.Body.String())
	}

	seedActiveUser(t, db, adminSession.tenantID, "owner-no-rbac-user", "owner-no-rbac@example.com", "Owner No RBAC", "owner-no-rbac-password-123")
	assignRole(t, db, adminSession.tenantID, "owner-no-rbac-user", "limited")
	addOwnerNoRBACResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "owner-no-rbac-user",
		"role":   "OWNER",
	}, adminSession.cookies, adminSession.csrfHeader())
	if addOwnerNoRBACResponse.Code != http.StatusCreated {
		t.Fatalf("add owner without RBAC status = %d, want %d: %s", addOwnerNoRBACResponse.Code, http.StatusCreated, addOwnerNoRBACResponse.Body.String())
	}
	ownerNoRBACSession := loginProjectRouteUser(t, router, adminSession.tenantID, "owner-no-rbac@example.com", "owner-no-rbac-password-123")
	seedActiveUser(t, db, adminSession.tenantID, "owner-no-rbac-target-user", "owner-no-rbac-target@example.com", "Owner No RBAC Target", "owner-no-rbac-target-password-123")
	ownerNoRBACManageResponse := performJSON(router, http.MethodPost, "/api/v1/projects/"+projectID+"/members", map[string]string{
		"userId": "owner-no-rbac-target-user",
		"role":   "VIEWER",
	}, ownerNoRBACSession.cookies, ownerNoRBACSession.csrfHeader())
	if ownerNoRBACManageResponse.Code != http.StatusForbidden {
		t.Fatalf("owner without RBAC add status = %d, want %d: %s", ownerNoRBACManageResponse.Code, http.StatusForbidden, ownerNoRBACManageResponse.Body.String())
	}
}

type projectRouteSession struct {
	tenantID string
	userID   string
	cookies  []*http.Cookie
	csrf     string
}

func (s projectRouteSession) csrfHeader() map[string]string {
	return map[string]string{"X-CSRF-Token": s.csrf}
}

func newProjectRouteTestRouter(t *testing.T) (http.Handler, *gorm.DB, projectRouteSession) {
	t.Helper()

	db := newAuthRouteTestDB(t)
	router := NewRouter(RouterOptions{
		Config:   authRouteTestConfig("test"),
		Logger:   discardLogger(),
		Database: db,
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

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func loginProjectRouteUser(t *testing.T, router http.Handler, tenantID string, email string, password string) projectRouteSession {
	t.Helper()

	response := performJSON(router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"tenantId": tenantID,
		"email":    email,
		"password": password,
	}, nil, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("login %s status = %d, want %d: %s", email, response.Code, http.StatusOK, response.Body.String())
	}
	data := decodeData(t, response)
	authCookie := findCookie(t, response, "studio_auth")
	csrfCookie := findCookie(t, response, "studio_csrf")
	return projectRouteSession{
		tenantID: tenantID,
		userID:   nestedString(t, data, "user", "id"),
		cookies:  []*http.Cookie{authCookie, csrfCookie},
		csrf:     csrfCookie.Value,
	}
}

func seedActiveUser(t *testing.T, db *gorm.DB, tenantID string, userID string, email string, displayName string, password string) {
	t.Helper()

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now().UTC()
	if err := db.Create(&database.User{
		ID:           userID,
		TenantID:     tenantID,
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: hash,
		Status:       auth.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed active user %s: %v", userID, err)
	}
}

func assignRole(t *testing.T, db *gorm.DB, tenantID string, userID string, roleCode string) {
	t.Helper()

	var role database.Role
	if roleCode == "limited" {
		now := time.Now().UTC()
		role = database.Role{
			ID:          "role-limited",
			TenantID:    tenantID,
			Code:        "limited",
			Name:        "Limited",
			Description: "No project permissions",
			Status:      auth.RoleStatusActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := db.Create(&role).Error; err != nil {
			t.Fatalf("seed limited role: %v", err)
		}
	} else if err := db.Where("tenant_id = ? AND code = ?", tenantID, roleCode).First(&role).Error; err != nil {
		t.Fatalf("find role %s: %v", roleCode, err)
	}

	if err := db.Create(&database.UserRole{
		ID:        "user-role-" + userID + "-" + roleCode,
		TenantID:  tenantID,
		UserID:    userID,
		RoleID:    role.ID,
		CreatedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("assign role %s to %s: %v", roleCode, userID, err)
	}
}

func assignRoleWithPermissions(t *testing.T, db *gorm.DB, tenantID string, userID string, roleCode string, permissionCodes []string) {
	t.Helper()

	now := time.Now().UTC()
	role := database.Role{
		ID:          "role-" + roleCode,
		TenantID:    tenantID,
		Code:        roleCode,
		Name:        roleCode,
		Description: "Project member route test role",
		Status:      auth.RoleStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("seed role %s: %v", roleCode, err)
	}
	for _, code := range permissionCodes {
		var permission database.Permission
		if err := db.Where("code = ?", code).First(&permission).Error; err != nil {
			t.Fatalf("find permission %s: %v", code, err)
		}
		if err := db.Create(&database.RolePermission{
			ID:           "role-permission-" + roleCode + "-" + code,
			TenantID:     tenantID,
			RoleID:       role.ID,
			PermissionID: permission.ID,
			CreatedAt:    now,
		}).Error; err != nil {
			t.Fatalf("assign permission %s to role %s: %v", code, roleCode, err)
		}
	}
	if err := db.Create(&database.UserRole{
		ID:        "user-role-" + userID + "-" + roleCode,
		TenantID:  tenantID,
		UserID:    userID,
		RoleID:    role.ID,
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("assign role %s to %s: %v", roleCode, userID, err)
	}
}

func seedInactiveUser(t *testing.T, db *gorm.DB, tenantID string, userID string, email string, displayName string) {
	t.Helper()

	if err := db.Create(&database.User{
		ID:           userID,
		TenantID:     tenantID,
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: "inactive-password-hash",
		Status:       "DISABLED",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("seed inactive user %s: %v", userID, err)
	}
}

func createProjectForTest(t *testing.T, router http.Handler, session projectRouteSession, name string) string {
	t.Helper()

	response := performJSON(router, http.MethodPost, "/api/v1/projects", map[string]string{
		"name": name,
	}, session.cookies, session.csrfHeader())
	if response.Code != http.StatusCreated {
		t.Fatalf("create project %q status = %d, want %d: %s", name, response.Code, http.StatusCreated, response.Body.String())
	}
	return stringField(t, decodeData(t, response), "id")
}

func assertProjectMemberRole(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string, role string) {
	t.Helper()

	var member database.ProjectMember
	if err := db.Where("tenant_id = ? AND project_id = ? AND user_id = ?", tenantID, projectID, userID).First(&member).Error; err != nil {
		t.Fatalf("load project member %s/%s: %v", projectID, userID, err)
	}
	if member.Role != role {
		t.Fatalf("member %s/%s role = %q, want %q", projectID, userID, member.Role, role)
	}
}

func assertProjectMemberMissing(t *testing.T, db *gorm.DB, tenantID string, projectID string, userID string) {
	t.Helper()

	var count int64
	if err := db.Model(&database.ProjectMember{}).Where("tenant_id = ? AND project_id = ? AND user_id = ?", tenantID, projectID, userID).Count(&count).Error; err != nil {
		t.Fatalf("count project member %s/%s: %v", projectID, userID, err)
	}
	if count != 0 {
		t.Fatalf("member %s/%s exists, want missing", projectID, userID)
	}
}

func countProjectOperationLogs(t *testing.T, db *gorm.DB, action string) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&database.OperationLog{}).Where("action = ?", action).Count(&count).Error; err != nil {
		t.Fatalf("count operation logs %s: %v", action, err)
	}
	return count
}

func seedOtherTenantProject(t *testing.T, db *gorm.DB) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Create(&database.Tenant{
		ID:        "tenant-b",
		Name:      "Tenant B",
		Status:    auth.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B: %v", err)
	}
	if err := db.Create(&database.User{
		ID:           "user-tenant-b",
		TenantID:     "tenant-b",
		Email:        "tenant-b@example.com",
		DisplayName:  "Tenant B User",
		PasswordHash: "hash",
		Status:       auth.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B user: %v", err)
	}
	if err := db.Create(&database.Project{
		ID:        "project-tenant-b",
		TenantID:  "tenant-b",
		Name:      "Tenant B Project",
		Status:    project.StatusActive,
		CreatedBy: "user-tenant-b",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant B project: %v", err)
	}
}

func decodeDataArray(t *testing.T, response *httptest.ResponseRecorder) []any {
	t.Helper()

	var payload struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response array: %v", err)
	}
	return payload.Data
}

func stringField(t *testing.T, data map[string]any, key string) string {
	t.Helper()
	value, ok := data[key].(string)
	if !ok {
		t.Fatalf("response data.%s is not a string: %#v", key, data[key])
	}
	return value
}

func assertProjectOperationLogs(t *testing.T, db *gorm.DB, expectedActions []string) {
	t.Helper()

	var logs []database.OperationLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}

	seen := map[string]bool{}
	for _, log := range logs {
		seen[log.Action] = true
		metadata := strings.ToLower(log.MetadataJSON)
		for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "apikey", "jwt"} {
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
