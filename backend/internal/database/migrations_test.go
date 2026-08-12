package database

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestMigrationsUseIdempotentTableCreation(t *testing.T) {
	for _, migration := range migrations {
		if strings.TrimSpace(migration.ID) == "" {
			t.Fatal("migration ID must not be empty")
		}
		seenStatements := 0
		for _, statement := range migration.Statements {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			seenStatements++
			if strings.HasPrefix(strings.ToUpper(statement), "CREATE TABLE") && !strings.Contains(statement, "CREATE TABLE IF NOT EXISTS") {
				t.Fatalf("migration %s statement is not idempotent: %s", migration.ID, statement)
			}
		}
		if seenStatements == 0 {
			t.Fatalf("migration %s has no statements", migration.ID)
		}
	}
}

func TestBaseMigrationTenantScopeColumns(t *testing.T) {
	requiredTenantTables := []string{
		"users",
		"roles",
		"user_roles",
		"role_permissions",
		"operation_logs",
		"projects",
		"project_members",
		"image_assets",
		"ai_providers",
		"ai_models",
		"user_model_access_grants",
		"generation_tasks",
		"task_events",
		"task_outputs",
		"api_call_logs",
		"usage_records",
		"system_settings",
	}
	for _, table := range requiredTenantTables {
		statement := findCreateTableStatement(t, table)
		if !strings.Contains(statement, "tenant_id VARCHAR(36) NOT NULL") {
			t.Fatalf("%s migration must include tenant_id", table)
		}
	}

	permissions := findCreateTableStatement(t, "permissions")
	if strings.Contains(permissions, "tenant_id") {
		t.Fatal("permissions must remain a system-level dictionary without tenant_id")
	}
	if !strings.Contains(permissions, "System-level permission dictionary") {
		t.Fatal("permissions migration must document the system-level tenant_id exception")
	}
}

func TestTaskMigrationsUseTenantScopeStatusAndRedactedEventStorage(t *testing.T) {
	tasks := findCreateTableStatement(t, "generation_tasks")
	for _, required := range []string{
		"tenant_id VARCHAR(36) NOT NULL",
		"type VARCHAR(32) NOT NULL",
		"status VARCHAR(32) NOT NULL",
		"params_json JSON NOT NULL",
		"input_asset_ids_json JSON NOT NULL",
		"attempt INT UNSIGNED NOT NULL DEFAULT 1",
		"max_attempts INT UNSIGNED NOT NULL DEFAULT 3",
		"UNIQUE KEY uk_generation_tasks_tenant_id (tenant_id, id)",
		"KEY idx_generation_tasks_tenant_project_created (tenant_id, project_id, created_at)",
		"KEY idx_generation_tasks_tenant_status (tenant_id, status)",
		"KEY idx_generation_tasks_tenant_created_by (tenant_id, created_by, created_at)",
		"KEY idx_generation_tasks_tenant_timeout (tenant_id, timeout_at)",
		"CONSTRAINT fk_generation_tasks_provider FOREIGN KEY (tenant_id, provider_id) REFERENCES ai_providers(tenant_id, id)",
		"MySQL final source of durable generation task state",
	} {
		if !strings.Contains(tasks, required) {
			t.Fatalf("generation_tasks migration missing %q", required)
		}
	}

	events := findCreateTableStatement(t, "task_events")
	for _, required := range []string{
		"sequence BIGINT UNSIGNED NOT NULL AUTO_INCREMENT",
		"tenant_id VARCHAR(36) NOT NULL",
		"event_payload_json JSON NOT NULL",
		"PRIMARY KEY (sequence)",
		"UNIQUE KEY uk_task_events_id (id)",
		"KEY idx_task_events_tenant_task_sequence (tenant_id, task_id, sequence)",
		"KEY idx_task_events_tenant_project_sequence (tenant_id, project_id, sequence)",
		"KEY idx_task_events_tenant_sequence (tenant_id, sequence)",
		"SSE replay source; payloads must be structured and redacted",
	} {
		if !strings.Contains(events, required) {
			t.Fatalf("task_events migration missing %q", required)
		}
	}
}

