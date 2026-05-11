package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/gin-gonic/gin"
)

func TestHandlerReturnsHealthyStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(httpx.RequestID())
	router.GET("/healthz", Handler("api"))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body struct {
		Data Status `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Data.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.Data.Status)
	}
	if body.Data.Service != "api" {
		t.Fatalf("service = %q, want api", body.Data.Service)
	}
}

func TestHandlerReturnsUnavailableWhenDependencyFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(httpx.RequestID())
	router.GET("/healthz", Handler("api", failingChecker{}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}

	var body struct {
		Data Status `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Data.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", body.Data.Status)
	}
	if body.Data.Dependencies["database"] != "unhealthy" {
		t.Fatalf("database dependency = %q, want unhealthy", body.Data.Dependencies["database"])
	}
}

type failingChecker struct{}

func (failingChecker) Name() string {
	return "database"
}

func (failingChecker) Check(context.Context) error {
	return errors.New("dependency unavailable")
}
