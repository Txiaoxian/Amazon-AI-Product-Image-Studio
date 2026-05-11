package database

import "time"

// Tenant is the root tenant record. It intentionally has no tenant_id because
// tenant_id identifies records that belong to one of these tenant roots.
type Tenant struct {
	ID        string    `gorm:"type:varchar(36);primaryKey"`
	Name      string    `gorm:"type:varchar(255);not null"`
	Status    string    `gorm:"type:varchar(32);not null;index"`
	CreatedAt time.Time `gorm:"type:datetime(3);not null"`
	UpdatedAt time.Time `gorm:"type:datetime(3);not null"`
}

func (Tenant) TableName() string {
	return "tenants"
}

type User struct {
	ID           string     `gorm:"type:varchar(36);primaryKey;uniqueIndex:uk_users_tenant_id,priority:2"`
	TenantID     string     `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_users_tenant_email,priority:1;uniqueIndex:uk_users_tenant_id,priority:1"`
	Email        string     `gorm:"type:varchar(255);not null;uniqueIndex:uk_users_tenant_email,priority:2"`
	DisplayName  string     `gorm:"type:varchar(255);not null"`
	PasswordHash string     `gorm:"type:varchar(255);not null"`
	Status       string     `gorm:"type:varchar(32);not null;index"`
	LastLoginAt  *time.Time `gorm:"type:datetime(3)"`
	CreatedAt    time.Time  `gorm:"type:datetime(3);not null"`
	UpdatedAt    time.Time  `gorm:"type:datetime(3);not null"`
}

func (User) TableName() string {
	return "users"
}

type Role struct {
	ID          string    `gorm:"type:varchar(36);primaryKey;uniqueIndex:uk_roles_tenant_id,priority:2"`
	TenantID    string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_roles_tenant_code,priority:1;uniqueIndex:uk_roles_tenant_id,priority:1"`
	Code        string    `gorm:"type:varchar(128);not null;uniqueIndex:uk_roles_tenant_code,priority:2"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	Status      string    `gorm:"type:varchar(32);not null;index"`
	CreatedAt   time.Time `gorm:"type:datetime(3);not null"`
	UpdatedAt   time.Time `gorm:"type:datetime(3);not null"`
}

func (Role) TableName() string {
	return "roles"
}

// Permission is a system-level dictionary. It intentionally has no tenant_id
// because permission codes are global; tenant-scoped grants are stored in
// role_permissions with an explicit tenant_id.
type Permission struct {
	ID          string    `gorm:"type:varchar(36);primaryKey"`
	Code        string    `gorm:"type:varchar(128);not null;uniqueIndex"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"type:datetime(3);not null"`
	UpdatedAt   time.Time `gorm:"type:datetime(3);not null"`
}

func (Permission) TableName() string {
	return "permissions"
}

type UserRole struct {
	ID        string    `gorm:"type:varchar(36);primaryKey"`
	TenantID  string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_user_roles_tenant_user_role,priority:1"`
	UserID    string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_user_roles_tenant_user_role,priority:2"`
	RoleID    string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_user_roles_tenant_user_role,priority:3"`
	CreatedAt time.Time `gorm:"type:datetime(3);not null"`
}

func (UserRole) TableName() string {
	return "user_roles"
}

type RolePermission struct {
	ID           string    `gorm:"type:varchar(36);primaryKey"`
	TenantID     string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_role_permissions_tenant_role_permission,priority:1"`
	RoleID       string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_role_permissions_tenant_role_permission,priority:2"`
	PermissionID string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_role_permissions_tenant_role_permission,priority:3"`
	CreatedAt    time.Time `gorm:"type:datetime(3);not null"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}

type OperationLog struct {
	ID           string    `gorm:"type:varchar(36);primaryKey"`
	TenantID     string    `gorm:"type:varchar(36);not null;index;index:idx_operation_logs_tenant_created,priority:1"`
	ActorUserID  *string   `gorm:"type:varchar(36);index"`
	Action       string    `gorm:"type:varchar(128);not null;index"`
	ResourceType string    `gorm:"type:varchar(128);not null;index"`
	ResourceID   string    `gorm:"type:varchar(64);not null;index"`
	IP           string    `gorm:"type:varchar(45);not null"`
	UserAgent    string    `gorm:"type:varchar(512);not null"`
	MetadataJSON string    `gorm:"type:json"`
	CreatedAt    time.Time `gorm:"type:datetime(3);not null;index:idx_operation_logs_tenant_created,priority:2"`
}

func (OperationLog) TableName() string {
	return "operation_logs"
}
