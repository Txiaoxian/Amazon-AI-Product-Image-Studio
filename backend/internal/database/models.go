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

type AIProvider struct {
	ID               string         `gorm:"type:varchar(36);primaryKey;uniqueIndex:uk_ai_providers_tenant_id,priority:2"`
	TenantID         string         `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_ai_providers_tenant_id,priority:1;index:idx_ai_providers_tenant_type,priority:1;index:idx_ai_providers_tenant_status,priority:1;index:idx_ai_providers_tenant_deleted,priority:1"`
	Type             string         `gorm:"type:varchar(32);not null;index:idx_ai_providers_tenant_type,priority:2"`
	Name             string         `gorm:"type:varchar(255);not null"`
	BaseURL          string         `gorm:"type:varchar(512);not null"`
	EncryptedAPIKey  string         `gorm:"type:text;not null"`
	APIKeyHint       string         `gorm:"type:varchar(32);not null"`
	APIKeyUpdatedAt  *time.Time     `gorm:"type:datetime(3)"`
	Status           string         `gorm:"type:varchar(32);not null;index:idx_ai_providers_tenant_status,priority:2"`
	TimeoutSeconds   int            `gorm:"type:int unsigned;not null"`
	ConcurrencyLimit int            `gorm:"type:int unsigned;not null"`
	LastTestStatus   string         `gorm:"type:varchar(32)"`
	LastTestedAt     *time.Time     `gorm:"type:datetime(3)"`
	LastTestError    string         `gorm:"type:varchar(255)"`
	CreatedBy        string         `gorm:"type:varchar(36);not null;index"`
	CreatedAt        time.Time      `gorm:"type:datetime(3);not null"`
	UpdatedAt        time.Time      `gorm:"type:datetime(3);not null"`
	DeletedAt        gorm.DeletedAt `gorm:"type:datetime(3);index;index:idx_ai_providers_tenant_deleted,priority:2"`
}

func (AIProvider) TableName() string {
	return "ai_providers"
}

type AIModel struct {
	ID                         string         `gorm:"type:varchar(36);primaryKey;uniqueIndex:uk_ai_models_tenant_id,priority:2"`
	TenantID                   string         `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_ai_models_tenant_id,priority:1;index:idx_ai_models_tenant_provider,priority:1;index:idx_ai_models_tenant_status,priority:1;index:idx_ai_models_tenant_deleted,priority:1;index:idx_ai_models_tenant_provider_model,priority:1;index:idx_ai_models_tenant_generate,priority:1;index:idx_ai_models_tenant_edit,priority:1"`
	ProviderID                 string         `gorm:"type:varchar(36);not null;index:idx_ai_models_tenant_provider,priority:2;index:idx_ai_models_tenant_provider_model,priority:2"`
	ModelName                  string         `gorm:"type:varchar(255);not null;index:idx_ai_models_tenant_provider_model,priority:3"`
	DisplayName                string         `gorm:"type:varchar(255);not null"`
	SupportsGenerate           bool           `gorm:"type:boolean;not null;index:idx_ai_models_tenant_generate,priority:2"`
	SupportsEdit               bool           `gorm:"type:boolean;not null;index:idx_ai_models_tenant_edit,priority:2"`
	SupportsMultiReference     bool           `gorm:"type:boolean;not null"`
	SupportsN                  bool           `gorm:"type:boolean;not null"`
	MaxOutputCount             int            `gorm:"type:int unsigned;not null"`
	SupportedSizesJSON         string         `gorm:"type:json;not null"`
	SupportedQualitiesJSON     string         `gorm:"type:json;not null"`
	SupportedOutputFormatsJSON string         `gorm:"type:json;not null"`
	PricingJSON                string         `gorm:"type:json;not null"`
	Status                     string         `gorm:"type:varchar(32);not null;index:idx_ai_models_tenant_status,priority:2"`
	CreatedBy                  string         `gorm:"type:varchar(36);not null;index"`
	CreatedAt                  time.Time      `gorm:"type:datetime(3);not null"`
	UpdatedAt                  time.Time      `gorm:"type:datetime(3);not null"`
	DeletedAt                  gorm.DeletedAt `gorm:"type:datetime(3);index;index:idx_ai_models_tenant_deleted,priority:2"`
}

