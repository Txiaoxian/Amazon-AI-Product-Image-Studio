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
