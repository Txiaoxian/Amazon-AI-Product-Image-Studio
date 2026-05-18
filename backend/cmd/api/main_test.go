package main

import (
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
	router := newRouter(config.Config{}, slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)

	for _, path := range []string{"/healthz", "/api/v1/healthz"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}
}

func TestLoadStartupConfigRejectsPlaceholderProductionSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SIGNING_SECRET", "")
	t.Setenv("API_KEY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")

	_, err := loadStartupConfig()
	if err == nil {
		t.Fatal("loadStartupConfig returned nil error for placeholder JWT signing secret")
	}
	if got := err.Error(); got != "invalid JWT_SIGNING_SECRET: placeholder secret is not allowed in production" {
		t.Fatalf("loadStartupConfig error = %q", got)
	}
}
