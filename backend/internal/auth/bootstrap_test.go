package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestReconcileBuiltInRolesBackfillsMissingRolesAndAssetPermissions(t *testing.T) {
	db := openBootstrapTestDB(t)
	now := time.Now().UTC()
	tenant := database.Tenant{ID: "tenant-legacy", Name: "Legacy tenant", Status: TenantStatusActive, CreatedAt: now, UpdatedAt: now}
	adminRole := database.Role{ID: "role-admin", TenantID: tenant.ID, Code: "admin", Name: "Admin", Status: RoleStatusActive, CreatedAt: now, UpdatedAt: now}
	sellerRole := database.Role{ID: "role-seller", TenantID: tenant.ID, Code: "seller", Name: "Seller", Status: RoleStatusActive, CreatedAt: now, UpdatedAt: now}
	customRole := database.Role{ID: "role-custom", TenantID: tenant.ID, Code: "catalog-editor", Name: "Catalog editor", Status: RoleStatusActive, CreatedAt: now, UpdatedAt: now}
	projectRead := database.Permission{ID: "permission-project-read", Code: "project:read", Name: "project:read", CreatedAt: now, UpdatedAt: now}
	providerRead := database.Permission{ID: "permission-provider-read", Code: "provider:read", Name: "provider:read", CreatedAt: now, UpdatedAt: now}
	existingGrant := database.RolePermission{ID: "grant-admin-project-read", TenantID: tenant.ID, RoleID: adminRole.ID, PermissionID: projectRead.ID, CreatedAt: now}
	customGrant := database.RolePermission{ID: "grant-custom-project-read", TenantID: tenant.ID, RoleID: customRole.ID, PermissionID: projectRead.ID, CreatedAt: now}
	legacySellerProviderGrant := database.RolePermission{ID: "grant-seller-provider-read", TenantID: tenant.ID, RoleID: sellerRole.ID, PermissionID: providerRead.ID, CreatedAt: now}

	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if err := db.Create([]database.Role{adminRole, sellerRole, customRole}).Error; err != nil {
		t.Fatalf("create roles: %v", err)
	}
	if err := db.Create([]database.Permission{projectRead, providerRead}).Error; err != nil {
		t.Fatalf("create permission: %v", err)
	}
	if err := db.Create([]database.RolePermission{existingGrant, customGrant, legacySellerProviderGrant}).Error; err != nil {
		t.Fatalf("create role permissions: %v", err)
	}

	if err := ReconcileBuiltInRoles(context.Background(), db); err != nil {
		t.Fatalf("ReconcileBuiltInRoles returned error: %v", err)
	}

	assertRoleHasPermissions(t, db, tenant.ID, "admin", builtInPermissionCodes)
	assertRoleHasPermissions(t, db, tenant.ID, "seller", []string{"asset:read", "asset:upload", "asset:update", "asset:delete", "asset:download"})
	assertRoleHasPermissions(t, db, tenant.ID, "viewer", []string{"asset:read", "asset:download"})

	var reconciledPermission database.Permission
	if err := db.Where("code = ?", "project:read").First(&reconciledPermission).Error; err != nil {
		t.Fatalf("load reconciled project:read permission: %v", err)
	}
	if reconciledPermission.ID != projectRead.ID {
		t.Fatalf("reconciled permission ID = %q, want existing %q", reconciledPermission.ID, projectRead.ID)
	}
	if reconciledPermission.Name != "查看产品" || reconciledPermission.Description != "查看当前租户中有权限访问的产品。" {
		t.Fatalf("reconciled permission text = %q / %q", reconciledPermission.Name, reconciledPermission.Description)
	}

	var reconciledAdmin database.Role
	if err := db.Where("tenant_id = ? AND code = ?", tenant.ID, "admin").First(&reconciledAdmin).Error; err != nil {
		t.Fatalf("load reconciled admin role: %v", err)
	}
	if reconciledAdmin.ID != adminRole.ID || reconciledAdmin.Name != "管理员" || reconciledAdmin.Description == "" {
		t.Fatalf("reconciled admin role = %#v", reconciledAdmin)
	}

	var customGrantCount int64
	if err := db.Model(&database.RolePermission{}).Where("tenant_id = ? AND role_id = ?", tenant.ID, customRole.ID).Count(&customGrantCount).Error; err != nil {
		t.Fatalf("count custom role grants: %v", err)
	}
	if customGrantCount != 1 {
		t.Fatalf("custom role grant count = %d, want 1", customGrantCount)
	}
	var sellerProviderGrantCount int64
	if err := db.Model(&database.RolePermission{}).Where("tenant_id = ? AND role_id = ? AND permission_id = ?", tenant.ID, sellerRole.ID, providerRead.ID).Count(&sellerProviderGrantCount).Error; err != nil {
		t.Fatalf("count legacy seller provider grant: %v", err)
	}
	if sellerProviderGrantCount != 0 {
		t.Fatalf("legacy seller provider grant count = %d, want 0", sellerProviderGrantCount)
	}
}

