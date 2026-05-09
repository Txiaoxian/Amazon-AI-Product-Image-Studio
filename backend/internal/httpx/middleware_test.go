package httpx

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDGeneratesAndReturnsID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		JSON(c, http.StatusOK, map[string]string{"status": "ok"})
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	requestID := response.Header().Get(RequestIDHeader)
	if requestID == "" {
		t.Fatal("request id header is empty")
	}

	var body SuccessResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.RequestID != requestID {
		t.Fatalf("body requestId = %q, want %q", body.RequestID, requestID)
	}
}

func TestRequestIDUsesSafeInboundID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		JSON(c, http.StatusOK, nil)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set(RequestIDHeader, "req_existing")
	router.ServeHTTP(response, request)

	if response.Header().Get(RequestIDHeader) != "req_existing" {
		t.Fatalf("request id header = %q, want req_existing", response.Header().Get(RequestIDHeader))
	}
}

func TestRecoveryReturnsSanitizedError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery(slog.New(slog.NewJSONHandler(io.Discard, nil))))
	router.GET("/panic", func(c *gin.Context) {
		panic("do not leak this")
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}

	body := response.Body.String()
	if strings.Contains(body, "do not leak this") {
		t.Fatalf("response leaked panic value: %s", body)
	}
	if !strings.Contains(body, "INTERNAL_ERROR") {
		t.Fatalf("response missing INTERNAL_ERROR code: %s", body)
	}
}
