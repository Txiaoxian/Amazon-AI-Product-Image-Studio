package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"gorm.io/gorm"
)

func TestProvisionTenantCreatesBuiltInRolesGrantsAndInitialAdmin(t *testing.T) {
	db := openBootstrapTestDB(t)
	seedInitializedPlatform(t, db)
	input := validTenantProvisioningInput()

	result, err := ProvisionTenant(context.Background(), db, input)
	if err != nil {
		t.Fatalf("ProvisionTenant returned error: %v", err)
	}
	if result.TenantID == "" {
		t.Fatal("ProvisionTenant tenant ID is empty")
	}

	var tenant database.Tenant
	if err := db.First(&tenant, "id = ?", result.TenantID).Error; err != nil {
		t.Fatalf("load tenant: %v", err)
	}
	if tenant.Name != input.TenantName || tenant.Status != TenantStatusActive {
		t.Fatalf("tenant = %+v, want name %q and active status", tenant, input.TenantName)
	}

	assertRoleHasPermissions(t, db, result.TenantID, "admin", builtInPermissionCodes)
	assertRoleHasPermissions(t, db, result.TenantID, "seller", builtInRoles[1].Permissions)
	assertRoleHasPermissions(t, db, result.TenantID, "viewer", builtInRoles[2].Permissions)

	var admin database.User
	if err := db.First(&admin, "tenant_id = ? AND email = ?", result.TenantID, input.AdminEmail).Error; err != nil {
		t.Fatalf("load initial admin: %v", err)
	}
	if admin.DisplayName != input.AdminDisplayName || admin.Status != UserStatusActive {
		t.Fatal("initial admin safe fields do not match provisioning input")
	}
	if !CheckPassword(admin.PasswordHash, input.AdminPassword) {
		t.Fatal("initial admin password hash does not match input")
	}

	var adminRoleCount int64
	if err := db.Table("user_roles").
		Joins("JOIN roles ON roles.tenant_id = user_roles.tenant_id AND roles.id = user_roles.role_id").
		Where("user_roles.tenant_id = ? AND user_roles.user_id = ? AND roles.code = ?", result.TenantID, admin.ID, "admin").
		Count(&adminRoleCount).Error; err != nil {
		t.Fatalf("count initial admin role: %v", err)
	}
	if adminRoleCount != 1 {
		t.Fatalf("initial admin role count = %d, want 1", adminRoleCount)
	}

	assertProvisioningRowCount(t, db, &database.OperationLog{}, 1)
}

func TestProvisionTenantRejectsInvalidFieldsAndWeakPasswordWithoutWrites(t *testing.T) {
	tests := []struct {
		name  string
		input TenantProvisioningInput
	}{
		{name: "missing tenant name", input: TenantProvisioningInput{AdminEmail: "admin@example.com", AdminDisplayName: "Admin", AdminPassword: "strong-password"}},
		{name: "invalid email", input: TenantProvisioningInput{TenantName: "Tenant", AdminEmail: "not-an-email", AdminDisplayName: "Admin", AdminPassword: "strong-password"}},
		{name: "missing display name", input: TenantProvisioningInput{TenantName: "Tenant", AdminEmail: "admin@example.com", AdminPassword: "strong-password"}},
		{name: "weak password", input: TenantProvisioningInput{TenantName: "Tenant", AdminEmail: "admin@example.com", AdminDisplayName: "Admin", AdminPassword: "weak"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openBootstrapTestDB(t)

			if _, err := ProvisionTenant(context.Background(), db, tt.input); err == nil {
				t.Fatal("ProvisionTenant returned nil error")
			}

			assertProvisioningDatabaseEmpty(t, db)
		})
	}
}

