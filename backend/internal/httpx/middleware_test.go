package httpx

import (
	"bytes"
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

func TestSecurityHeadersAreSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(response, request)

	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", response.Header().Get("X-Content-Type-Options"))
	}
	if response.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", response.Header().Get("X-Frame-Options"))
	}
}

func TestCORSAllowsOnlyConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS([]string{"https://studio.example.com"}))
	router.GET("/test", func(c *gin.Context) {
		JSON(c, http.StatusOK, nil)
	})

	allowedResponse := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/test", nil)
	allowedRequest.Header.Set("Origin", "https://studio.example.com")
	router.ServeHTTP(allowedResponse, allowedRequest)

	if allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://studio.example.com" {
		t.Fatalf("allowed origin header = %q, want https://studio.example.com", allowedResponse.Header().Get("Access-Control-Allow-Origin"))
	}
	if allowedResponse.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("credentials header = %q, want true", allowedResponse.Header().Get("Access-Control-Allow-Credentials"))
	}

	blockedResponse := httptest.NewRecorder()
	blockedRequest := httptest.NewRequest(http.MethodGet, "/test", nil)
	blockedRequest.Header.Set("Origin", "https://evil.example.com")
	router.ServeHTTP(blockedResponse, blockedRequest)

	if blockedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("blocked origin unexpectedly received CORS header %q", blockedResponse.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSHandlesAllowedPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS([]string{"https://studio.example.com"}))
	router.GET("/test", func(c *gin.Context) {
		t.Fatal("preflight should not reach route handler")
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/test", nil)
	request.Header.Set("Origin", "https://studio.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "https://studio.example.com" {
		t.Fatalf("allowed origin header = %q, want https://studio.example.com", response.Header().Get("Access-Control-Allow-Origin"))
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, X-Request-ID, X-CSRF-Token" {
		t.Fatalf("allowed headers = %q, want fixed CSRF header", got)
	}
	if strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "X-Test-CSRF") {
		t.Fatal("preflight unexpectedly exposed configurable CSRF header alias")
	}
}

func TestAccessLogDoesNotLogSensitiveHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	router := gin.New()
	router.Use(RequestID())
	router.Use(AccessLog(log))
	router.GET("/test", func(c *gin.Context) {
		JSON(c, http.StatusOK, nil)
	})

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	request.Header.Set("Cookie", "session=secret-cookie")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	output := buf.String()
	if strings.Contains(output, "secret-token") || strings.Contains(output, "secret-cookie") {
		t.Fatalf("access log leaked sensitive header value: %s", output)
	}
}
