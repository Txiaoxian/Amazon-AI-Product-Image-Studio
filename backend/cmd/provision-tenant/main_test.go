package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"gorm.io/gorm"
)

func TestRunDefaultsToDryRunWithoutOpeningDatabase(t *testing.T) {
	deps, stdout, stderr := testDependencies()
	var opened int
	var provisioned int
	deps.openDatabase = func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
		opened++
		return nil, errors.New("database must not be opened")
	}
	deps.provisionTenant = func(context.Context, *gorm.DB, auth.TenantProvisioningInput) (auth.TenantProvisioningResult, error) {
		provisioned++
		return auth.TenantProvisioningResult{}, errors.New("tenant must not be provisioned")
	}

	if code := run(context.Background(), nil, deps); code != 0 {
		t.Fatalf("run exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if opened != 0 || provisioned != 0 {
		t.Fatalf("dry-run opened = %d provisioned = %d, want zero", opened, provisioned)
	}
	if output := stdout.String(); output != "tenant provisioning dry-run validated; no changes applied\n" {
		t.Fatalf("stdout = %q", output)
	}
}

func TestRunApplyRequiresExplicitConfirmationBeforeOpeningDatabase(t *testing.T) {
	deps, stdout, stderr := testDependencies()
	deps.getenv = envLookup(map[string]string{
		tenantNameEnv:       "Tenant",
		adminEmailEnv:       "admin@example.com",
		adminDisplayNameEnv: "Tenant Admin",
		adminPasswordEnv:    "strong-password",
	})
	var opened int
	deps.openDatabase = func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
		opened++
		return nil, errors.New("database must not be opened")
	}

	if code := run(context.Background(), []string{"--apply"}, deps); code != 1 {
		t.Fatalf("run exit code = %d, want 1", code)
	}
	if opened != 0 {
		t.Fatalf("database opened = %d times, want zero", opened)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if output := stderr.String(); output != "tenant provisioning refused: explicit apply confirmation is required\n" {
		t.Fatalf("stderr = %q", output)
	}
}

