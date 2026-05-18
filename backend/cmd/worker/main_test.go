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

func TestWorkerStartupRejectsPlaceholderProductionSecrets(t *testing.T) {
	tests := []struct {
		name             string
		jwtSigningSecret string
		apiKeyEncryption string
		wantErrorMessage string
	}{
		{
			name:             "placeholder JWT signing secret",
			jwtSigningSecret: "",
			apiKeyEncryption: "0123456789abcdef0123456789abcdef",
			wantErrorMessage: "invalid JWT_SIGNING_SECRET: placeholder secret is not allowed in production",
		},
		{
			name:             "placeholder API key encryption secret",
			jwtSigningSecret: "0123456789abcdef0123456789abcdef",
			apiKeyEncryption: "",
			wantErrorMessage: "invalid API_KEY_ENCRYPTION_KEY: placeholder secret is not allowed in production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_ENV", "production")
			t.Setenv("JWT_SIGNING_SECRET", tt.jwtSigningSecret)
			t.Setenv("API_KEY_ENCRYPTION_KEY", tt.apiKeyEncryption)

			_, err := loadStartupConfig()
			if err == nil {
				t.Fatal("loadStartupConfig returned nil error for placeholder production secret")
			}
			if got := err.Error(); got != tt.wantErrorMessage {
				t.Fatalf("loadStartupConfig returned unexpected error: %q", got)
			}
		})
	}
}