func TestProvisionTenantRollsBackAllWritesAfterInjectedFailure(t *testing.T) {
	db := openBootstrapTestDB(t)
	seedInitializedPlatform(t, db)
	before := provisioningRowCounts(t, db)
	injected := errors.New("injected provisioning failure")

	_, err := provisionTenant(context.Background(), db, validTenantProvisioningInput(), func() error {
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("provisionTenant error = %v, want %v", err, injected)
	}

	after := provisioningRowCounts(t, db)
	if after != before {
		t.Fatalf("row counts after rollback = %+v, want %+v", after, before)
	}
}

func TestProvisionTenantAllowsSameAdminEmailInDifferentNewTenants(t *testing.T) {
	db := openBootstrapTestDB(t)
	seedInitializedPlatform(t, db)
	input := validTenantProvisioningInput()
	firstPassword := input.AdminPassword

	first, err := ProvisionTenant(context.Background(), db, input)
	if err != nil {
		t.Fatalf("first ProvisionTenant returned error: %v", err)
	}
	input.TenantName = "Second tenant"
	input.AdminPassword = "second-strong-password"
	second, err := ProvisionTenant(context.Background(), db, input)
	if err != nil {
		t.Fatalf("second ProvisionTenant returned error: %v", err)
	}
	if first.TenantID == second.TenantID {
		t.Fatalf("tenant IDs are equal: %q", first.TenantID)
	}

	var count int64
	if err := db.Model(&database.User{}).Where("email = ?", input.AdminEmail).Count(&count).Error; err != nil {
		t.Fatalf("count users by email: %v", err)
	}
	if count != 2 {
		t.Fatalf("users with shared email = %d, want 2", count)
	}

	service := NewService(db, config.Config{}, nil)
	firstSession, err := service.login(context.Background(), first.TenantID, input.AdminEmail, firstPassword, "", "", "", "")
	if err != nil {
		t.Fatalf("login first provisioned tenant: %v", err)
	}
	secondSession, err := service.login(context.Background(), second.TenantID, input.AdminEmail, input.AdminPassword, "", "", "", "")
	if err != nil {
		t.Fatalf("login second provisioned tenant: %v", err)
	}
	if firstSession.Tenant.ID != first.TenantID || secondSession.Tenant.ID != second.TenantID {
		t.Fatal("provisioned tenant logins did not stay tenant scoped")
	}
}

func TestProvisionTenantRequiresInitializedPlatformWithoutWrites(t *testing.T) {
	db := openBootstrapTestDB(t)

	_, err := ProvisionTenant(context.Background(), db, validTenantProvisioningInput())
	if !errors.Is(err, ErrPlatformNotInitialized) {
		t.Fatalf("ProvisionTenant error = %v, want %v", err, ErrPlatformNotInitialized)
	}

	assertProvisioningDatabaseEmpty(t, db)
}

func validTenantProvisioningInput() TenantProvisioningInput {
	return TenantProvisioningInput{
		TenantName:       "Provisioned tenant",
		AdminEmail:       "admin@example.com",
		AdminDisplayName: "Tenant Admin",
		AdminPassword:    "strong-password",
	}
}

func assertProvisioningDatabaseEmpty(t *testing.T, db *gorm.DB) {
	t.Helper()

	if counts := provisioningRowCounts(t, db); counts != (provisioningCounts{}) {
		t.Fatalf("provisioning database row counts = %+v, want empty", counts)
	}
}

func assertProvisioningRowCount(t *testing.T, db *gorm.DB, model any, want int64) {
	t.Helper()

	var count int64
	if err := db.Model(model).Count(&count).Error; err != nil {
		t.Fatalf("count %T rows: %v", model, err)
	}
	if count != want {
		t.Fatalf("%T row count = %d, want %d", model, count, want)
	}
}

type provisioningCounts struct {
	tenants         int64
	permissions     int64
	roles           int64
	rolePermissions int64
	users           int64
	userRoles       int64
	operationLogs   int64
}

func provisioningRowCounts(t *testing.T, db *gorm.DB) provisioningCounts {
	t.Helper()

	var counts provisioningCounts
	countProvisioningRows(t, db, &database.Tenant{}, &counts.tenants)
	countProvisioningRows(t, db, &database.Permission{}, &counts.permissions)
	countProvisioningRows(t, db, &database.Role{}, &counts.roles)
	countProvisioningRows(t, db, &database.RolePermission{}, &counts.rolePermissions)
	countProvisioningRows(t, db, &database.User{}, &counts.users)
	countProvisioningRows(t, db, &database.UserRole{}, &counts.userRoles)
	countProvisioningRows(t, db, &database.OperationLog{}, &counts.operationLogs)
	return counts
}

func countProvisioningRows(t *testing.T, db *gorm.DB, model any, count *int64) {
	t.Helper()

	if err := db.Model(model).Count(count).Error; err != nil {
		t.Fatalf("count %T rows: %v", model, err)
	}
}

func seedInitializedPlatform(t *testing.T, db *gorm.DB) {
	t.Helper()

	now := time.Now().UTC()
	tenant := database.Tenant{
		ID:        "tenant-initial",
		Name:      "Initial tenant",
		Status:    TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	user := database.User{
		ID:           "user-initial-admin",
		TenantID:     tenant.ID,
		Email:        "initial-admin@example.com",
		DisplayName:  "Initial Admin",
		PasswordHash: "existing-password-hash",
		Status:       UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create initial tenant: %v", err)
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create initial admin: %v", err)
	}
}
