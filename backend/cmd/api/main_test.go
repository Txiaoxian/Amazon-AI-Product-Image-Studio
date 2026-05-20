package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/gin-gonic/gin"
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

func TestAPIStartupRejectsPlaceholderProductionSecrets(t *testing.T) {
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
