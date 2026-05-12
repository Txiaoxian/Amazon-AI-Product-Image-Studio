package database

import (
	"time"

	"gorm.io/gorm"
)

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

type Project struct {
	ID        string         `gorm:"type:varchar(36);primaryKey;uniqueIndex:uk_projects_tenant_id,priority:2"`
	TenantID  string         `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_projects_tenant_id,priority:1;index:idx_projects_tenant_status_created,priority:1;index:idx_projects_tenant_asin,priority:1;index:idx_projects_tenant_deleted,priority:1"`
	Name      string         `gorm:"type:varchar(255);not null"`
	Brand     string         `gorm:"type:varchar(255)"`
	ASIN      string         `gorm:"type:varchar(32);index:idx_projects_tenant_asin,priority:2"`
	Site      string         `gorm:"type:varchar(64)"`
	Notes     string         `gorm:"type:text"`
	Status    string         `gorm:"type:varchar(32);not null;index:idx_projects_tenant_status_created,priority:2"`
	CreatedBy string         `gorm:"type:varchar(36);not null;index"`
	CreatedAt time.Time      `gorm:"type:datetime(3);not null;index:idx_projects_tenant_status_created,priority:3"`
	UpdatedAt time.Time      `gorm:"type:datetime(3);not null"`
	DeletedAt gorm.DeletedAt `gorm:"type:datetime(3);index;index:idx_projects_tenant_deleted,priority:2"`
}

func (Project) TableName() string {
	return "projects"
}

type ProjectMember struct {
	ID        string    `gorm:"type:varchar(36);primaryKey"`
	TenantID  string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_project_members_tenant_project_user,priority:1;index:idx_project_members_tenant_project,priority:1;index:idx_project_members_tenant_user,priority:1"`
	ProjectID string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_project_members_tenant_project_user,priority:2;index:idx_project_members_tenant_project,priority:2"`
	UserID    string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_project_members_tenant_project_user,priority:3;index:idx_project_members_tenant_user,priority:2"`
	Role      string    `gorm:"type:varchar(32);not null;index"`
	CreatedAt time.Time `gorm:"type:datetime(3);not null"`
	UpdatedAt time.Time `gorm:"type:datetime(3);not null"`
}

func (ProjectMember) TableName() string {
	return "project_members"
}

type ImageAsset struct {
	ID                 string         `gorm:"type:varchar(36);primaryKey;uniqueIndex:uk_image_assets_tenant_id,priority:2"`
	TenantID           string         `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_image_assets_tenant_id,priority:1;index:idx_image_assets_tenant_project_created,priority:1;index:idx_image_assets_tenant_project_kind,priority:1;index:idx_image_assets_tenant_favorite,priority:1;index:idx_image_assets_tenant_deleted,priority:1;index:idx_image_assets_tenant_sha256,priority:1"`
	ProjectID          string         `gorm:"type:varchar(36);not null;index;index:idx_image_assets_tenant_project_created,priority:2;index:idx_image_assets_tenant_project_kind,priority:2"`
	Kind               string         `gorm:"type:varchar(32);not null;index:idx_image_assets_tenant_project_kind,priority:3"`
	Category           string         `gorm:"type:varchar(128);not null"`
	Filename           string         `gorm:"type:varchar(255);not null"`
	ObjectKey          string         `gorm:"type:varchar(512);not null;uniqueIndex:uk_image_assets_object_key"`
	ThumbnailObjectKey *string        `gorm:"type:varchar(512)"`
	MimeType           string         `gorm:"type:varchar(128);not null"`
	SizeBytes          int64          `gorm:"type:bigint unsigned;not null"`
	Width              int            `gorm:"type:int unsigned;not null"`
	Height             int            `gorm:"type:int unsigned;not null"`
	SHA256             string         `gorm:"type:char(64);not null;index:idx_image_assets_tenant_sha256,priority:2"`
	IsFavorite         bool           `gorm:"type:boolean;not null;index:idx_image_assets_tenant_favorite,priority:2"`
	SourceTaskID       *string        `gorm:"type:varchar(36)"`
	CreatedBy          string         `gorm:"type:varchar(36);not null;index"`
	CreatedAt          time.Time      `gorm:"type:datetime(3);not null;index:idx_image_assets_tenant_project_created,priority:3"`
	UpdatedAt          time.Time      `gorm:"type:datetime(3);not null"`
	DeletedAt          gorm.DeletedAt `gorm:"type:datetime(3);index;index:idx_image_assets_tenant_deleted,priority:2"`
}

func (ImageAsset) TableName() string {
	return "image_assets"
}
