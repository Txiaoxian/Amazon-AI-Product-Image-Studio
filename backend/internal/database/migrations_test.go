package database

import (
	"strings"
	"testing"
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
			if !strings.Contains(statement, "CREATE TABLE IF NOT EXISTS") {
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

func TestSystemSettingsMigrationIsTenantScopedAndUploadPolicyOnly(t *testing.T) {
	statement := findCreateTableStatement(t, "system_settings")
	for _, required := range []string{
		"tenant_id VARCHAR(36) NOT NULL",
		"`key` VARCHAR(128) NOT NULL",
		"value_json JSON NOT NULL",
		"UNIQUE KEY uk_system_settings_tenant_key (tenant_id, `key`)",
		"CONSTRAINT fk_system_settings_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)",
		"first active key is upload_policy only",
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