func (AIModel) TableName() string {
	return "ai_models"
}

type GenerationTask struct {
	ID                string     `gorm:"type:varchar(36);primaryKey;uniqueIndex:uk_generation_tasks_tenant_id,priority:2"`
	TenantID          string     `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_generation_tasks_tenant_id,priority:1;index:idx_generation_tasks_tenant_project_created,priority:1;index:idx_generation_tasks_tenant_status,priority:1;index:idx_generation_tasks_tenant_created_by,priority:1;index:idx_generation_tasks_tenant_provider_status,priority:1;index:idx_generation_tasks_tenant_model_status,priority:1;index:idx_generation_tasks_tenant_timeout,priority:1"`
	ProjectID         string     `gorm:"type:varchar(36);not null;index;index:idx_generation_tasks_tenant_project_created,priority:2"`
	Type              string     `gorm:"type:varchar(32);not null;index"`
	ProviderID        string     `gorm:"type:varchar(36);not null;index:idx_generation_tasks_tenant_provider_status,priority:2"`
	ModelID           string     `gorm:"type:varchar(36);not null;index:idx_generation_tasks_tenant_model_status,priority:2"`
	Status            string     `gorm:"type:varchar(32);not null;index:idx_generation_tasks_tenant_status,priority:2;index:idx_generation_tasks_tenant_provider_status,priority:3;index:idx_generation_tasks_tenant_model_status,priority:3"`
	Prompt            string     `gorm:"type:text;not null"`
	ImageType         string     `gorm:"type:varchar(64)"`
	ParamsJSON        string     `gorm:"type:json;not null"`
	InputAssetIDsJSON string     `gorm:"type:json;not null"`
	Attempt           int        `gorm:"type:int unsigned;not null"`
	MaxAttempts       int        `gorm:"type:int unsigned;not null"`
	QueuedAt          *time.Time `gorm:"type:datetime(3)"`
	StartedAt         *time.Time `gorm:"type:datetime(3)"`
	FinishedAt        *time.Time `gorm:"type:datetime(3)"`
	TimeoutAt         *time.Time `gorm:"type:datetime(3);index:idx_generation_tasks_tenant_timeout,priority:2"`
	CreatedBy         string     `gorm:"type:varchar(36);not null;index;index:idx_generation_tasks_tenant_created_by,priority:2"`
	ErrorCode         string     `gorm:"type:varchar(128)"`
	ErrorMessage      string     `gorm:"type:varchar(512)"`
	CreatedAt         time.Time  `gorm:"type:datetime(3);not null;index:idx_generation_tasks_tenant_project_created,priority:3;index:idx_generation_tasks_tenant_created_by,priority:3"`
	UpdatedAt         time.Time  `gorm:"type:datetime(3);not null"`
}

func (GenerationTask) TableName() string {
	return "generation_tasks"
}

type TaskEvent struct {
	Sequence         uint64    `gorm:"column:sequence;primaryKey;autoIncrement;index:idx_task_events_tenant_task_sequence,priority:3;index:idx_task_events_tenant_project_sequence,priority:3;index:idx_task_events_tenant_sequence,priority:2"`
	ID               string    `gorm:"type:varchar(64);not null;uniqueIndex:uk_task_events_id"`
	TenantID         string    `gorm:"type:varchar(36);not null;index;index:idx_task_events_tenant_task_sequence,priority:1;index:idx_task_events_tenant_project_sequence,priority:1;index:idx_task_events_tenant_sequence,priority:1"`
	TaskID           string    `gorm:"type:varchar(36);not null;index:idx_task_events_tenant_task_sequence,priority:2"`
	ProjectID        string    `gorm:"type:varchar(36);not null;index:idx_task_events_tenant_project_sequence,priority:2"`
	EventType        string    `gorm:"type:varchar(64);not null;index"`
	EventPayloadJSON string    `gorm:"type:json;not null"`
	CreatedAt        time.Time `gorm:"type:datetime(3);not null"`
}

