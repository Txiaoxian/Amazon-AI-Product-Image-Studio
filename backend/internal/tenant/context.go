package tenant

import (
	"context"
	"errors"
	"strings"
)

var ErrMissingTenantID = errors.New("tenant_id is required")

type Scope struct {
	tenantID string
}

func NewScope(tenantID string) (Scope, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return Scope{}, ErrMissingTenantID
	}

	return Scope{tenantID: tenantID}, nil
}

func (s Scope) ID() string {
	return s.tenantID
}

func (s Scope) Valid() bool {
	return s.tenantID != ""
}

type contextKey struct{}

func ContextWithScope(ctx context.Context, scope Scope) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !scope.Valid() {
		return nil, ErrMissingTenantID
	}

	return context.WithValue(ctx, contextKey{}, scope), nil
}

func ScopeFromContext(ctx context.Context) (Scope, bool) {
	if ctx == nil {
		return Scope{}, false
	}

	scope, ok := ctx.Value(contextKey{}).(Scope)
	return scope, ok && scope.Valid()
}
