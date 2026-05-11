package auth

import (
	"context"

	"github.com/gin-gonic/gin"
)

const (
	UserStatusActive   = "ACTIVE"
	TenantStatusActive = "ACTIVE"
	RoleStatusActive   = "ACTIVE"
)

type Principal struct {
	UserID      string
	TenantID    string
	Email       string
	DisplayName string
	Status      string
	CSRFToken   string
	Roles       []RoleInfo
	Permissions []string
}

type RoleInfo struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UserInfo struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName string  `json:"displayName"`
	Status      string  `json:"status"`
	LastLoginAt *string `json:"lastLoginAt"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type TenantInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type SessionResponse struct {
	User        UserInfo   `json:"user"`
	Tenant      TenantInfo `json:"tenant"`
	Roles       []RoleInfo `json:"roles"`
	Permissions []string   `json:"permissions"`
	CSRFToken   string     `json:"csrfToken,omitempty"`
}

type contextKey struct{}

const ginPrincipalKey = "auth.principal"

func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(contextKey{}).(Principal)
	return principal, ok && principal.UserID != "" && principal.TenantID != ""
}

func SetPrincipal(c *gin.Context, principal Principal) {
	c.Set(ginPrincipalKey, principal)
	c.Request = c.Request.WithContext(ContextWithPrincipal(c.Request.Context(), principal))
}

func PrincipalFromGin(c *gin.Context) (Principal, bool) {
	value, ok := c.Get(ginPrincipalKey)
	if ok {
		principal, ok := value.(Principal)
		if ok && principal.UserID != "" && principal.TenantID != "" {
			return principal, true
		}
	}

	return PrincipalFromContext(c.Request.Context())
}

func (p Principal) HasPermission(permission string) bool {
	for _, candidate := range p.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}
