package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestAuthLifecycleCookieCSRFAndAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newAuthRouteTestDB(t)
	router := NewRouter(RouterOptions{
		Config:   authRouteTestConfig("test"),
		Logger:   slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Database: db,
	})

	initResponse := performJSON(router, http.MethodPost, "/api/v1/auth/init-admin", map[string]string{
		"tenantName":  "Studio Tenant",
		"email":       "Admin@Example.com",
		"displayName": "Admin User",
		"password":    "initial-password-123",
	}, nil, nil)
	if initResponse.Code != http.StatusCreated {
		t.Fatalf("init-admin status = %d, want %d: %s", initResponse.Code, http.StatusCreated, initResponse.Body.String())
	}
	assertNoSensitiveFields(t, initResponse.Body.String())

	authCookie := findCookie(t, initResponse, "studio_auth")
	if !authCookie.HttpOnly {
		t.Fatal("auth cookie must be HttpOnly")
	}
	if authCookie.Value == "" {
		t.Fatal("auth cookie value is empty")
	}
	csrfCookie := findCookie(t, initResponse, "studio_csrf")
	if csrfCookie.HttpOnly {
		t.Fatal("CSRF cookie must be readable by the frontend")
	}
	if csrfCookie.Value == "" {
		t.Fatal("CSRF cookie value is empty")
	}

	initData := decodeData(t, initResponse)
	tenantID := nestedString(t, initData, "tenant", "id")
	if tenantID == "" {
		t.Fatal("init-admin response missing tenant.id")
	}
	if !containsString(asStringSlice(t, initData["permissions"]), "audit:read") {
		t.Fatal("admin permissions missing audit:read")
	}

	repeatedInitResponse := performJSON(router, http.MethodPost, "/api/v1/auth/init-admin", map[string]string{
		"tenantName":  "Another Tenant",
		"email":       "another@example.com",
		"displayName": "Another Admin",
		"password":    "another-password-123",
	}, nil, nil)
	if repeatedInitResponse.Code != http.StatusConflict {
		t.Fatalf("repeat init-admin status = %d, want %d", repeatedInitResponse.Code, http.StatusConflict)
	}

	unauthenticatedMe := performJSON(router, http.MethodGet, "/api/v1/me", nil, nil, nil)
	if unauthenticatedMe.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /me status = %d, want %d", unauthenticatedMe.Code, http.StatusUnauthorized)
	}

	failedLogin := performJSON(router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"tenantId": tenantID,
		"email":    "admin@example.com",
		"password": "wrong-password",
	}, nil, nil)
	if failedLogin.Code != http.StatusUnauthorized {
		t.Fatalf("failed login status = %d, want %d: %s", failedLogin.Code, http.StatusUnauthorized, failedLogin.Body.String())
	}
	if !strings.Contains(failedLogin.Body.String(), "Invalid email or password.") {
		t.Fatal("failed login must use the generic invalid credentials message")
	}

	loginResponse := performJSON(router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"tenantId": tenantID,
		"email":    "admin@example.com",
		"password": "initial-password-123",
	}, nil, nil)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", loginResponse.Code, http.StatusOK, loginResponse.Body.String())
	}
	assertNoSensitiveFields(t, loginResponse.Body.String())
	authCookie = findCookie(t, loginResponse, "studio_auth")
	csrfCookie = findCookie(t, loginResponse, "studio_csrf")

	meResponse := performJSON(router, http.MethodGet, "/api/v1/me", nil, []*http.Cookie{authCookie}, nil)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("/me status = %d, want %d: %s", meResponse.Code, http.StatusOK, meResponse.Body.String())
	}
	assertNoSensitiveFields(t, meResponse.Body.String())
	meData := decodeData(t, meResponse)
	if nestedString(t, meData, "user", "email") != "admin@example.com" {
		t.Fatalf("/me email = %q, want admin@example.com", nestedString(t, meData, "user", "email"))
	}

	noCSRFResponse := performJSON(router, http.MethodPatch, "/api/v1/me/password", map[string]string{
		"currentPassword": "initial-password-123",
		"newPassword":     "updated-password-123",
	}, []*http.Cookie{authCookie}, nil)
	if noCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("password change without CSRF status = %d, want %d", noCSRFResponse.Code, http.StatusForbidden)
	}

	badCSRFResponse := performJSON(router, http.MethodPatch, "/api/v1/me/password", map[string]string{
		"currentPassword": "initial-password-123",
		"newPassword":     "updated-password-123",
	}, []*http.Cookie{authCookie, csrfCookie}, map[string]string{"X-CSRF-Token": "bad-token"})
	if badCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("password change with bad CSRF status = %d, want %d", badCSRFResponse.Code, http.StatusForbidden)
	}

	passwordResponse := performJSON(router, http.MethodPatch, "/api/v1/me/password", map[string]string{
		"currentPassword": "initial-password-123",
		"newPassword":     "updated-password-123",
	}, []*http.Cookie{authCookie, csrfCookie}, map[string]string{"X-CSRF-Token": csrfCookie.Value})
	if passwordResponse.Code != http.StatusOK {
		t.Fatalf("password change status = %d, want %d: %s", passwordResponse.Code, http.StatusOK, passwordResponse.Body.String())
	}

	oldPasswordLogin := performJSON(router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"tenantId": tenantID,
		"email":    "admin@example.com",
		"password": "initial-password-123",
	}, nil, nil)
	if oldPasswordLogin.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d, want %d", oldPasswordLogin.Code, http.StatusUnauthorized)
	}

	newPasswordLogin := performJSON(router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"tenantId": tenantID,
		"email":    "admin@example.com",
		"password": "updated-password-123",
	}, nil, nil)
	if newPasswordLogin.Code != http.StatusOK {
		t.Fatalf("new password login status = %d, want %d", newPasswordLogin.Code, http.StatusOK)
	}
	authCookie = findCookie(t, newPasswordLogin, "studio_auth")
	csrfCookie = findCookie(t, newPasswordLogin, "studio_csrf")

	logoutResponse := performJSON(router, http.MethodPost, "/api/v1/auth/logout", nil, []*http.Cookie{authCookie, csrfCookie}, map[string]string{"X-CSRF-Token": csrfCookie.Value})
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d: %s", logoutResponse.Code, http.StatusOK, logoutResponse.Body.String())
	}
	clearedAuthCookie := findCookie(t, logoutResponse, "studio_auth")
	if clearedAuthCookie.MaxAge >= 0 {
		t.Fatal("logout must clear the auth cookie")
	}

	assertOperationLogs(t, db)
}

func TestProductionAuthCookieIsSecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newAuthRouteTestDB(t)
	router := NewRouter(RouterOptions{
		Config:   authRouteTestConfig("production"),
		Logger:   slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Database: db,
	})

	response := performJSON(router, http.MethodPost, "/api/v1/auth/init-admin", map[string]string{
		"tenantName":  "Studio Tenant",
		"email":       "admin@example.com",
		"displayName": "Admin User",
		"password":    "initial-password-123",
	}, nil, nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("init-admin status = %d, want %d", response.Code, http.StatusCreated)
	}
	if !findCookie(t, response, "studio_auth").Secure {
		t.Fatal("production auth cookie must be Secure")
	}
	if !findCookie(t, response, "studio_csrf").Secure {
		t.Fatal("production CSRF cookie must be Secure")
	}
}

func newAuthRouteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared&_loc=auto"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access sqlite test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	for _, statement := range authRouteTestSchema {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("migrate auth test database: %v", err)
		}
	}

	return db
}

var authRouteTestSchema = []string{
	`CREATE TABLE tenants (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		email TEXT NOT NULL,
		display_name TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		status TEXT NOT NULL,
		last_login_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE roles (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE permissions (
		id TEXT PRIMARY KEY,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE user_roles (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE role_permissions (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		permission_id TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE operation_logs (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		actor_user_id TEXT NULL,
		action TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		ip TEXT NOT NULL,
		user_agent TEXT NOT NULL,
		metadata_json TEXT NULL,
		created_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE projects (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		name TEXT NOT NULL,
		brand TEXT NOT NULL,
		asin TEXT NOT NULL,
		site TEXT NOT NULL,
		notes TEXT NULL,
		status TEXT NOT NULL,
		created_by TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		deleted_at TIMESTAMP NULL
	)`,
	`CREATE TABLE project_members (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`,
	`CREATE TABLE image_assets (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		category TEXT NOT NULL,
		filename TEXT NOT NULL,
		object_key TEXT NOT NULL,
		thumbnail_object_key TEXT NULL,
		mime_type TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		width INTEGER NOT NULL,
		height INTEGER NOT NULL,
		sha256 TEXT NOT NULL,
		is_favorite BOOLEAN NOT NULL,
		source_task_id TEXT NULL,
		created_by TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		deleted_at TIMESTAMP NULL
	)`,
}

