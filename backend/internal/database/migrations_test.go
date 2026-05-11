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
