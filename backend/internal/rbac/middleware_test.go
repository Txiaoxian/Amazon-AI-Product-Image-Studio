package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestRequirePermissionsRejectsMissingPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin", func(c *gin.Context) {
		auth.SetPrincipal(c, auth.Principal{
			UserID:      "user-1",
			TenantID:    "tenant-1",
			Permissions: []string{"project:read"},
		})
	}, RequirePermissions("user:read"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestRequirePermissionsAllowsAllRequiredPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin", func(c *gin.Context) {
		auth.SetPrincipal(c, auth.Principal{
			UserID:      "user-1",
			TenantID:    "tenant-1",
			Permissions: []string{"user:read", "audit:read"},
		})
	}, RequirePermissions("user:read", "audit:read"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestRequirePermissionsRejectsUnauthenticatedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin", RequirePermissions("user:read"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
