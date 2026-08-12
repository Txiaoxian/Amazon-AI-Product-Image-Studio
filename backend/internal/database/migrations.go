package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	migrationLockName           = "amazon-ai-product-image-studio:migrations"
	migrationLockTimeoutSeconds = 30
	migrationLockReleaseTimeout = 5 * time.Second
)

type Migration struct {
	ID         string
	Name       string
	Statements []string
	Checks     []SchemaCheck
}

type SchemaCheck struct {
	Table   string
	Columns []string
	Indexes []string
}

func RunMigrations(ctx context.Context, db *gorm.DB) error {
	return runMigrations(ctx, db, migrations, withMigrationLock)
}

type migrationLockRunner func(context.Context, *gorm.DB, func(*gorm.DB) error) error

func runMigrations(ctx context.Context, db *gorm.DB, migrationList []Migration, lockRunner migrationLockRunner) error {
	if db == nil {
		return ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if lockRunner == nil {
		lockRunner = withMigrationLock
	}

	db = db.WithContext(ctx)
	return lockRunner(ctx, db, func(lockedDB *gorm.DB) error {
		db := lockedDB.WithContext(ctx)
		if err := ensureSchemaMigrations(db); err != nil {
			return err
		}

		for _, migration := range migrationList {
			if err := validateMigrationDefinition(migration); err != nil {
				return err
			}

			applied, err := migrationApplied(db, migration.ID)
			if err != nil {
				return err
			}
			if !applied {
				if err := runMigration(db, migration); err != nil {
					return err
				}
			}

			if err := verifyMigrationSchema(db, migration); err != nil {
				return err
			}
		}

		return nil
	})
}

func ensureSchemaMigrations(db *gorm.DB) error {
	statement := `
CREATE TABLE IF NOT EXISTS schema_migrations (
  id VARCHAR(128) NOT NULL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Backend migration history'
`
	if dialectName(db) == "sqlite" {
		statement = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  id TEXT NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)
`
	}

	if err := db.Exec(statement).Error; err != nil {
		return migrationDatabaseError("ensure schema_migrations table", err)
	}

	return nil
}

func migrationApplied(db *gorm.DB, id string) (bool, error) {
	var count int64
	if err := db.Table("schema_migrations").Where("id = ?", id).Count(&count).Error; err != nil {
		return false, migrationDatabaseError(fmt.Sprintf("check migration %s", id), err)
	}

	return count > 0, nil
}

func validateMigrationDefinition(migration Migration) error {
	if strings.TrimSpace(migration.ID) == "" {
		return fmt.Errorf("migration id is required")
	}
	if strings.TrimSpace(migration.Name) == "" {
		return fmt.Errorf("migration name is required")
	}
	return nil
}

func runMigration(db *gorm.DB, migration Migration) error {
	if err := validateMigrationDefinition(migration); err != nil {
		return err
	}

	for _, statement := range migration.Statements {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if err := executeMigrationStatement(db, migration.ID, statement); err != nil {
			return err
		}
	}

	if err := db.Table("schema_migrations").Create(map[string]any{
		"id":   migration.ID,
		"name": migration.Name,
	}).Error; err != nil {
		return migrationDatabaseError(fmt.Sprintf("record migration %s", migration.ID), err)
	}

	return nil
}

var (
	addColumnStatementPattern   = regexp.MustCompile("(?is)^ALTER\\s+TABLE\\s+`?([A-Za-z0-9_]+)`?\\s+ADD\\s+COLUMN\\s+`?([A-Za-z0-9_]+)`?\\b")
	createIndexStatementPattern = regexp.MustCompile(
		"(?is)^CREATE\\s+(?:UNIQUE\\s+)?INDEX\\s+`?([A-Za-z0-9_]+)`?\\s+ON\\s+`?([A-Za-z0-9_]+)`?\\s*\\(",
	)
)

func executeMigrationStatement(db *gorm.DB, migrationID, statement string) error {
	statement = strings.TrimSpace(statement)
	if match := addColumnStatementPattern.FindStringSubmatch(statement); match != nil {
		table, column := match[1], match[2]
		exists, err := columnExists(db, table, column)
		if err != nil {
			return migrationDatabaseError(fmt.Sprintf("check migration %s column %s.%s", migrationID, table, column), err)
		}
		if exists {
			return nil
		}
	}
	if match := createIndexStatementPattern.FindStringSubmatch(statement); match != nil {
		indexName, table := match[1], match[2]
		exists, err := indexExists(db, table, indexName)
		if err != nil {
			return migrationDatabaseError(fmt.Sprintf("check migration %s index %s", migrationID, indexName), err)
		}
		if exists {
			return nil
		}
	}

	if err := db.Exec(statement).Error; err != nil {
		return migrationDatabaseError(fmt.Sprintf("apply migration %s", migrationID), err)
	}

	return nil
}

func verifyMigrationSchema(db *gorm.DB, migration Migration) error {
	for _, check := range migration.Checks {
		table := strings.TrimSpace(check.Table)
		if table == "" {
			return fmt.Errorf("verify migration %s: schema check table is required", migration.ID)
		}

		exists, err := tableExists(db, table)
		if err != nil {
			return migrationDatabaseError(fmt.Sprintf("verify migration %s table %s", migration.ID, table), err)
		}
		if !exists {
			return fmt.Errorf("verify migration %s: missing table %s", migration.ID, table)
		}

		for _, column := range check.Columns {
			column = strings.TrimSpace(column)
			if column == "" {
				return fmt.Errorf("verify migration %s: schema check column is required", migration.ID)
			}
			exists, err := columnExists(db, table, column)
			if err != nil {
				return migrationDatabaseError(fmt.Sprintf("verify migration %s column %s.%s", migration.ID, table, column), err)
			}
			if !exists {
				return fmt.Errorf("verify migration %s: missing column %s.%s", migration.ID, table, column)
			}
		}

		for _, indexName := range check.Indexes {
			indexName = strings.TrimSpace(indexName)
			if indexName == "" {
				return fmt.Errorf("verify migration %s: schema check index is required", migration.ID)
			}
			exists, err := indexExists(db, table, indexName)
			if err != nil {
				return migrationDatabaseError(fmt.Sprintf("verify migration %s index %s", migration.ID, indexName), err)
			}
			if !exists {
				return fmt.Errorf("verify migration %s: missing index %s on %s", migration.ID, indexName, table)
			}
		}
	}

	return nil
}

func withMigrationLock(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
	if dialectName(db) != "mysql" {
		return fn(db)
	}

	connectionOpened := false
	err := db.Connection(func(connDB *gorm.DB) error {
		connectionOpened = true
		connDB = connDB.WithContext(ctx)
		if err := acquireMySQLMigrationLock(ctx, connDB); err != nil {
			return err
		}

		runErr := fn(connDB)

		releaseCtx, cancel := context.WithTimeout(context.Background(), migrationLockReleaseTimeout)
		defer cancel()
		releaseErr := releaseMySQLMigrationLock(releaseCtx, connDB.WithContext(releaseCtx))
		if runErr != nil {
			if releaseErr != nil {
				return fmt.Errorf("%w; %v", runErr, releaseErr)
			}
			return runErr
		}
		if releaseErr != nil {
			return releaseErr
		}

		return nil
	})
	if err != nil && !connectionOpened {
		return migrationDatabaseError("open migration lock connection", err)
	}
	return err
}

func acquireMySQLMigrationLock(ctx context.Context, db *gorm.DB) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}

	var acquired sql.NullInt64
	if err := db.WithContext(ctx).Raw("SELECT GET_LOCK(?, ?)", migrationLockName, migrationLockTimeoutSeconds).Scan(&acquired).Error; err != nil {
		return migrationDatabaseError("acquire migration lock", err)
	}
	if !acquired.Valid {
		return fmt.Errorf("acquire migration lock: database returned no lock state")
	}
	if acquired.Int64 != 1 {
		return fmt.Errorf("acquire migration lock: lock not acquired before timeout")
	}

	return nil
}

func releaseMySQLMigrationLock(ctx context.Context, db *gorm.DB) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("release migration lock: %w", err)
	}

	var released sql.NullInt64
	if err := db.WithContext(ctx).Raw("SELECT RELEASE_LOCK(?)", migrationLockName).Scan(&released).Error; err != nil {
		return migrationDatabaseError("release migration lock", err)
	}
	if !released.Valid {
		return fmt.Errorf("release migration lock: database returned no lock state")
	}
	if released.Int64 != 1 {
		return fmt.Errorf("release migration lock: lock was not held by this connection")
	}

	return nil
}

func tableExists(db *gorm.DB, table string) (bool, error) {
	switch dialectName(db) {
	case "mysql":
		var count int64
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			table,
		).Scan(&count).Error
		return count > 0, err
	case "sqlite":
		var count int64
		err := db.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count).Error
		return count > 0, err
	default:
		return db.Migrator().HasTable(table), nil
	}
}

func columnExists(db *gorm.DB, table, column string) (bool, error) {
	switch dialectName(db) {
	case "mysql":
		var count int64
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
			table,
			column,
		).Scan(&count).Error
		return count > 0, err
	case "sqlite":
		rows, err := db.Raw("PRAGMA table_info(" + quoteSQLiteIdentifier(table) + ")").Rows()
		if err != nil {
			return false, err
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name, columnType string
			var notNull int
			var defaultValue sql.NullString
			var primaryKey int
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
				return false, err
			}
			if strings.EqualFold(name, column) {
				return true, nil
			}
		}
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	default:
		return db.Migrator().HasColumn(table, column), nil
	}
}

func indexExists(db *gorm.DB, table, indexName string) (bool, error) {
	switch dialectName(db) {
	case "mysql":
		var count int64
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
			table,
			indexName,
		).Scan(&count).Error
		return count > 0, err
	case "sqlite":
		var count int64
		err := db.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND name = ?",
			table,
			indexName,
		).Scan(&count).Error
		return count > 0, err
	default:
		return db.Migrator().HasIndex(table, indexName), nil
	}
}

func quoteSQLiteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func dialectName(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return ""
	}
	return strings.ToLower(db.Dialector.Name())
}

func migrationDatabaseError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w", operation, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", operation, context.DeadlineExceeded)
	}
	return fmt.Errorf("%s: database operation failed", operation)
}

func schemaCheck(table string, columns []string, indexes []string) SchemaCheck {
	return SchemaCheck{Table: table, Columns: columns, Indexes: indexes}
}

var migrations = []Migration{
	{
		ID:   "202605110001_identity_and_audit_base",
		Name: "create tenant identity rbac and operation log tables",
		Checks: []SchemaCheck{
			schemaCheck("tenants", []string{"id", "name", "status", "created_at", "updated_at"}, []string{"idx_tenants_status"}),
			schemaCheck("users", []string{"id", "tenant_id", "email", "display_name", "password_hash", "status", "last_login_at", "created_at", "updated_at"}, []string{"uk_users_tenant_email", "idx_users_tenant_id", "uk_users_tenant_id", "idx_users_status"}),
			schemaCheck("roles", []string{"id", "tenant_id", "code", "name", "description", "status", "created_at", "updated_at"}, []string{"uk_roles_tenant_code", "idx_roles_tenant_id", "uk_roles_tenant_id", "idx_roles_status"}),
			schemaCheck("permissions", []string{"id", "code", "name", "description", "created_at", "updated_at"}, []string{"uk_permissions_code"}),
			schemaCheck("user_roles", []string{"id", "tenant_id", "user_id", "role_id", "created_at"}, []string{"uk_user_roles_tenant_user_role", "idx_user_roles_tenant_id", "idx_user_roles_user_id", "idx_user_roles_role_id"}),
			schemaCheck("role_permissions", []string{"id", "tenant_id", "role_id", "permission_id", "created_at"}, []string{"uk_role_permissions_tenant_role_permission", "idx_role_permissions_tenant_id", "idx_role_permissions_role_id", "idx_role_permissions_permission_id"}),
			schemaCheck("operation_logs", []string{"id", "tenant_id", "actor_user_id", "action", "resource_type", "resource_id", "ip", "user_agent", "metadata_json", "created_at"}, []string{"idx_operation_logs_tenant_id", "idx_operation_logs_tenant_created", "idx_operation_logs_actor_user_id", "idx_operation_logs_action", "idx_operation_logs_resource_type", "idx_operation_logs_resource_id"}),
		},
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
		ID:   "202606050001_user_session_version",
		Name: "add user session version for revocable jwt sessions",
		Checks: []SchemaCheck{
			schemaCheck("users", []string{"session_version"}, nil),
		},
		Statements: []string{
			`
ALTER TABLE users
  ADD COLUMN session_version BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER status
`,
		},
	},
	{
		ID:   "202605120001_projects_and_members",
		Name: "create project and project member tables",
		Checks: []SchemaCheck{
			schemaCheck("projects", []string{"id", "tenant_id", "name", "brand", "asin", "site", "notes", "status", "created_by", "created_at", "updated_at", "deleted_at"}, []string{"uk_projects_tenant_id", "idx_projects_tenant_status_created", "idx_projects_tenant_asin", "idx_projects_tenant_deleted", "idx_projects_created_by"}),
			schemaCheck("project_members", []string{"id", "tenant_id", "project_id", "user_id", "role", "created_at", "updated_at"}, []string{"uk_project_members_tenant_project_user", "idx_project_members_tenant_project", "idx_project_members_tenant_user", "idx_project_members_role"}),
		},
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
		ID:   "202606050002_project_sort_order",
		Name: "add tenant scoped project sort order",
		Checks: []SchemaCheck{
			schemaCheck("projects", []string{"sort_order"}, []string{"idx_projects_tenant_status_sort"}),
		},
		Statements: []string{
			`ALTER TABLE projects ADD COLUMN sort_order INT NOT NULL DEFAULT 0`,
			`CREATE INDEX idx_projects_tenant_status_sort ON projects (tenant_id, status, sort_order, created_at)`,
		},
	},
	{
		ID:   "202605120002_image_assets",
		Name: "create image asset metadata table",
		Checks: []SchemaCheck{
			schemaCheck("image_assets", []string{"id", "tenant_id", "project_id", "kind", "category", "filename", "object_key", "thumbnail_object_key", "mime_type", "size_bytes", "width", "height", "sha256", "is_favorite", "source_task_id", "created_by", "created_at", "updated_at", "deleted_at"}, []string{"uk_image_assets_tenant_id", "uk_image_assets_object_key", "idx_image_assets_tenant_project_created", "idx_image_assets_tenant_project_kind", "idx_image_assets_tenant_favorite", "idx_image_assets_tenant_deleted", "idx_image_assets_tenant_sha256", "idx_image_assets_created_by"}),
		},
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
		Checks: []SchemaCheck{
			schemaCheck("ai_providers", []string{"id", "tenant_id", "type", "name", "base_url", "encrypted_api_key", "api_key_hint", "api_key_updated_at", "status", "timeout_seconds", "concurrency_limit", "last_test_status", "last_tested_at", "last_test_error", "created_by", "created_at", "updated_at", "deleted_at"}, []string{"uk_ai_providers_tenant_id", "idx_ai_providers_tenant_type", "idx_ai_providers_tenant_status", "idx_ai_providers_tenant_deleted", "idx_ai_providers_created_by"}),
		},
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
		Checks: []SchemaCheck{
			schemaCheck("ai_models", []string{"id", "tenant_id", "provider_id", "model_name", "display_name", "supports_generate", "supports_edit", "supports_multi_reference", "supports_n", "max_output_count", "supported_sizes_json", "supported_qualities_json", "supported_output_formats_json", "pricing_json", "status", "created_by", "created_at", "updated_at", "deleted_at"}, []string{"uk_ai_models_tenant_id", "idx_ai_models_tenant_provider", "idx_ai_models_tenant_status", "idx_ai_models_tenant_provider_model", "idx_ai_models_tenant_generate", "idx_ai_models_tenant_edit", "idx_ai_models_tenant_deleted", "idx_ai_models_created_by"}),
		},
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
		Checks: []SchemaCheck{
			schemaCheck("generation_tasks", []string{"id", "tenant_id", "project_id", "type", "provider_id", "model_id", "status", "prompt", "image_type", "params_json", "input_asset_ids_json", "attempt", "max_attempts", "queued_at", "started_at", "finished_at", "timeout_at", "created_by", "error_code", "error_message", "created_at", "updated_at"}, []string{"uk_generation_tasks_tenant_id", "idx_generation_tasks_tenant_project_created", "idx_generation_tasks_tenant_status", "idx_generation_tasks_tenant_created_by", "idx_generation_tasks_tenant_provider_status", "idx_generation_tasks_tenant_model_status", "idx_generation_tasks_tenant_timeout"}),
			schemaCheck("task_events", []string{"sequence", "id", "tenant_id", "task_id", "project_id", "event_type", "event_payload_json", "created_at"}, []string{"uk_task_events_id", "idx_task_events_tenant_task_sequence", "idx_task_events_tenant_project_sequence", "idx_task_events_tenant_sequence", "idx_task_events_event_type"}),
			schemaCheck("task_outputs", []string{"id", "tenant_id", "task_id", "asset_id", "output_index", "created_at"}, []string{"uk_task_outputs_tenant_id", "uk_task_outputs_tenant_task_index", "idx_task_outputs_tenant_task", "idx_task_outputs_tenant_asset"}),
			schemaCheck("api_call_logs", []string{"id", "tenant_id", "task_id", "provider_id", "model_id", "status", "duration_ms", "request_id", "http_status", "error_code", "error_message", "redacted_request_json", "redacted_response_json", "created_at"}, []string{"idx_api_call_logs_tenant_task", "idx_api_call_logs_tenant_provider", "idx_api_call_logs_tenant_created", "idx_api_call_logs_model_id", "idx_api_call_logs_status"}),
			schemaCheck("usage_records", []string{"id", "tenant_id", "task_id", "user_id", "project_id", "provider_id", "model_id", "input_tokens", "output_tokens", "image_count", "estimated_cost", "currency", "raw_usage_json", "created_at"}, []string{"idx_usage_records_tenant_task", "idx_usage_records_tenant_project_created", "idx_usage_records_tenant_user_created", "idx_usage_records_provider_id", "idx_usage_records_model_id"}),
		},
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
		Checks: []SchemaCheck{
			schemaCheck("system_settings", []string{"id", "tenant_id", "key", "value_json", "created_at", "updated_at"}, []string{"uk_system_settings_tenant_id", "uk_system_settings_tenant_key", "idx_system_settings_tenant_id"}),
		},
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tenant-scoped system settings stored as JSON by active key'
`,
		},
	},
	{
		ID:   "202605240001_image_asset_purge_marker",
		Name: "add durable image asset physical purge marker",
		Checks: []SchemaCheck{
			schemaCheck("image_assets", []string{"purged_at"}, []string{"idx_image_assets_tenant_deleted_purged"}),
		},
		Statements: []string{
			`ALTER TABLE image_assets ADD COLUMN purged_at DATETIME(3) NULL`,
			`CREATE INDEX idx_image_assets_tenant_deleted_purged ON image_assets (tenant_id, deleted_at, purged_at)`,
		},
	},
	{
		ID:   "202605270001_storage_quota_reservations",
		Name: "create tenant scoped storage quota counter and reservation tables",
		Checks: []SchemaCheck{
			schemaCheck("storage_quota_counters", []string{"id", "tenant_id", "used_bytes", "reserved_bytes", "reconciled_at", "created_at", "updated_at"}, []string{"uk_storage_quota_counters_tenant", "uk_storage_quota_counters_tenant_id"}),
			schemaCheck("storage_quota_reservations", []string{"id", "tenant_id", "bytes", "finalized_bytes", "status", "expires_at", "created_at", "updated_at"}, []string{"uk_storage_quota_reservations_tenant_id", "idx_storage_quota_reservations_tenant_status", "idx_storage_quota_reservations_tenant_expires"}),
		},
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS storage_quota_counters (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  used_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
  reserved_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
  reconciled_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_storage_quota_counters_tenant (tenant_id),
  UNIQUE KEY uk_storage_quota_counters_tenant_id (tenant_id, id),
  CONSTRAINT fk_storage_quota_counters_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tenant storage quota used and reserved byte counters; image_assets remains reconciliation truth'
`,
			`
CREATE TABLE IF NOT EXISTS storage_quota_reservations (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  bytes BIGINT UNSIGNED NOT NULL,
  finalized_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_storage_quota_reservations_tenant_id (tenant_id, id),
  KEY idx_storage_quota_reservations_tenant_status (tenant_id, status),
  KEY idx_storage_quota_reservations_tenant_expires (tenant_id, expires_at),
  CONSTRAINT fk_storage_quota_reservations_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Internal storage quota reservations; never expose IDs in API responses or audit metadata'
`,
		},
	},
	{
		ID:   "202607130001_user_model_access_grants",
		Name: "create tenant scoped user model access grants",
		Checks: []SchemaCheck{
			schemaCheck("user_model_access_grants", []string{"id", "tenant_id", "user_id", "model_id", "granted_by", "created_at", "updated_at"}, []string{"uk_user_model_access_tenant_user_model", "idx_user_model_access_tenant_user", "idx_user_model_access_tenant_model", "idx_user_model_access_granted_by"}),
		},
		Statements: []string{
			`
CREATE TABLE IF NOT EXISTS user_model_access_grants (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  user_id VARCHAR(36) NOT NULL,
  model_id VARCHAR(36) NOT NULL,
  granted_by VARCHAR(36) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_user_model_access_tenant_user_model (tenant_id, user_id, model_id),
  KEY idx_user_model_access_tenant_user (tenant_id, user_id),
  KEY idx_user_model_access_tenant_model (tenant_id, model_id),
  KEY idx_user_model_access_granted_by (tenant_id, granted_by),
  CONSTRAINT fk_user_model_access_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CONSTRAINT fk_user_model_access_user FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id),
  CONSTRAINT fk_user_model_access_model FOREIGN KEY (tenant_id, model_id) REFERENCES ai_models(tenant_id, id),
  CONSTRAINT fk_user_model_access_granted_by FOREIGN KEY (tenant_id, granted_by) REFERENCES users(tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Administrator-assigned tenant-scoped AI model access for non-admin users'
`,
			`
INSERT IGNORE INTO user_model_access_grants (id, tenant_id, user_id, model_id, granted_by, created_at, updated_at)
SELECT UUID(), users.tenant_id, users.id, ai_models.id, NULL, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)
FROM users
JOIN ai_models ON ai_models.tenant_id = users.tenant_id AND ai_models.deleted_at IS NULL
`,
		},
	},
	{
		ID:   "202608110001_admin_analytics_foundation",
		Name: "add admin analytics cost provenance and tenant time indexes",
		Checks: []SchemaCheck{
			schemaCheck("generation_tasks", nil, []string{"idx_generation_tasks_tenant_created", "idx_generation_tasks_tenant_provider_created", "idx_generation_tasks_tenant_model_created"}),
			schemaCheck("task_outputs", nil, []string{"idx_task_outputs_tenant_created"}),
			schemaCheck("api_call_logs", nil, []string{"idx_api_call_logs_tenant_provider_created", "idx_api_call_logs_tenant_model_created", "idx_api_call_logs_tenant_status_created"}),
			schemaCheck("usage_records", []string{"cost_status", "pricing_snapshot_json"}, []string{"idx_usage_records_tenant_created", "idx_usage_records_tenant_provider_created", "idx_usage_records_tenant_model_created"}),
		},
		Statements: []string{
			`ALTER TABLE usage_records ADD COLUMN cost_status VARCHAR(32) NOT NULL DEFAULT 'LEGACY_UNKNOWN'`,
			`ALTER TABLE usage_records ADD COLUMN pricing_snapshot_json JSON NULL`,
			`CREATE INDEX idx_generation_tasks_tenant_created ON generation_tasks (tenant_id, created_at)`,
			`CREATE INDEX idx_generation_tasks_tenant_provider_created ON generation_tasks (tenant_id, provider_id, created_at)`,
			`CREATE INDEX idx_generation_tasks_tenant_model_created ON generation_tasks (tenant_id, model_id, created_at)`,
			`CREATE INDEX idx_task_outputs_tenant_created ON task_outputs (tenant_id, created_at)`,
			`CREATE INDEX idx_api_call_logs_tenant_provider_created ON api_call_logs (tenant_id, provider_id, created_at)`,
			`CREATE INDEX idx_api_call_logs_tenant_model_created ON api_call_logs (tenant_id, model_id, created_at)`,
			`CREATE INDEX idx_api_call_logs_tenant_status_created ON api_call_logs (tenant_id, status, created_at)`,
			`CREATE INDEX idx_usage_records_tenant_created ON usage_records (tenant_id, created_at)`,
			`CREATE INDEX idx_usage_records_tenant_provider_created ON usage_records (tenant_id, provider_id, created_at)`,
			`CREATE INDEX idx_usage_records_tenant_model_created ON usage_records (tenant_id, model_id, created_at)`,
		},
	},
}
