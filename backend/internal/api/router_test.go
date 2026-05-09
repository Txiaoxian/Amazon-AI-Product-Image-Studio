package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/gin-gonic/gin"
)

func TestNewRouterServesBaseAndVersionedHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterOptions{
		Config: config.Config{},
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})

	for _, path := range []string{"/healthz", "/api/v1/healthz"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}
}

func TestNewRouterAppliesConfiguredCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterOptions{
		Config: config.Config{
			API: config.APIConfig{
				CORSAllowedOrigins: []string{"https://studio.example.com"},
			},
		},
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	request.Header.Set("Origin", "https://studio.example.com")
	router.ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "https://studio.example.com" {
		t.Fatalf("CORS origin header = %q, want https://studio.example.com", response.Header().Get("Access-Control-Allow-Origin"))
	}
}
