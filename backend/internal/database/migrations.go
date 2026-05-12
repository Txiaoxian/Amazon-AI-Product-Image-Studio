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
}
