package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkerHealthcheckFileUsesDefault(t *testing.T) {
	t.Setenv("WORKER_HEALTHCHECK_FILE", "")

	if got := workerHealthcheckFile(); got != defaultWorkerHealthcheckFile {
		t.Fatalf("workerHealthcheckFile() = %q, want %q", got, defaultWorkerHealthcheckFile)
	}
}

func TestWorkerHealthcheckFileTrimsConfiguredPath(t *testing.T) {
	t.Setenv("WORKER_HEALTHCHECK_FILE", "  /tmp/custom-worker-ready  ")

	if got := workerHealthcheckFile(); got != "/tmp/custom-worker-ready" {
		t.Fatalf("workerHealthcheckFile() = %q, want /tmp/custom-worker-ready", got)
	}
}

func TestWorkerReadinessFileLifecycle(t *testing.T) {
	readyFile := filepath.Join(t.TempDir(), "health", "worker-ready")

	if err := markWorkerReady(readyFile); err != nil {
		t.Fatalf("markWorkerReady returned error: %v", err)
	}

	content, err := os.ReadFile(readyFile)
	if err != nil {
		t.Fatalf("read readiness file: %v", err)
	}
	if string(content) != "ready\n" {
		t.Fatalf("readiness file content = %q, want ready newline", string(content))
	}

	if err := removeWorkerReady(readyFile); err != nil {
		t.Fatalf("removeWorkerReady returned error: %v", err)
	}
	if _, err := os.Stat(readyFile); !os.IsNotExist(err) {
		t.Fatalf("readiness file still exists or unexpected stat error: %v", err)
	}

	if err := removeWorkerReady(readyFile); err != nil {
		t.Fatalf("second removeWorkerReady returned error: %v", err)
	}
}

func TestWorkerStartupRejectsUnsafeProductionConfig(t *testing.T) {
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
			name:             "placeholder MinIO secret key",
			key:              "MINIO_SECRET_KEY",
			value:            "prod-change-me-minio-secret",
			wantErrorMessage: "invalid MINIO_SECRET_KEY: placeholder value is not allowed in production",
		},
		{
			name:             "insecure cookie",
			key:              "COOKIE_SECURE",
			value:            "false",
			wantErrorMessage: "invalid COOKIE_SECURE: must be true in production",
		},
		{
			name:             "localhost CORS origin",
			key:              "CORS_ALLOWED_ORIGINS",
			value:            "https://localhost",
			wantErrorMessage: "invalid CORS_ALLOWED_ORIGINS: localhost, loopback, private, and link-local origins are not allowed in production",
		},
		{
			name:             "non-default CSRF header",
			key:              "CSRF_HEADER_NAME",
			value:            "X-Test-CSRF",
			wantErrorMessage: "invalid CSRF_HEADER_NAME: must be X-CSRF-Token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidWorkerProductionEnv(t)
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

func setValidWorkerProductionEnv(t *testing.T) {
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
