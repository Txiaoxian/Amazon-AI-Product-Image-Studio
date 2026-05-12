package project

import (
	"context"
	"errors"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

type Authorizer struct {
	repo Repository
}

func NewAuthorizer(db *gorm.DB) Authorizer {
	return Authorizer{repo: NewRepository(db)}
}

func (a Authorizer) withDB(db *gorm.DB) Authorizer {
	return Authorizer{repo: NewRepository(db)}
}

func (a Authorizer) Authorize(ctx context.Context, principal auth.Principal, projectID string, permission string, allowedRoles ...string) (database.Project, error) {
	scope, err := tenant.NewScope(principal.TenantID)
	if err != nil {
		return database.Project{}, err
	}

	record, err := a.repo.FindProject(ctx, scope, projectID)
	if err != nil {
		return database.Project{}, err
	}

	if IsTenantAdmin(principal) {
		return record, nil
	}
	if !principal.HasPermission(permission) {
		return database.Project{}, ErrForbidden
	}

	member, err := a.repo.FindMember(ctx, scope, record.ID, principal.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return database.Project{}, ErrForbidden
		}
		return database.Project{}, err
	}
	if !roleAllowed(member.Role, allowedRoles) {
		return database.Project{}, ErrForbidden
	}

	return record, nil
}

func IsTenantAdmin(principal auth.Principal) bool {
	for _, role := range principal.Roles {
		if role.Code == "admin" {
			return true
		}
	}
	return false
}

func roleAllowed(actual string, allowed []string) bool {
	for _, role := range allowed {
		if actual == role {
			return true
		}
	}
	return false
}

func rolesForPermission(permission string) []string {
	switch permission {
	case PermissionDelete, PermissionMemberManage:
		return []string{RoleOwner}
	case PermissionUpdate:
		return []string{RoleOwner, RoleEditor}
	default:
		return []string{RoleOwner, RoleEditor, RoleViewer}
	}
}