func TestTaskOutputAPICallAndUsageMigrationsAvoidSensitiveBlobStorage(t *testing.T) {
	for _, table := range []string{"task_outputs", "api_call_logs", "usage_records"} {
		statement := findCreateTableStatement(t, table)
		if !strings.Contains(statement, "tenant_id VARCHAR(36) NOT NULL") {
			t.Fatalf("%s migration missing tenant_id", table)
		}
		for _, forbidden := range []string{" BLOB", " LONGBLOB", " MEDIUMBLOB", " TINYBLOB", "plain_api_key"} {
			if strings.Contains(strings.ToLower(statement), strings.ToLower(forbidden)) {
				t.Fatalf("%s migration must not store sensitive binary or plaintext secrets: found %q", table, forbidden)
			}
		}
	}

	apiCallLogs := findCreateTableStatement(t, "api_call_logs")
	for _, required := range []string{
		"redacted_request_json JSON NULL",
		"redacted_response_json JSON NULL",
		"never store API keys, Authorization headers, Cookies, image base64, or raw image bytes",
	} {
		if !strings.Contains(apiCallLogs, required) {
			t.Fatalf("api_call_logs migration missing %q", required)
		}
	}
}

func TestAIProvidersMigrationProtectsSecretsAndTenantScope(t *testing.T) {
	statement := findCreateTableStatement(t, "ai_providers")
	for _, required := range []string{
		"tenant_id VARCHAR(36) NOT NULL",
		"encrypted_api_key TEXT NOT NULL",
		"api_key_hint VARCHAR(32) NOT NULL",
		"api_key_updated_at DATETIME(3) NULL",
		"deleted_at DATETIME(3) NULL",
		"UNIQUE KEY uk_ai_providers_tenant_id (tenant_id, id)",
		"KEY idx_ai_providers_tenant_type (tenant_id, type)",
		"KEY idx_ai_providers_tenant_status (tenant_id, status)",
		"Provider API keys are stored only as encrypted payloads",
	} {
		if !strings.Contains(statement, required) {
			t.Fatalf("ai_providers migration missing %q", required)
		}
	}
	if strings.Contains(statement, " plain_api_key") {
		t.Fatal("ai_providers migration must not include plaintext API key storage")
	}
}

func TestAIModelsMigrationValidatesTenantScopeAndCapabilityStorage(t *testing.T) {
	statement := findCreateTableStatement(t, "ai_models")
	for _, required := range []string{
		"tenant_id VARCHAR(36) NOT NULL",
		"provider_id VARCHAR(36) NOT NULL",
		"supports_generate BOOLEAN NOT NULL DEFAULT FALSE",
		"supports_edit BOOLEAN NOT NULL DEFAULT FALSE",
		"supports_multi_reference BOOLEAN NOT NULL DEFAULT FALSE",
		"supports_n BOOLEAN NOT NULL DEFAULT FALSE",
		"max_output_count INT UNSIGNED NOT NULL DEFAULT 1",
		"supported_sizes_json JSON NOT NULL",
		"supported_qualities_json JSON NOT NULL",
		"supported_output_formats_json JSON NOT NULL",
		"pricing_json JSON NOT NULL",
		"UNIQUE KEY uk_ai_models_tenant_id (tenant_id, id)",
		"KEY idx_ai_models_tenant_provider (tenant_id, provider_id)",
		"KEY idx_ai_models_tenant_status (tenant_id, status)",
		"KEY idx_ai_models_tenant_generate (tenant_id, supports_generate)",
		"KEY idx_ai_models_tenant_edit (tenant_id, supports_edit)",
		"CONSTRAINT fk_ai_models_provider FOREIGN KEY (tenant_id, provider_id) REFERENCES ai_providers(tenant_id, id)",
		"Tenant-scoped AI model capability configuration with validated JSON fields",
	} {
		if !strings.Contains(statement, required) {
			t.Fatalf("ai_models migration missing %q", required)
		}
	}
}

