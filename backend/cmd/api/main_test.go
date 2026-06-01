package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestNewRouterServesHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newRouter(context.Background(), config.Config{AppEnv: "test"}, slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)

	for _, path := range []string{"/healthz", "/api/v1/healthz"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}
}

func TestAPIStartupRejectsUnsafeProductionConfig(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		value            string
		wantErrorMessage string
	}{
		{
			name:             "placeholder JWT signing secret",
			key:              "JWT_SIGNING_SECRET",
			value:            "",
			wantErrorMessage: "invalid JWT_SIGNING_SECRET: placeholder secret is not allowed in production",
		},
		{
			name:             "placeholder API key encryption secret",
			key:              "API_KEY_ENCRYPTION_KEY",
			value:            "",
			wantErrorMessage: "invalid API_KEY_ENCRYPTION_KEY: placeholder secret is not allowed in production",
		},
		{
			name:             "placeholder MySQL password",
			key:              "MYSQL_PASSWORD",
			value:            "prod-change-me-mysql",
			wantErrorMessage: "invalid MYSQL_PASSWORD: placeholder value is not allowed in production",
		},
		{
			name:             "insecure cookie",
			key:              "COOKIE_SECURE",
			value:            "false",
			wantErrorMessage: "invalid COOKIE_SECURE: must be true in production",
		},
		{
			name:             "HTTP CORS origin",
			key:              "CORS_ALLOWED_ORIGINS",
			value:            "http://studio.example.com",
			wantErrorMessage: "invalid CORS_ALLOWED_ORIGINS: only https origins are allowed in production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidAPIProductionEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := loadStartupConfig()
			if err == nil {
				t.Fatal("loadStartupConfig returned nil error for unsafe production config")
			}
			if got := err.Error(); got != tt.wantErrorMessage {
				t.Fatalf("loadStartupConfig returned unexpected error: %q", got)
			}
		})
	}
}

func TestRunDatabaseStartupTasksPropagatesBuiltInRoleReconciliationFailure(t *testing.T) {
	var calls []string
	reconciliationErr := errors.New("reconciliation failed")
	err := runDatabaseStartupTasks(
		context.Background(),
		nil,
		config.DatabaseConfig{MigrationsMode: "startup-gate"},
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		func(context.Context, *gorm.DB) error {
			calls = append(calls, "migrations")
			return nil
		},
		func(context.Context, *gorm.DB) error {
			calls = append(calls, "reconciliation")
			return reconciliationErr
		},
	)
	if !errors.Is(err, errBuiltInRoleReconciliationFailed) {
		t.Fatalf("runDatabaseStartupTasks error = %v, want %v", err, errBuiltInRoleReconciliationFailed)
	}
	if errors.Is(err, reconciliationErr) {
		t.Fatalf("runDatabaseStartupTasks error = %v, must not expose underlying reconciliation error", err)
	}
	if want := []string{"migrations", "reconciliation"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("startup task calls = %v, want %v", calls, want)
	}
}

func TestRunDatabaseStartupTasksReconcilesWhenMigrationsDisabled(t *testing.T) {
	var calls []string
	err := runDatabaseStartupTasks(
		context.Background(),
		nil,
		config.DatabaseConfig{MigrationsMode: "disabled"},
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		func(context.Context, *gorm.DB) error {
			calls = append(calls, "migrations")
			return nil
		},
		func(context.Context, *gorm.DB) error {
			calls = append(calls, "reconciliation")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runDatabaseStartupTasks returned error: %v", err)
	}
	if want := []string{"reconciliation"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("startup task calls = %v, want %v", calls, want)
	}
}

func setValidAPIProductionEnv(t *testing.T) {
	t.Helper()

	for key, value := range map[string]string{
		"APP_ENV":                "production",
		"JWT_SIGNING_SECRET":     "0123456789abcdef0123456789abcdef",
		"API_KEY_ENCRYPTION_KEY": "abcdef0123456789abcdef0123456789",
		"MYSQL_PASSWORD":         "prod-mysql-password",
		"REDIS_PASSWORD":         "prod-redis-password",
		"MINIO_ACCESS_KEY":       "prod-minio-access",
		"MINIO_SECRET_KEY":       "prod-minio-secret",
		"COOKIE_SECURE":          "true",
		"CORS_ALLOWED_ORIGINS":   "https://studio.example.com",
	} {
		t.Setenv(key, value)
	}
}
