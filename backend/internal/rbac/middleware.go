package rbac

import (
	"net/http"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/gin-gonic/gin"
)

type ObjectAuthorizer interface {
	AuthorizeObject(c *gin.Context, principal auth.Principal, permission string, objectType string, objectID string) bool
}

func RequirePermissions(permissions ...string) gin.HandlerFunc {
	required := append([]string(nil), permissions...)
	return func(c *gin.Context) {
		principal, ok := auth.PrincipalFromGin(c)
		if !ok {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}

		for _, permission := range required {
			if !principal.HasPermission(permission) {
				httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
				return
			}
		}

		c.Next()
	}
}

func RequireObjectPermission(permission string, objectType string, objectIDParam string, authorizer ObjectAuthorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := auth.PrincipalFromGin(c)
		if !ok {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}
		if authorizer == nil || !principal.HasPermission(permission) {
			httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
			return
		}

		if !authorizer.AuthorizeObject(c, principal, permission, objectType, c.Param(objectIDParam)) {
			httpx.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Forbidden.", nil)
			return
		}

		c.Next()
	}
}