func TestUserModelAccessGrantMigrationUsesTenantScopedForeignKeys(t *testing.T) {
	statement := findCreateTableStatement(t, "user_model_access_grants")
	for _, required := range []string{
		"tenant_id VARCHAR(36) NOT NULL",
		"user_id VARCHAR(36) NOT NULL",
		"model_id VARCHAR(36) NOT NULL",
		"granted_by VARCHAR(36) NULL",
		"UNIQUE KEY uk_user_model_access_tenant_user_model (tenant_id, user_id, model_id)",
		"KEY idx_user_model_access_tenant_user (tenant_id, user_id)",
		"KEY idx_user_model_access_tenant_model (tenant_id, model_id)",
		"CONSTRAINT fk_user_model_access_user FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id)",
		"CONSTRAINT fk_user_model_access_model FOREIGN KEY (tenant_id, model_id) REFERENCES ai_models(tenant_id, id)",
	} {
		if !strings.Contains(statement, required) {
			t.Fatalf("user_model_access_grants migration missing %q", required)
		}
	}
}

func TestAdminAnalyticsFoundationMigrationKeepsHistoricalCostUnknownAndAddsTenantTimeIndexes(t *testing.T) {
	statements := findMigrationStatements(t, "202608110001_admin_analytics_foundation")
	joined := strings.Join(statements, "\n")
	for _, required := range []string{
		"cost_status VARCHAR(32) NOT NULL DEFAULT 'LEGACY_UNKNOWN'",
		"pricing_snapshot_json JSON NULL",
		"idx_generation_tasks_tenant_created ON generation_tasks (tenant_id, created_at)",
		"idx_task_outputs_tenant_created ON task_outputs (tenant_id, created_at)",
		"idx_api_call_logs_tenant_provider_created ON api_call_logs (tenant_id, provider_id, created_at)",
		"idx_usage_records_tenant_provider_created ON usage_records (tenant_id, provider_id, created_at)",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("admin analytics foundation migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"UPDATE usage_records SET estimated_cost", "DROP TABLE", "DELETE FROM"} {
		if strings.Contains(strings.ToUpper(joined), strings.ToUpper(forbidden)) {
			t.Fatalf("admin analytics foundation migration must not include %q", forbidden)
		}
	}
}

func TestImageAssetsMigrationStoresMetadataOnly(t *testing.T) {
	statement := findCreateTableStatement(t, "image_assets")
	for _, required := range []string{
		"object_key VARCHAR(512) NOT NULL",
		"thumbnail_object_key VARCHAR(512) NULL",
		"mime_type VARCHAR(128) NOT NULL",
		"size_bytes BIGINT UNSIGNED NOT NULL",
		"sha256 CHAR(64) NOT NULL",
		"deleted_at DATETIME(3) NULL",
		"KEY idx_image_assets_tenant_project_created (tenant_id, project_id, created_at)",
		"KEY idx_image_assets_tenant_project_kind (tenant_id, project_id, kind)",
		"KEY idx_image_assets_tenant_favorite (tenant_id, is_favorite)",
		"KEY idx_image_assets_tenant_deleted (tenant_id, deleted_at)",
	} {
		if !strings.Contains(statement, required) {
			t.Fatalf("image_assets migration missing %q", required)
		}
	}
	for _, forbidden := range []string{" BLOB", " LONGBLOB", " MEDIUMBLOB", " TINYBLOB", "base64"} {
		if strings.Contains(strings.ToLower(statement), strings.ToLower(forbidden)) {
			t.Fatalf("image_assets migration must not store image bytes: found %q", forbidden)
		}
	}
}

func TestImageAssetsPurgeMarkerMigrationIsTenantScopedAndBatchFriendly(t *testing.T) {
	statements := findMigrationStatements(t, "202605240001_image_asset_purge_marker")
	joined := strings.Join(statements, "\n")
	for _, required := range []string{
		"ALTER TABLE image_assets ADD COLUMN purged_at DATETIME(3) NULL",
		"CREATE INDEX idx_image_assets_tenant_deleted_purged ON image_assets (tenant_id, deleted_at, purged_at)",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("image asset purge marker migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"DROP TABLE", "DELETE FROM image_assets", "storage_quota", "retention_days"} {
		if strings.Contains(strings.ToUpper(joined), strings.ToUpper(forbidden)) {
			t.Fatalf("image asset purge marker migration must not include %q", forbidden)
		}
	}
}

func TestImageAssetModelIncludesNullablePurgedAt(t *testing.T) {
	field, ok := reflect.TypeOf(ImageAsset{}).FieldByName("PurgedAt")
	if !ok {
		t.Fatal("ImageAsset model missing PurgedAt field")
	}
	if field.Type.String() != "*time.Time" {
		t.Fatalf("PurgedAt type = %s, want *time.Time", field.Type.String())
	}
	tag := string(field.Tag.Get("gorm"))
	if !strings.Contains(tag, "datetime(3)") {
		t.Fatalf("PurgedAt gorm tag = %q, want datetime(3)", tag)
	}
}

func TestStorageQuotaReservationMigrationIsTenantScopedAndInternal(t *testing.T) {
	statements := findMigrationStatements(t, "202605270001_storage_quota_reservations")
	joined := strings.Join(statements, "\n")
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS storage_quota_counters",
		"tenant_id VARCHAR(36) NOT NULL",
		"used_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"reserved_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"UNIQUE KEY uk_storage_quota_counters_tenant (tenant_id)",
		"CONSTRAINT fk_storage_quota_counters_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)",
		"CREATE TABLE IF NOT EXISTS storage_quota_reservations",
		"finalized_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"KEY idx_storage_quota_reservations_tenant_status (tenant_id, status)",
		"KEY idx_storage_quota_reservations_tenant_expires (tenant_id, expires_at)",
		"CONSTRAINT fk_storage_quota_reservations_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)",
		"image_assets remains reconciliation truth",
		"never expose IDs in API responses or audit metadata",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("storage quota reservation migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"DROP TABLE", "DELETE FROM image_assets", "bucket", "object_key", "minio"} {
		if strings.Contains(strings.ToLower(joined), strings.ToLower(forbidden)) {
			t.Fatalf("storage quota reservation migration must not include %q", forbidden)
		}
	}
}

func TestSystemSettingsMigrationIsTenantScopedGenericJSON(t *testing.T) {
	statement := findCreateTableStatement(t, "system_settings")
	for _, required := range []string{
		"tenant_id VARCHAR(36) NOT NULL",
		"`key` VARCHAR(128) NOT NULL",
		"value_json JSON NOT NULL",
		"UNIQUE KEY uk_system_settings_tenant_key (tenant_id, `key`)",
		"CONSTRAINT fk_system_settings_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)",
		"Tenant-scoped system settings stored as JSON by active key",
	} {
		if !strings.Contains(statement, required) {
			t.Fatalf("system_settings migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"default_provider_id", "default_model_id", "tenant_concurrency", "storage_quota", "log_retention"} {
		if strings.Contains(strings.ToLower(statement), forbidden) {
			t.Fatalf("system_settings migration must not include deferred setting %q", forbidden)
		}
	}
}

func TestRunMigrationsSerializesConcurrentExecutors(t *testing.T) {
	db := openMigrationTestDB(t)
	migration := Migration{
		ID:   "test_concurrent_executor",
		Name: "test concurrent executor",
		Statements: []string{
			"CREATE TABLE IF NOT EXISTS ddl_runs (id INTEGER PRIMARY KEY AUTOINCREMENT)",
			"INSERT INTO ddl_runs DEFAULT VALUES",
		},
		Checks: []SchemaCheck{
			schemaCheck("ddl_runs", []string{"id"}, nil),
		},
	}

	var lock sync.Mutex
	var lockEntries int64
	lockRunner := func(_ context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
		lock.Lock()
		defer lock.Unlock()
		atomic.AddInt64(&lockEntries, 1)
		return fn(db)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			errs <- runMigrations(context.Background(), db, []Migration{migration}, lockRunner)
		}()
	}
	close(start)

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("runMigrations returned error: %v", err)
		}
	}

	var ddlRuns int64
	if err := db.Table("ddl_runs").Count(&ddlRuns).Error; err != nil {
		t.Fatalf("count ddl runs: %v", err)
	}
	if ddlRuns != 1 {
		t.Fatalf("ddl run count = %d, want 1", ddlRuns)
	}
	var appliedRows int64
	if err := db.Table("schema_migrations").Where("id = ?", migration.ID).Count(&appliedRows).Error; err != nil {
		t.Fatalf("count schema migration rows: %v", err)
	}
	if appliedRows != 1 {
		t.Fatalf("schema migration rows = %d, want 1", appliedRows)
	}
	if got := atomic.LoadInt64(&lockEntries); got != 2 {
		t.Fatalf("lock entries = %d, want 2", got)
	}
}

func TestRunMigrationsRecoversInterruptedIncrementalDDL(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := db.Exec("CREATE TABLE image_assets (id TEXT PRIMARY KEY, tenant_id TEXT, deleted_at DATETIME)").Error; err != nil {
		t.Fatalf("create image_assets fixture: %v", err)
	}

	failedMigration := Migration{
		ID:   "test_interrupted_incremental_ddl",
		Name: "test interrupted incremental ddl",
		Statements: []string{
			"ALTER TABLE image_assets ADD COLUMN purged_at DATETIME(3) NULL",
			"CREATE INDEX idx_image_assets_tenant_deleted_purged ON missing_image_assets (tenant_id, deleted_at, purged_at)",
		},
		Checks: []SchemaCheck{
			schemaCheck("image_assets", []string{"purged_at"}, []string{"idx_image_assets_tenant_deleted_purged"}),
		},
	}

	err := runMigrations(context.Background(), db, []Migration{failedMigration}, noMigrationLock)
	if err == nil {
		t.Fatal("runMigrations returned nil error for interrupted migration")
	}
	if !strings.Contains(err.Error(), "apply migration test_interrupted_incremental_ddl") {
		t.Fatalf("runMigrations error = %q, want migration execution context", err.Error())
	}
	if hasColumn, err := columnExists(db, "image_assets", "purged_at"); err != nil || !hasColumn {
		t.Fatalf("purged_at column exists = %v, err = %v, want true nil", hasColumn, err)
	}
	if hasIndex, err := indexExists(db, "image_assets", "idx_image_assets_tenant_deleted_purged"); err != nil || hasIndex {
		t.Fatalf("partial index exists = %v, err = %v, want false nil", hasIndex, err)
	}
	if applied, err := migrationApplied(db, failedMigration.ID); err != nil || applied {
		t.Fatalf("failed migration applied = %v, err = %v, want false nil", applied, err)
	}

	recoveryMigration := failedMigration
	recoveryMigration.Statements = []string{
		"ALTER TABLE image_assets ADD COLUMN purged_at DATETIME(3) NULL",
		"CREATE INDEX idx_image_assets_tenant_deleted_purged ON image_assets (tenant_id, deleted_at, purged_at)",
	}
	if err := runMigrations(context.Background(), db, []Migration{recoveryMigration}, noMigrationLock); err != nil {
		t.Fatalf("recovery runMigrations returned error: %v", err)
	}
	if hasIndex, err := indexExists(db, "image_assets", "idx_image_assets_tenant_deleted_purged"); err != nil || !hasIndex {
		t.Fatalf("recovered index exists = %v, err = %v, want true nil", hasIndex, err)
	}
	if applied, err := migrationApplied(db, recoveryMigration.ID); err != nil || !applied {
		t.Fatalf("recovered migration applied = %v, err = %v, want true nil", applied, err)
	}
}

func TestRunMigrationsSkipsExistingColumnIndexAndTable(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := db.Exec("CREATE TABLE image_assets (id TEXT PRIMARY KEY, purged_at DATETIME(3) NULL)").Error; err != nil {
		t.Fatalf("create image_assets fixture: %v", err)
	}
	if err := db.Exec("CREATE INDEX idx_image_assets_tenant_deleted_purged ON image_assets (purged_at)").Error; err != nil {
		t.Fatalf("create existing index fixture: %v", err)
	}

	migration := Migration{
		ID:   "test_existing_objects",
		Name: "test existing objects",
		Statements: []string{
			"CREATE TABLE IF NOT EXISTS image_assets (id TEXT PRIMARY KEY)",
			"ALTER TABLE image_assets ADD COLUMN purged_at DATETIME(3) NULL",
			"CREATE INDEX idx_image_assets_tenant_deleted_purged ON image_assets (purged_at)",
		},
		Checks: []SchemaCheck{
			schemaCheck("image_assets", []string{"id", "purged_at"}, []string{"idx_image_assets_tenant_deleted_purged"}),
		},
	}

	if err := runMigrations(context.Background(), db, []Migration{migration}, noMigrationLock); err != nil {
		t.Fatalf("runMigrations returned error: %v", err)
	}
	if err := runMigrations(context.Background(), db, []Migration{migration}, noMigrationLock); err != nil {
		t.Fatalf("second runMigrations returned error: %v", err)
	}
}

func TestRunMigrationsFailsClosedForAppliedButIncompleteSchema(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := ensureSchemaMigrations(db); err != nil {
		t.Fatalf("ensure schema migrations: %v", err)
	}
	if err := db.Exec("CREATE TABLE image_assets (id TEXT PRIMARY KEY, purged_at DATETIME(3) NULL)").Error; err != nil {
		t.Fatalf("create partial image_assets fixture: %v", err)
	}
	migration := Migration{
		ID:   "test_applied_incomplete_schema",
		Name: "test applied incomplete schema",
		Statements: []string{
			"ALTER TABLE image_assets ADD COLUMN purged_at DATETIME(3) NULL",
			"CREATE INDEX idx_image_assets_tenant_deleted_purged ON image_assets (purged_at)",
		},
		Checks: []SchemaCheck{
			schemaCheck("image_assets", []string{"purged_at"}, []string{"idx_image_assets_tenant_deleted_purged"}),
		},
	}
	if err := db.Table("schema_migrations").Create(map[string]any{"id": migration.ID, "name": migration.Name}).Error; err != nil {
		t.Fatalf("insert applied migration fixture: %v", err)
	}

	err := runMigrations(context.Background(), db, []Migration{migration}, noMigrationLock)
	if err == nil {
		t.Fatal("runMigrations returned nil error for applied incomplete schema")
	}
	if !strings.Contains(err.Error(), "missing index idx_image_assets_tenant_deleted_purged on image_assets") {
		t.Fatalf("runMigrations error = %q, want missing index failure", err.Error())
	}
}

func TestSQLiteMigrationPathDoesNotUseMySQLAdvisoryLock(t *testing.T) {
	db := openMigrationTestDB(t)
	called := false
	if err := withMigrationLock(context.Background(), db, func(*gorm.DB) error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withMigrationLock returned error on sqlite path: %v", err)
	}
	if !called {
		t.Fatal("withMigrationLock did not execute callback on sqlite path")
	}
}

func TestMySQLMigrationLockFailureIsFailClosedAndSanitized(t *testing.T) {
	db := openNamedMigrationTestDB(t, "mysql")

	err := acquireMySQLMigrationLock(context.Background(), db)
	if err == nil {
		t.Fatal("acquireMySQLMigrationLock returned nil error on sqlite-backed mysql path")
	}
	if got := err.Error(); got != "acquire migration lock: database operation failed" {
		t.Fatalf("acquireMySQLMigrationLock error = %q", got)
	}
	assertNoSensitiveMigrationErrorText(t, err.Error())
}

func TestMySQLMigrationLockWrapperDoesNotRunMigrationsWhenLockFails(t *testing.T) {
	db := openNamedMigrationTestDB(t, "mysql")
	called := false

	err := withMigrationLock(context.Background(), db, func(*gorm.DB) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("withMigrationLock returned nil error when lock acquisition failed")
	}
	if called {
		t.Fatal("withMigrationLock executed callback after lock acquisition failed")
	}
	if got := err.Error(); got != "acquire migration lock: database operation failed" {
		t.Fatalf("withMigrationLock error = %q", got)
	}
}

func TestMySQLMigrationLockReleaseFailureIsFailClosedAndSanitized(t *testing.T) {
	db := openNamedMigrationTestDB(t, "mysql")

	err := releaseMySQLMigrationLock(context.Background(), db)
	if err == nil {
		t.Fatal("releaseMySQLMigrationLock returned nil error on sqlite-backed mysql path")
	}
	if got := err.Error(); got != "release migration lock: database operation failed" {
		t.Fatalf("releaseMySQLMigrationLock error = %q", got)
	}
	assertNoSensitiveMigrationErrorText(t, err.Error())
}

func TestMySQLMigrationLockContextCancelIsExplicit(t *testing.T) {
	db := openNamedMigrationTestDB(t, "mysql")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := acquireMySQLMigrationLock(ctx, db)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireMySQLMigrationLock error = %v, want context.Canceled", err)
	}
}