func (TaskEvent) TableName() string {
	return "task_events"
}

type TaskOutput struct {
	ID          string    `gorm:"type:varchar(36);primaryKey;uniqueIndex:uk_task_outputs_tenant_id,priority:2"`
	TenantID    string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_task_outputs_tenant_id,priority:1;uniqueIndex:uk_task_outputs_tenant_task_index,priority:1;index:idx_task_outputs_tenant_task,priority:1;index:idx_task_outputs_tenant_asset,priority:1"`
	TaskID      string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_task_outputs_tenant_task_index,priority:2;index:idx_task_outputs_tenant_task,priority:2"`
	AssetID     string    `gorm:"type:varchar(36);not null;index:idx_task_outputs_tenant_asset,priority:2"`
	OutputIndex int       `gorm:"type:int unsigned;not null;uniqueIndex:uk_task_outputs_tenant_task_index,priority:3"`
	CreatedAt   time.Time `gorm:"type:datetime(3);not null"`
}

func (TaskOutput) TableName() string {
	return "task_outputs"
}

type APICallLog struct {
	ID                   string    `gorm:"type:varchar(36);primaryKey"`
	TenantID             string    `gorm:"type:varchar(36);not null;index;index:idx_api_call_logs_tenant_task,priority:1;index:idx_api_call_logs_tenant_provider,priority:1;index:idx_api_call_logs_tenant_created,priority:1"`
	TaskID               string    `gorm:"type:varchar(36);not null;index:idx_api_call_logs_tenant_task,priority:2"`
	ProviderID           string    `gorm:"type:varchar(36);not null;index:idx_api_call_logs_tenant_provider,priority:2"`
	ModelID              string    `gorm:"type:varchar(36);not null;index"`
	Status               string    `gorm:"type:varchar(32);not null;index"`
	DurationMs           int64     `gorm:"type:bigint unsigned;not null"`
	RequestID            string    `gorm:"type:varchar(128)"`
	HTTPStatus           *int      `gorm:"type:int unsigned"`
	ErrorCode            string    `gorm:"type:varchar(128)"`
	ErrorMessage         string    `gorm:"type:varchar(512)"`
	RedactedRequestJSON  string    `gorm:"type:json"`
	RedactedResponseJSON string    `gorm:"type:json"`
	CreatedAt            time.Time `gorm:"type:datetime(3);not null;index:idx_api_call_logs_tenant_created,priority:2"`
}

func (APICallLog) TableName() string {
	return "api_call_logs"
}

type UsageRecord struct {
	ID            string    `gorm:"type:varchar(36);primaryKey"`
	TenantID      string    `gorm:"type:varchar(36);not null;index;index:idx_usage_records_tenant_task,priority:1;index:idx_usage_records_tenant_project_created,priority:1;index:idx_usage_records_tenant_user_created,priority:1"`
	TaskID        string    `gorm:"type:varchar(36);not null;index:idx_usage_records_tenant_task,priority:2"`
	UserID        string    `gorm:"type:varchar(36);not null;index:idx_usage_records_tenant_user_created,priority:2"`
	ProjectID     string    `gorm:"type:varchar(36);not null;index:idx_usage_records_tenant_project_created,priority:2"`
	ProviderID    string    `gorm:"type:varchar(36);not null;index"`
	ModelID       string    `gorm:"type:varchar(36);not null;index"`
	InputTokens   int64     `gorm:"type:bigint unsigned;not null"`
	OutputTokens  int64     `gorm:"type:bigint unsigned;not null"`
	ImageCount    int       `gorm:"type:int unsigned;not null"`
	EstimatedCost string    `gorm:"type:decimal(18,8);not null"`
	Currency      string    `gorm:"type:char(3);not null"`
	RawUsageJSON  string    `gorm:"type:json"`
	CreatedAt     time.Time `gorm:"type:datetime(3);not null;index:idx_usage_records_tenant_project_created,priority:3;index:idx_usage_records_tenant_user_created,priority:3"`
}

func (UsageRecord) TableName() string {
	return "usage_records"
}
