package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(func(string) (string, bool) {
		return "", false
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.API.Addr != ":8080" {
		t.Fatalf("API.Addr = %q, want :8080", cfg.API.Addr)
	}
	if cfg.API.Host != "" {
		t.Fatalf("API.Host = %q, want empty host", cfg.API.Host)
	}
	if cfg.API.Port != 8080 {
		t.Fatalf("API.Port = %d, want 8080", cfg.API.Port)
	}
	if len(cfg.API.CORSAllowedOrigins) != 0 {
		t.Fatalf("API.CORSAllowedOrigins = %v, want empty", cfg.API.CORSAllowedOrigins)
	}
	if cfg.API.ReadTimeout != 15*time.Second {
		t.Fatalf("API.ReadTimeout = %s, want 15s", cfg.API.ReadTimeout)
	}
	if cfg.Worker.Name != "backend-worker" {
		t.Fatalf("Worker.Name = %q, want backend-worker", cfg.Worker.Name)
	}
	if cfg.Database.Host != "127.0.0.1" {
		t.Fatalf("Database.Host = %q, want 127.0.0.1", cfg.Database.Host)
	}
	if cfg.Database.Port != 3306 {
		t.Fatalf("Database.Port = %d, want 3306", cfg.Database.Port)
	}
	if cfg.Database.Name != "amazon_ai_image_studio" {
		t.Fatalf("Database.Name = %q, want amazon_ai_image_studio", cfg.Database.Name)
	}
	if cfg.Database.User != "studio_app" {
		t.Fatalf("Database.User = %q, want studio_app", cfg.Database.User)
	}
	if cfg.Database.Password != "" {
		t.Fatal("Database.Password default should be empty")
	}
	if cfg.Database.MigrationsMode != "startup-gate" {
		t.Fatalf("Database.MigrationsMode = %q, want startup-gate", cfg.Database.MigrationsMode)
	}
}

func TestLoadOverrides(t *testing.T) {
	values := map[string]string{
		"APP_ENV":                 "production",
		"LOG_LEVEL":               "warn",
		"BACKEND_HTTP_HOST":       "127.0.0.1",
		"BACKEND_HTTP_PORT":       "9090",
		"CORS_ALLOWED_ORIGINS":    "http://localhost:8080,https://studio.example.com",
		"API_READ_TIMEOUT":        "3s",
		"API_WRITE_TIMEOUT":       "4s",
		"API_SHUTDOWN_TIMEOUT":    "5s",
		"WORKER_NAME":             "image-worker",
		"WORKER_SHUTDOWN_TIMEOUT": "6s",
		"MYSQL_HOST":              "mysql",
		"MYSQL_PORT":              "3307",
		"MYSQL_DATABASE":          "studio_test",
		"MYSQL_USER":              "studio_user",
		"MYSQL_PASSWORD":          "local-password",
		"MYSQL_CONNECT_TIMEOUT":   "7s",
		"MYSQL_MAX_OPEN_CONNS":    "12",
		"MYSQL_MAX_IDLE_CONNS":    "4",
		"MYSQL_CONN_MAX_LIFETIME": "8m",
		"MIGRATIONS_MODE":         "disabled",
	}

	cfg, err := load(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	if !cfg.IsProduction() {
		t.Fatal("IsProduction returned false for production env")
	}
	if cfg.API.Addr != "127.0.0.1:9090" {
		t.Fatalf("API.Addr = %q, want 127.0.0.1:9090", cfg.API.Addr)
	}
	if cfg.API.Host != "127.0.0.1" {
		t.Fatalf("API.Host = %q, want 127.0.0.1", cfg.API.Host)
	}
	if cfg.API.Port != 9090 {
		t.Fatalf("API.Port = %d, want 9090", cfg.API.Port)
	}
	if len(cfg.API.CORSAllowedOrigins) != 2 {
		t.Fatalf("API.CORSAllowedOrigins length = %d, want 2", len(cfg.API.CORSAllowedOrigins))
	}
	if cfg.API.CORSAllowedOrigins[0] != "http://localhost:8080" {
		t.Fatalf("first CORS origin = %q, want http://localhost:8080", cfg.API.CORSAllowedOrigins[0])
	}
	if cfg.API.ReadTimeout != 3*time.Second {
		t.Fatalf("API.ReadTimeout = %s, want 3s", cfg.API.ReadTimeout)
	}
	if cfg.API.WriteTimeout != 4*time.Second {
		t.Fatalf("API.WriteTimeout = %s, want 4s", cfg.API.WriteTimeout)
	}
	if cfg.API.ShutdownTimeout != 5*time.Second {
		t.Fatalf("API.ShutdownTimeout = %s, want 5s", cfg.API.ShutdownTimeout)
	}
	if cfg.Worker.Name != "image-worker" {
		t.Fatalf("Worker.Name = %q, want image-worker", cfg.Worker.Name)
	}
	if cfg.Worker.ShutdownTimeout != 6*time.Second {
		t.Fatalf("Worker.ShutdownTimeout = %s, want 6s", cfg.Worker.ShutdownTimeout)
	}
	if cfg.Database.Host != "mysql" {
		t.Fatalf("Database.Host = %q, want mysql", cfg.Database.Host)
	}
	if cfg.Database.Port != 3307 {
		t.Fatalf("Database.Port = %d, want 3307", cfg.Database.Port)
	}
	if cfg.Database.Name != "studio_test" {
		t.Fatalf("Database.Name = %q, want studio_test", cfg.Database.Name)
	}
	if cfg.Database.User != "studio_user" {
		t.Fatalf("Database.User = %q, want studio_user", cfg.Database.User)
	}
	if cfg.Database.Password != "local-password" {
		t.Fatal("Database.Password override was not loaded")
	}
	if cfg.Database.ConnectTimeout != 7*time.Second {
		t.Fatalf("Database.ConnectTimeout = %s, want 7s", cfg.Database.ConnectTimeout)
	}
	if cfg.Database.MaxOpenConns != 12 {
		t.Fatalf("Database.MaxOpenConns = %d, want 12", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns != 4 {
		t.Fatalf("Database.MaxIdleConns = %d, want 4", cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime != 8*time.Minute {
		t.Fatalf("Database.ConnMaxLifetime = %s, want 8m", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Database.MigrationsMode != "disabled" {
		t.Fatalf("Database.MigrationsMode = %q, want disabled", cfg.Database.MigrationsMode)
	}
}

func TestLoadAPIAddrOverridesHostAndPort(t *testing.T) {
	values := map[string]string{
		"API_ADDR":          "127.0.0.1:7070",
		"BACKEND_HTTP_HOST": "0.0.0.0",
		"BACKEND_HTTP_PORT": "8080",
	}

	cfg, err := load(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	if cfg.API.Addr != "127.0.0.1:7070" {
		t.Fatalf("API.Addr = %q, want 127.0.0.1:7070", cfg.API.Addr)
	}
	if cfg.API.Host != "127.0.0.1" {
		t.Fatalf("API.Host = %q, want 127.0.0.1", cfg.API.Host)
	}
	if cfg.API.Port != 7070 {
		t.Fatalf("API.Port = %d, want 7070", cfg.API.Port)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "API_READ_TIMEOUT" {
			return "soon", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("load returned nil error for invalid duration")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "BACKEND_HTTP_PORT" {
			return "70000", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("load returned nil error for invalid port")
	}
}

func TestLoadRejectsInvalidMySQLPort(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "MYSQL_PORT" {
			return "0", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("load returned nil error for invalid MySQL port")
	}
}

func TestLoadRejectsInvalidMigrationsMode(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "MIGRATIONS_MODE" {
			return "auto-drop", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("load returned nil error for invalid migrations mode")
	}
}

func TestLoadRejectsWildcardCORSOrigin(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "CORS_ALLOWED_ORIGINS" {
			return "*", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("load returned nil error for wildcard CORS origin")
	}
}

func TestLoadRejectsCORSOriginWithPath(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "CORS_ALLOWED_ORIGINS" {
			return "https://studio.example.com/app", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("load returned nil error for CORS origin with path")
	}
}
