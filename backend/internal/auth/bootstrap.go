package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"gorm.io/gorm"
)

type builtInRole struct {
	Code        string
	Name        string
	Description string
	Permissions []string
}

var builtInPermissionCodes = []string{
	"user:read",
	"user:create",
	"user:update",
	"user:disable",
	"role:read",
	"role:manage",
	"project:read",
	"project:create",
	"project:update",
	"project:delete",
	"project:member:manage",
	"asset:read",
	"asset:upload",
	"asset:update",
	"asset:delete",
	"asset:download",
	"task:read",
	"task:create",
	"task:cancel",
	"task:retry",
	"provider:read",
	"provider:manage",
	"model:read",
	"model:manage",
	"usage:read",
	"audit:read",
	"system:settings:manage",
}

var builtInRoles = []builtInRole{
	{
		Code:        "admin",
		Name:        "Admin",
		Description: "Tenant administrator",
		Permissions: builtInPermissionCodes,
	},
	{
		Code:        "seller",
		Name:        "Seller",
		Description: "Operational seller user",
		Permissions: []string{
			"project:read",
			"project:create",
			"project:update",
			"asset:read",
			"asset:upload",
			"asset:update",
			"asset:delete",
			"asset:download",
			"task:read",
			"task:create",
			"task:cancel",
			"task:retry",
			"provider:read",
			"model:read",
			"usage:read",
		},
	},
	{
		Code:        "viewer",
		Name:        "Viewer",
		Description: "Read-only user",
		Permissions: []string{
			"project:read",
			"asset:read",
			"asset:download",
			"task:read",
		},
	},
}

func ReconcileBuiltInRoles(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var tenants []database.Tenant
	if err := db.WithContext(ctx).Select("id").Order("id ASC").Find(&tenants).Error; err != nil {
		return fmt.Errorf("list tenants for built-in role reconciliation: %w", err)
	}

	now := time.Now().UTC()
	service := &Service{}
	for _, tenant := range tenants {
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, err := service.seedBuiltInRoles(ctx, tx, tenant.ID, now)
			return err
		}); err != nil {
			return fmt.Errorf("reconcile tenant built-in roles: %w", err)
		}
	}

	return nil
}

func (s *Service) seedBuiltInRoles(ctx context.Context, tx *gorm.DB, tenantID string, now time.Time) (map[string]database.Role, error) {
	permissionsByCode := make(map[string]database.Permission, len(builtInPermissionCodes))
	for _, code := range builtInPermissionCodes {
		permission, err := ensurePermission(ctx, tx, code, now)
		if err != nil {
			return nil, err
		}
		permissionsByCode[code] = permission
	}

	rolesByCode := make(map[string]database.Role, len(builtInRoles))
	for _, builtIn := range builtInRoles {
		role, err := ensureRole(ctx, tx, tenantID, builtIn, now)
		if err != nil {
			return nil, err
		}
		rolesByCode[builtIn.Code] = role

		for _, code := range builtIn.Permissions {
			permission, ok := permissionsByCode[code]
			if !ok {
				return nil, errors.New("missing built-in permission " + code)
			}
			if err := ensureRolePermission(ctx, tx, tenantID, role.ID, permission.ID, now); err != nil {
				return nil, err
			}
		}
	}

	return rolesByCode, nil
}

func ensurePermission(ctx context.Context, tx *gorm.DB, code string, now time.Time) (database.Permission, error) {
	code = strings.TrimSpace(code)
	var permission database.Permission
	err := tx.WithContext(ctx).Where("code = ?", code).First(&permission).Error
	if err == nil {
		return permission, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Permission{}, err
	}

	permission = database.Permission{
		ID:          idgen.New(),
		Code:        code,
		Name:        code,
		Description: "Built-in permission " + code,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return permission, tx.WithContext(ctx).Create(&permission).Error
}

func ensureRole(ctx context.Context, tx *gorm.DB, tenantID string, builtIn builtInRole, now time.Time) (database.Role, error) {
	var role database.Role
	err := tx.WithContext(ctx).Where("tenant_id = ? AND code = ?", tenantID, builtIn.Code).First(&role).Error
	if err == nil {
		return role, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Role{}, err
	}

	role = database.Role{
		ID:          idgen.New(),
		TenantID:    tenantID,
		Code:        builtIn.Code,
		Name:        builtIn.Name,
		Description: builtIn.Description,
		Status:      RoleStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return role, tx.WithContext(ctx).Create(&role).Error
}

func ensureRolePermission(ctx context.Context, tx *gorm.DB, tenantID string, roleID string, permissionID string, now time.Time) error {
	var existing database.RolePermission
	err := tx.WithContext(ctx).
		Where("tenant_id = ? AND role_id = ? AND permission_id = ?", tenantID, roleID, permissionID).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return tx.WithContext(ctx).Create(&database.RolePermission{
		ID:           idgen.New(),
		TenantID:     tenantID,
		RoleID:       roleID,
		PermissionID: permissionID,
		CreatedAt:    now,
	}).Error
}