func TestRunApplyProvisionsTenantAndPrintsOnlyTenantID(t *testing.T) {
	deps, stdout, stderr := testDependencies()
	deps.getenv = envLookup(validApplyEnvironment())
	db := &gorm.DB{}
	var migrations int
	var closes int
	deps.openDatabase = func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
		return db, nil
	}
	deps.runMigrations = func(context.Context, *gorm.DB) error {
		migrations++
		return nil
	}
	deps.closeDatabase = func(*gorm.DB) error {
		closes++
		return nil
	}
	deps.provisionTenant = func(_ context.Context, actualDB *gorm.DB, input auth.TenantProvisioningInput) (auth.TenantProvisioningResult, error) {
		if actualDB != db {
			t.Fatalf("provision database = %p, want %p", actualDB, db)
		}
		if input.TenantName != "Tenant" || input.AdminEmail != "admin@example.com" || input.AdminDisplayName != "Tenant Admin" || input.AdminPassword != "strong-password" {
			t.Fatal("provision input does not match expected values")
		}
		return auth.TenantProvisioningResult{TenantID: "tenant-public-id"}, nil
	}

	if code := run(context.Background(), []string{"--apply"}, deps); code != 0 {
		t.Fatalf("run exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if migrations != 1 || closes != 1 {
		t.Fatalf("migrations = %d closes = %d, want 1 each", migrations, closes)
	}
	if output := stdout.String(); output != "tenant provisioning applied tenant_id=tenant-public-id\n" {
		t.Fatalf("stdout = %q", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunApplyReadsPasswordFromStdinWhenEnvironmentIsUnset(t *testing.T) {
	deps, _, stderr := testDependencies()
	env := validApplyEnvironment()
	delete(env, adminPasswordEnv)
	deps.getenv = envLookup(env)
	deps.stdin = strings.NewReader("stdin-strong-password\n")
	deps.openDatabase = func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
		return &gorm.DB{}, nil
	}
	deps.provisionTenant = func(_ context.Context, _ *gorm.DB, input auth.TenantProvisioningInput) (auth.TenantProvisioningResult, error) {
		if input.AdminPassword != "stdin-strong-password" {
			t.Fatal("admin password does not match stdin value")
		}
		return auth.TenantProvisioningResult{TenantID: "tenant-public-id"}, nil
	}

	if code := run(context.Background(), []string{"--apply"}, deps); code != 0 {
		t.Fatalf("run exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
}

func TestRunApplySanitizesFailureOutput(t *testing.T) {
	sensitiveError := errors.New("sensitive-password $2a$hash JWT CSRF Cookie admin@example.com SQL INSERT")
	tests := []struct {
		name      string
		configure func(*commandDependencies)
	}{
		{
			name: "config load",
			configure: func(deps *commandDependencies) {
				deps.loadConfig = func() (config.Config, error) {
					return config.Config{}, sensitiveError
				}
			},
		},
		{
			name: "database open",
			configure: func(deps *commandDependencies) {
				deps.openDatabase = func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
					return nil, sensitiveError
				}
			},
		},
		{
			name: "migration",
			configure: func(deps *commandDependencies) {
				deps.runMigrations = func(context.Context, *gorm.DB) error {
					return sensitiveError
				}
			},
		},
		{
			name: "provisioning",
			configure: func(deps *commandDependencies) {
				deps.provisionTenant = func(context.Context, *gorm.DB, auth.TenantProvisioningInput) (auth.TenantProvisioningResult, error) {
					return auth.TenantProvisioningResult{}, sensitiveError
				}
			},
		},
		{
			name: "database close",
			configure: func(deps *commandDependencies) {
				deps.closeDatabase = func(*gorm.DB) error {
					return sensitiveError
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, stdout, stderr := testDependencies()
			env := validApplyEnvironment()
			env[adminPasswordEnv] = "sensitive-password"
			deps.getenv = envLookup(env)
			deps.openDatabase = func(context.Context, config.DatabaseConfig) (*gorm.DB, error) {
				return &gorm.DB{}, nil
			}
			deps.provisionTenant = func(context.Context, *gorm.DB, auth.TenantProvisioningInput) (auth.TenantProvisioningResult, error) {
				return auth.TenantProvisioningResult{TenantID: "tenant-public-id"}, nil
			}
			tt.configure(&deps)

			if code := run(context.Background(), []string{"--apply"}, deps); code != 1 {
				t.Fatalf("run exit code = %d, want 1", code)
			}
			output := stdout.String() + stderr.String()
			for _, forbidden := range []string{"sensitive-password", "$2a$hash", "JWT", "CSRF", "Cookie", "admin@example.com", "SQL", "INSERT"} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("output %q contains forbidden marker %q", output, forbidden)
				}
			}
			if output != "tenant provisioning failed\n" {
				t.Fatalf("output = %q", output)
			}
		})
	}
}

func testDependencies() (commandDependencies, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	deps := defaultDependencies()
	deps.getenv = envLookup(validDryRunEnvironment())
	deps.stdin = strings.NewReader("")
	deps.stdout = stdout
	deps.stderr = stderr
	deps.loadConfig = func() (config.Config, error) {
		return config.Config{}, nil
	}
	deps.runMigrations = func(context.Context, *gorm.DB) error {
		return nil
	}
	deps.closeDatabase = func(*gorm.DB) error {
		return nil
	}
	return deps, stdout, stderr
}

func validDryRunEnvironment() map[string]string {
	return map[string]string{
		tenantNameEnv:       "Tenant",
		adminEmailEnv:       "admin@example.com",
		adminDisplayNameEnv: "Tenant Admin",
		adminPasswordEnv:    "strong-password",
	}
}

func validApplyEnvironment() map[string]string {
	env := validDryRunEnvironment()
	env[confirmationEnv] = requiredConfirmation
	return env
}

func envLookup(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
