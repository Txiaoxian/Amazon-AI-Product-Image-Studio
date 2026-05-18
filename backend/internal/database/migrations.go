package database

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type Migration struct {
	ID         string
	Name       string
	Statements []string
}

func RunMigrations(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}

	db = db.WithContext(ctx)
	if err := ensureSchemaMigrations(db); err != nil {
		return err
	}

	for _, migration := range migrations {
		applied, err := migrationApplied(db, migration.ID)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := runMigration(db, migration); err != nil {
			return err
		}
	}

	return nil
}

func ensureSchemaMigrations(db *gorm.DB) error {
	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  id VARCHAR(128) NOT NULL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Backend migration history'
`).Error; err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	return nil
}

func migrationApplied(db *gorm.DB, id string) (bool, error) {
	var count int64
	if err := db.Table("schema_migrations").Where("id = ?", id).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check migration %s: %w", id, err)
	}

	return count > 0, nil
}

func runMigration(db *gorm.DB, migration Migration) error {
	if strings.TrimSpace(migration.ID) == "" {
		return fmt.Errorf("migration id is required")
	}
	if strings.TrimSpace(migration.Name) == "" {
		return fmt.Errorf("migration name is required")
	}

	for _, statement := range migration.Statements {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.ID, err)
		}
	}

	if err := db.Table("schema_migrations").Create(map[string]any{
		"id":   migration.ID,
		"name": migration.Name,
	}).Error; err != nil {
		return fmt.Errorf("record migration %s: %w", migration.ID, err)
	}

	return nil
}

var migrations = []Migration{
	{
		ID:   "202605110001_identity_and_audit_base",
		Name: "create tenant identity rbac and operation log tables",
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS tenants (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_tenants_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`,
			`
CREATE TABLE IF NOT EXISTS users (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  email VARCHAR(255) NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL,
  last_login_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_users_tenant_email (tenant_id, email),
  KEY idx_users_tenant_id (tenant_id),
  UNIQUE KEY uk_users_tenant_id (tenant_id, id),
  KEY idx_users_status (status),
  CONSTRAINT fk_users_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`,
			`
CREATE TABLE IF NOT EXISTS roles (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  code VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT NULL,
  status VARCHAR(32) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_roles_tenant_code (tenant_id, code),
  KEY idx_roles_tenant_id (tenant_id),
  UNIQUE KEY uk_roles_tenant_id (tenant_id, id),
  KEY idx_roles_status (status),
  CONSTRAINT fk_roles_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`,
			`
CREATE TABLE IF NOT EXISTS permissions (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  code VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_permissions_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='System-level permission dictionary; tenant-scoped grants are stored in role_permissions'
`,
			`
CREATE TABLE IF NOT EXISTS user_roles (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  user_id VARCHAR(36) NOT NULL,
  role_id VARCHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_user_roles_tenant_user_role (tenant_id, user_id, role_id),
  KEY idx_user_roles_tenant_id (tenant_id),
  KEY idx_user_roles_user_id (user_id),
  KEY idx_user_roles_role_id (role_id),
  CONSTRAINT fk_user_roles_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_user_roles_user FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id),
  CONSTRAINT fk_user_roles_role FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`,
			`
CREATE TABLE IF NOT EXISTS role_permissions (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  role_id VARCHAR(36) NOT NULL,
  permission_id VARCHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_role_permissions_tenant_role_permission (tenant_id, role_id, permission_id),
  KEY idx_role_permissions_tenant_id (tenant_id),
  KEY idx_role_permissions_role_id (role_id),
  KEY idx_role_permissions_permission_id (permission_id),
  CONSTRAINT fk_role_permissions_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_role_permissions_role FOREIGN KEY (tenant_id, role_id) REFERENCES roles(tenant_id, id),
  CONSTRAINT fk_role_permissions_permission FOREIGN KEY (permission_id) REFERENCES permissions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`,
			`
CREATE TABLE IF NOT EXISTS operation_logs (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  actor_user_id VARCHAR(36) NULL,
  action VARCHAR(128) NOT NULL,
  resource_type VARCHAR(128) NOT NULL,
  resource_id VARCHAR(64) NOT NULL,
  ip VARCHAR(45) NOT NULL,
  user_agent VARCHAR(512) NOT NULL,
  metadata_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_operation_logs_tenant_id (tenant_id),
  KEY idx_operation_logs_tenant_created (tenant_id, created_at),
  KEY idx_operation_logs_actor_user_id (actor_user_id),
  KEY idx_operation_logs_action (action),
  KEY idx_operation_logs_resource_type (resource_type),
  KEY idx_operation_logs_resource_id (resource_id),
  CONSTRAINT fk_operation_logs_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_operation_logs_actor_user FOREIGN KEY (tenant_id, actor_user_id) REFERENCES users(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Audit metadata must be redacted before insert'
`,
		},
	},
	{
		ID:   "202605120001_projects_and_members",
		Name: "create project and project member tables",
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS projects (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  name VARCHAR(255) NOT NULL,
  brand VARCHAR(255) NOT NULL DEFAULT '',
  asin VARCHAR(32) NOT NULL DEFAULT '',
  site VARCHAR(64) NOT NULL DEFAULT '',
  notes TEXT NULL,
  status VARCHAR(32) NOT NULL,
  created_by VARCHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  UNIQUE KEY uk_projects_tenant_id (tenant_id, id),
  KEY idx_projects_tenant_status_created (tenant_id, status, created_at),
  KEY idx_projects_tenant_asin (tenant_id, asin),
  KEY idx_projects_tenant_deleted (tenant_id, deleted_at),
  KEY idx_projects_created_by (created_by),
  CONSTRAINT fk_projects_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_projects_created_by FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`,
			`
CREATE TABLE IF NOT EXISTS project_members (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  project_id VARCHAR(36) NOT NULL,
  user_id VARCHAR(36) NOT NULL,
  role VARCHAR(32) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_project_members_tenant_project_user (tenant_id, project_id, user_id),
  KEY idx_project_members_tenant_project (tenant_id, project_id),
  KEY idx_project_members_tenant_user (tenant_id, user_id),
  KEY idx_project_members_role (role),
  CONSTRAINT fk_project_members_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_project_members_project FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id),
  CONSTRAINT fk_project_members_user FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
			`,
		},
	},
	{
		ID:   "202605120002_image_assets",
		Name: "create image asset metadata table",
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS image_assets (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  project_id VARCHAR(36) NOT NULL,
  kind VARCHAR(32) NOT NULL,
  category VARCHAR(128) NOT NULL DEFAULT '',
  filename VARCHAR(255) NOT NULL DEFAULT '',
  object_key VARCHAR(512) NOT NULL,
  thumbnail_object_key VARCHAR(512) NULL,
  mime_type VARCHAR(128) NOT NULL,
  size_bytes BIGINT UNSIGNED NOT NULL,
  width INT UNSIGNED NOT NULL,
  height INT UNSIGNED NOT NULL,
  sha256 CHAR(64) NOT NULL,
  is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
  source_task_id VARCHAR(36) NULL,
  created_by VARCHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  UNIQUE KEY uk_image_assets_tenant_id (tenant_id, id),
  UNIQUE KEY uk_image_assets_object_key (object_key),
  KEY idx_image_assets_tenant_project_created (tenant_id, project_id, created_at),
  KEY idx_image_assets_tenant_project_kind (tenant_id, project_id, kind),
  KEY idx_image_assets_tenant_favorite (tenant_id, is_favorite),
  KEY idx_image_assets_tenant_deleted (tenant_id, deleted_at),
  KEY idx_image_assets_tenant_sha256 (tenant_id, sha256),
  KEY idx_image_assets_created_by (created_by),
  CONSTRAINT fk_image_assets_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_image_assets_project FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id),
  CONSTRAINT fk_image_assets_created_by FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Image metadata only; bytes live in MinIO object_key'
`,
		},
	},
	{
		ID:   "202605120003_ai_providers",
		Name: "create tenant scoped ai provider configuration table",
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS ai_providers (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  type VARCHAR(32) NOT NULL,
  name VARCHAR(255) NOT NULL,
  base_url VARCHAR(512) NOT NULL,
  encrypted_api_key TEXT NOT NULL,
  api_key_hint VARCHAR(32) NOT NULL,
  api_key_updated_at DATETIME(3) NULL,
  status VARCHAR(32) NOT NULL,
  timeout_seconds INT UNSIGNED NOT NULL,
  concurrency_limit INT UNSIGNED NOT NULL,
  last_test_status VARCHAR(32) NOT NULL DEFAULT '',
  last_tested_at DATETIME(3) NULL,
  last_test_error VARCHAR(255) NOT NULL DEFAULT '',
  created_by VARCHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  UNIQUE KEY uk_ai_providers_tenant_id (tenant_id, id),
  KEY idx_ai_providers_tenant_type (tenant_id, type),
  KEY idx_ai_providers_tenant_status (tenant_id, status),
  KEY idx_ai_providers_tenant_deleted (tenant_id, deleted_at),
  KEY idx_ai_providers_created_by (created_by),
  CONSTRAINT fk_ai_providers_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_ai_providers_created_by FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Provider API keys are stored only as encrypted payloads'
`,
		},
	},
	{
		ID:   "202605130001_ai_models",
		Name: "create tenant scoped ai model capability table",
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS ai_models (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  provider_id VARCHAR(36) NOT NULL,
  model_name VARCHAR(255) NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  supports_generate BOOLEAN NOT NULL DEFAULT FALSE,
  supports_edit BOOLEAN NOT NULL DEFAULT FALSE,
  supports_multi_reference BOOLEAN NOT NULL DEFAULT FALSE,
  supports_n BOOLEAN NOT NULL DEFAULT FALSE,
  max_output_count INT UNSIGNED NOT NULL DEFAULT 1,
  supported_sizes_json JSON NOT NULL,
  supported_qualities_json JSON NOT NULL,
  supported_output_formats_json JSON NOT NULL,
  pricing_json JSON NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_by VARCHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  UNIQUE KEY uk_ai_models_tenant_id (tenant_id, id),
  KEY idx_ai_models_tenant_provider (tenant_id, provider_id),
  KEY idx_ai_models_tenant_status (tenant_id, status),
  KEY idx_ai_models_tenant_provider_model (tenant_id, provider_id, model_name),
  KEY idx_ai_models_tenant_generate (tenant_id, supports_generate),
  KEY idx_ai_models_tenant_edit (tenant_id, supports_edit),
  KEY idx_ai_models_tenant_deleted (tenant_id, deleted_at),
  KEY idx_ai_models_created_by (created_by),
  CONSTRAINT fk_ai_models_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_ai_models_provider FOREIGN KEY (tenant_id, provider_id) REFERENCES ai_providers(tenant_id, id),
  CONSTRAINT fk_ai_models_created_by FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tenant-scoped AI model capability configuration with validated JSON fields'
`,
		},
	},
	{
		ID:   "202605130002_tasks_events_outputs_usage",
		Name: "create generation task state event output usage and api call log tables",
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS generation_tasks (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  project_id VARCHAR(36) NOT NULL,
  type VARCHAR(32) NOT NULL,
  provider_id VARCHAR(36) NOT NULL,
  model_id VARCHAR(36) NOT NULL,
  status VARCHAR(32) NOT NULL,
  prompt TEXT NOT NULL,
  image_type VARCHAR(64) NOT NULL DEFAULT '',
  params_json JSON NOT NULL,
  input_asset_ids_json JSON NOT NULL,
  attempt INT UNSIGNED NOT NULL DEFAULT 1,
  max_attempts INT UNSIGNED NOT NULL DEFAULT 3,
  queued_at DATETIME(3) NULL,
  started_at DATETIME(3) NULL,
  finished_at DATETIME(3) NULL,
  timeout_at DATETIME(3) NULL,
  created_by VARCHAR(36) NOT NULL,
  error_code VARCHAR(128) NOT NULL DEFAULT '',
  error_message VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_generation_tasks_tenant_id (tenant_id, id),
  KEY idx_generation_tasks_tenant_project_created (tenant_id, project_id, created_at),
  KEY idx_generation_tasks_tenant_status (tenant_id, status),
  KEY idx_generation_tasks_tenant_created_by (tenant_id, created_by, created_at),
  KEY idx_generation_tasks_tenant_provider_status (tenant_id, provider_id, status),
  KEY idx_generation_tasks_tenant_model_status (tenant_id, model_id, status),
  KEY idx_generation_tasks_tenant_timeout (tenant_id, timeout_at),
  CONSTRAINT fk_generation_tasks_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_generation_tasks_project FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id),
  CONSTRAINT fk_generation_tasks_provider FOREIGN KEY (tenant_id, provider_id) REFERENCES ai_providers(tenant_id, id),
  CONSTRAINT fk_generation_tasks_model FOREIGN KEY (tenant_id, model_id) REFERENCES ai_models(tenant_id, id),
  CONSTRAINT fk_generation_tasks_created_by FOREIGN KEY (tenant_id, created_by) REFERENCES users(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='MySQL final source of durable generation task state'
`,
			`
CREATE TABLE IF NOT EXISTS task_events (
  sequence BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(36) NOT NULL,
  task_id VARCHAR(36) NOT NULL,
  project_id VARCHAR(36) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  event_payload_json JSON NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (sequence),
  UNIQUE KEY uk_task_events_id (id),
  KEY idx_task_events_tenant_task_sequence (tenant_id, task_id, sequence),
  KEY idx_task_events_tenant_project_sequence (tenant_id, project_id, sequence),
  KEY idx_task_events_tenant_sequence (tenant_id, sequence),
  KEY idx_task_events_event_type (event_type),
  CONSTRAINT fk_task_events_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_task_events_task FOREIGN KEY (tenant_id, task_id) REFERENCES generation_tasks(tenant_id, id),
  CONSTRAINT fk_task_events_project FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SSE replay source; payloads must be structured and redacted'
`,
			`
CREATE TABLE IF NOT EXISTS task_outputs (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  task_id VARCHAR(36) NOT NULL,
  asset_id VARCHAR(36) NOT NULL,
  output_index INT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_task_outputs_tenant_id (tenant_id, id),
  UNIQUE KEY uk_task_outputs_tenant_task_index (tenant_id, task_id, output_index),
  KEY idx_task_outputs_tenant_task (tenant_id, task_id),
  KEY idx_task_outputs_tenant_asset (tenant_id, asset_id),
  CONSTRAINT fk_task_outputs_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_task_outputs_task FOREIGN KEY (tenant_id, task_id) REFERENCES generation_tasks(tenant_id, id),
  CONSTRAINT fk_task_outputs_asset FOREIGN KEY (tenant_id, asset_id) REFERENCES image_assets(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Maps completed task outputs to authorized image asset metadata'
`,
			`
CREATE TABLE IF NOT EXISTS api_call_logs (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  task_id VARCHAR(36) NOT NULL,
  provider_id VARCHAR(36) NOT NULL,
  model_id VARCHAR(36) NOT NULL,
  status VARCHAR(32) NOT NULL,
  duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0,
  request_id VARCHAR(128) NOT NULL DEFAULT '',
  http_status INT UNSIGNED NULL,
  error_code VARCHAR(128) NOT NULL DEFAULT '',
  error_message VARCHAR(512) NOT NULL DEFAULT '',
  redacted_request_json JSON NULL,
  redacted_response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_api_call_logs_tenant_task (tenant_id, task_id),
  KEY idx_api_call_logs_tenant_provider (tenant_id, provider_id),
  KEY idx_api_call_logs_tenant_created (tenant_id, created_at),
  KEY idx_api_call_logs_model_id (model_id),
  KEY idx_api_call_logs_status (status),
  CONSTRAINT fk_api_call_logs_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_api_call_logs_task FOREIGN KEY (tenant_id, task_id) REFERENCES generation_tasks(tenant_id, id),
  CONSTRAINT fk_api_call_logs_provider FOREIGN KEY (tenant_id, provider_id) REFERENCES ai_providers(tenant_id, id),
  CONSTRAINT fk_api_call_logs_model FOREIGN KEY (tenant_id, model_id) REFERENCES ai_models(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Provider call audit metadata only; never store API keys, Authorization headers, Cookies, image base64, or raw image bytes'
`,
			`
CREATE TABLE IF NOT EXISTS usage_records (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  task_id VARCHAR(36) NOT NULL,
  user_id VARCHAR(36) NOT NULL,
  project_id VARCHAR(36) NOT NULL,
  provider_id VARCHAR(36) NOT NULL,
  model_id VARCHAR(36) NOT NULL,
  input_tokens BIGINT UNSIGNED NOT NULL DEFAULT 0,
  output_tokens BIGINT UNSIGNED NOT NULL DEFAULT 0,
  image_count INT UNSIGNED NOT NULL DEFAULT 0,
  estimated_cost DECIMAL(18,8) NOT NULL DEFAULT 0,
  currency CHAR(3) NOT NULL DEFAULT 'USD',
  raw_usage_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_usage_records_tenant_task (tenant_id, task_id),
  KEY idx_usage_records_tenant_project_created (tenant_id, project_id, created_at),
  KEY idx_usage_records_tenant_user_created (tenant_id, user_id, created_at),
  KEY idx_usage_records_provider_id (provider_id),
  KEY idx_usage_records_model_id (model_id),
  CONSTRAINT fk_usage_records_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_usage_records_task FOREIGN KEY (tenant_id, task_id) REFERENCES generation_tasks(tenant_id, id),
  CONSTRAINT fk_usage_records_user FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id),
  CONSTRAINT fk_usage_records_project FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id),
  CONSTRAINT fk_usage_records_provider FOREIGN KEY (tenant_id, provider_id) REFERENCES ai_providers(tenant_id, id),
  CONSTRAINT fk_usage_records_model FOREIGN KEY (tenant_id, model_id) REFERENCES ai_models(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Usage and estimated cost metadata; raw usage must be sanitized before insert'
			`,
		},
	},
	{
		ID:   "202605180001_system_settings",
		Name: "create tenant scoped system settings table",
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS system_settings (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  ` + "`key`" + ` VARCHAR(128) NOT NULL,
  value_json JSON NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_system_settings_tenant_id (tenant_id, id),
  UNIQUE KEY uk_system_settings_tenant_key (tenant_id, ` + "`key`" + `),
  KEY idx_system_settings_tenant_id (tenant_id),
  CONSTRAINT fk_system_settings_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tenant-scoped system settings; first active key is upload_policy only'
`,
		},
	},
}