func TestMigrationDatabaseErrorsDoNotExposeSensitiveText(t *testing.T) {
	err := migrationDatabaseError(
		"apply migration test_secret_redaction",
		errors.New("mysql://user:db-password@tcp(localhost:3306)/studio Authorization=Bearer jwt Cookie=session Provider-Key=sk-secret"),
	)
	assertNoSensitiveMigrationErrorText(t, err.Error())
	if got := err.Error(); got != "apply migration test_secret_redaction: database operation failed" {
		t.Fatalf("migrationDatabaseError = %q", got)
	}
}

func findCreateTableStatement(t *testing.T, table string) string {
	t.Helper()

	needle := "CREATE TABLE IF NOT EXISTS " + table
	for _, migration := range migrations {
		for _, statement := range migration.Statements {
			if strings.Contains(statement, needle) {
				return statement
			}
		}
	}

	t.Fatalf("missing create table statement for %s", table)
	return ""
}

func findMigrationStatements(t *testing.T, id string) []string {
	t.Helper()

	for _, migration := range migrations {
		if migration.ID == id {
			return migration.Statements
		}
	}

	t.Fatalf("missing migration %s", id)
	return nil
}

func noMigrationLock(_ context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
	return fn(db)
}

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openNamedMigrationTestDB(t, "sqlite")
}

func openNamedMigrationTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()

	dialector := gorm.Dialector(sqlite.Open(":memory:"))
	if name != "sqlite" {
		dialector = namedDialector{Dialector: dialector, name: name}
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open migration test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access migration test db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db
}

type namedDialector struct {
	gorm.Dialector
	name string
}

func (d namedDialector) Name() string {
	return d.name
}

func assertNoSensitiveMigrationErrorText(t *testing.T, message string) {
	t.Helper()

	for _, forbidden := range []string{
		"db-password",
		"mysql://",
		"Authorization",
		"Bearer",
		"jwt",
		"Cookie",
		"Provider-Key",
		"sk-secret",
	} {
		if strings.Contains(message, forbidden) {
			t.Fatalf("migration error leaked sensitive text %q in %q", forbidden, message)
		}
	}
}