func TestReconcileBuiltInRolesIsIdempotent(t *testing.T) {
	db := openBootstrapTestDB(t)
	now := time.Now().UTC()
	tenant := database.Tenant{ID: "tenant-idempotent", Name: "Idempotent tenant", Status: TenantStatusActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	if err := ReconcileBuiltInRoles(context.Background(), db); err != nil {
		t.Fatalf("first ReconcileBuiltInRoles returned error: %v", err)
	}
	first := bootstrapRowCounts(t, db)

	if err := ReconcileBuiltInRoles(context.Background(), db); err != nil {
		t.Fatalf("second ReconcileBuiltInRoles returned error: %v", err)
	}
	second := bootstrapRowCounts(t, db)

	if second != first {
		t.Fatalf("row counts after second reconciliation = %+v, want %+v", second, first)
	}
}

func TestReconcileBuiltInRolesKeepsTenantGrantsIsolated(t *testing.T) {
	db := openBootstrapTestDB(t)
	now := time.Now().UTC()
	tenants := []database.Tenant{
		{ID: "tenant-a", Name: "Tenant A", Status: TenantStatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "tenant-b", Name: "Tenant B", Status: TenantStatusActive, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&tenants).Error; err != nil {
		t.Fatalf("create tenants: %v", err)
	}

	if err := ReconcileBuiltInRoles(context.Background(), db); err != nil {
		t.Fatalf("ReconcileBuiltInRoles returned error: %v", err)
	}

	var crossTenantGrantCount int64
	if err := db.Table("role_permissions").
		Joins("JOIN roles ON roles.id = role_permissions.role_id").
		Where("roles.tenant_id <> role_permissions.tenant_id").
		Count(&crossTenantGrantCount).Error; err != nil {
		t.Fatalf("count cross-tenant grants: %v", err)
	}
	if crossTenantGrantCount != 0 {
		t.Fatalf("cross-tenant grant count = %d, want 0", crossTenantGrantCount)
	}

	for _, tenant := range tenants {
		assertRoleHasPermissions(t, db, tenant.ID, "seller", []string{"asset:read", "asset:upload", "asset:update", "asset:delete", "asset:download"})
		assertRoleHasPermissions(t, db, tenant.ID, "viewer", []string{"asset:read", "asset:download"})
	}
}

type bootstrapCounts struct {
	permissions     int64
	roles           int64
	rolePermissions int64
}

func bootstrapRowCounts(t *testing.T, db *gorm.DB) bootstrapCounts {
	t.Helper()

	var counts bootstrapCounts
	if err := db.Model(&database.Permission{}).Count(&counts.permissions).Error; err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if err := db.Model(&database.Role{}).Count(&counts.roles).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if err := db.Model(&database.RolePermission{}).Count(&counts.rolePermissions).Error; err != nil {
		t.Fatalf("count role permissions: %v", err)
	}
	return counts
}

func assertRoleHasPermissions(t *testing.T, db *gorm.DB, tenantID string, roleCode string, permissionCodes []string) {
	t.Helper()

	var role database.Role
	if err := db.Where("tenant_id = ? AND code = ?", tenantID, roleCode).First(&role).Error; err != nil {
		t.Fatalf("load role %s for tenant %s: %v", roleCode, tenantID, err)
	}

	for _, permissionCode := range permissionCodes {
		var grantCount int64
		if err := db.Table("role_permissions").
			Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
			Where("role_permissions.tenant_id = ? AND role_permissions.role_id = ? AND permissions.code = ?", tenantID, role.ID, permissionCode).
			Count(&grantCount).Error; err != nil {
			t.Fatalf("count %s grant for role %s and tenant %s: %v", permissionCode, roleCode, tenantID, err)
		}
		if grantCount != 1 {
			t.Fatalf("%s grant count for role %s and tenant %s = %d, want 1", permissionCode, roleCode, tenantID, grantCount)
		}
	}
}

func openBootstrapTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsnName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := "file:" + dsnName + "_" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared&_loc=auto"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access sqlite test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	for _, statement := range bootstrapTestSchema {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("migrate sqlite test database: %v", err)
		}
	}
	return db
}

var bootstrapTestSchema = []string{
	`CREATE TABLE tenants (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE permissions (
		id TEXT PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		description TEXT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE roles (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		UNIQUE (tenant_id, code)
	)`,
	`CREATE TABLE role_permissions (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		permission_id TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		UNIQUE (tenant_id, role_id, permission_id)
	)`,
	`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		email TEXT NOT NULL,
		display_name TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		status TEXT NOT NULL,
		session_version INTEGER NOT NULL DEFAULT 1,
		last_login_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		UNIQUE (tenant_id, email)
	)`,
	`CREATE TABLE user_roles (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		UNIQUE (tenant_id, user_id, role_id)
	)`,
	`CREATE TABLE operation_logs (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		actor_user_id TEXT NULL,
		action TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		ip TEXT NOT NULL,
		user_agent TEXT NOT NULL,
		metadata_json TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL
	)`,
}