func authRouteTestConfig(appEnv string) config.Config {
	return config.Config{
		AppEnv: appEnv,
		Auth: config.AuthConfig{
			JWTSigningSecret: "0123456789abcdef0123456789abcdef",
			JWTIssuer:        "auth-route-test",
			AccessTokenTTL:   time.Hour,
			Cookie: config.CookieConfig{
				Name:     "studio_auth",
				SameSite: "Lax",
			},
			CSRF: config.CSRFConfig{
				Enabled:    true,
				CookieName: "studio_csrf",
				HeaderName: "X-CSRF-Token",
			},
		},
	}
}

func performJSON(router http.Handler, method string, path string, body any, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		payload = bytes.NewReader(encoded)
	}

	request := httptest.NewRequest(method, path, payload)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func findCookie(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("missing cookie %s", name)
	return nil
}

func decodeData(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload.Data
}

func nestedString(t *testing.T, data map[string]any, parent string, child string) string {
	t.Helper()
	nested, ok := data[parent].(map[string]any)
	if !ok {
		t.Fatalf("response data.%s is not an object", parent)
	}
	value, ok := nested[child].(string)
	if !ok {
		t.Fatalf("response data.%s.%s is not a string", parent, child)
	}
	return value
}

func asStringSlice(t *testing.T, value any) []string {
	t.Helper()
	raw, ok := value.([]any)
	if !ok {
		t.Fatalf("value is not an array: %#v", value)
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("array item is not a string: %#v", item)
		}
		values = append(values, text)
	}
	return values
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func assertNoSensitiveFields(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, forbidden := range []string{"password_hash", "passwordHash", "jwt", "authorization", "cookie"} {
		if strings.Contains(lower, strings.ToLower(forbidden)) {
			t.Fatalf("response contains sensitive field marker %q: %s", forbidden, body)
		}
	}
}

func assertOperationLogs(t *testing.T, db *gorm.DB) {
	t.Helper()

	var logs []database.OperationLog
	if err := db.Order("created_at ASC").Find(&logs).Error; err != nil {
		t.Fatalf("load operation logs: %v", err)
	}

	seen := map[string]bool{}
	for _, log := range logs {
		seen[log.Action] = true
		metadata := strings.ToLower(log.MetadataJSON)
		for _, forbidden := range []string{"password", "token", "cookie", "authorization", "api_key", "jwt"} {
			if strings.Contains(metadata, forbidden) {
				t.Fatalf("operation log metadata contains %q: %#v", forbidden, log)
			}
		}
	}

	for _, action := range []string{"auth.init_admin", "auth.login", "auth.password_change", "auth.logout"} {
		if !seen[action] {
			t.Fatalf("missing operation log action %s; logs = %#v", action, logs)
		}
	}
}
