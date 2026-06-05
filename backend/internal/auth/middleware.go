package auth

import (
	"errors"
	"net/http"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.db == nil {
			httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
			return
		}

		cookie, err := c.Request.Cookie(s.cfg.Auth.Cookie.Name)
		if err != nil || cookie.Value == "" {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}

		claims, err := parseToken(s.cfg.Auth.JWTSigningSecret, s.cfg.Auth.JWTIssuer, cookie.Value, s.now())
		if err != nil {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}

		var tenantRecord database.Tenant
		if err := s.db.WithContext(c.Request.Context()).
			Where("id = ? AND status = ?", claims.TenantID, TenantStatusActive).
			First(&tenantRecord).Error; err != nil {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}

		var userRecord database.User
		if err := s.db.WithContext(c.Request.Context()).
			Where("tenant_id = ? AND id = ? AND status = ?", claims.TenantID, claims.Subject, UserStatusActive).
			First(&userRecord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
				return
			}
			httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
			return
		}
		if userRecord.SessionVersion <= 0 || userRecord.SessionVersion != claims.SessionVersion {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}

		access, err := s.loadAccess(c.Request.Context(), s.db, claims.TenantID, claims.Subject)
		if err != nil {
			httpx.AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
			return
		}

		principal := Principal{
			UserID:         userRecord.ID,
			TenantID:       userRecord.TenantID,
			Email:          userRecord.Email,
			DisplayName:    userRecord.DisplayName,
			Status:         userRecord.Status,
			SessionVersion: userRecord.SessionVersion,
			CSRFToken:      claims.CSRFToken,
			Roles:          access.Roles,
			Permissions:    access.Permissions,
		}

		scope, err := tenant.NewScope(userRecord.TenantID)
		if err != nil {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}
		requestContext, err := tenant.ContextWithScope(c.Request.Context(), scope)
		if err != nil {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}
		c.Request = c.Request.WithContext(requestContext)
		c.Set("tenant.scope", scope)
		SetPrincipal(c, principal)

		c.Next()
	}
}

func (s *Service) RequireCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cfg.Auth.CSRF.Enabled || !stateChangingMethod(c.Request.Method) {
			c.Next()
			return
		}

		principal, ok := PrincipalFromGin(c)
		if !ok {
			httpx.AbortWithError(c, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication is required.", nil)
			return
		}

		headerToken := c.GetHeader(s.cfg.Auth.CSRF.HeaderName)
		csrfCookie, err := c.Request.Cookie(s.cfg.Auth.CSRF.CookieName)
		if err != nil || headerToken == "" || csrfCookie.Value == "" {
			httpx.AbortWithError(c, http.StatusForbidden, "CSRF_REQUIRED", "CSRF token is required.", nil)
			return
		}
		if headerToken != csrfCookie.Value || headerToken != principal.CSRFToken {
			httpx.AbortWithError(c, http.StatusForbidden, "CSRF_INVALID", "CSRF token is invalid.", nil)
			return
		}

		c.Next()
	}
}

func stateChangingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
