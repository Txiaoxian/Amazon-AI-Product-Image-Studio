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
	if cfg.Auth.JWTSigningSecret != defaultJWTSigningSecret {
		t.Fatal("Auth.JWTSigningSecret default was not loaded")
	}
	if cfg.Auth.JWTIssuer != "amazon-ai-product-image-studio" {
		t.Fatalf("Auth.JWTIssuer = %q, want amazon-ai-product-image-studio", cfg.Auth.JWTIssuer)
	}
	if cfg.Auth.AccessTokenTTL != time.Hour {
		t.Fatalf("Auth.AccessTokenTTL = %s, want 1h", cfg.Auth.AccessTokenTTL)
	}
	if cfg.Auth.Cookie.Name != "studio_auth" {
		t.Fatalf("Auth.Cookie.Name = %q, want studio_auth", cfg.Auth.Cookie.Name)
	}
	if cfg.Auth.Cookie.Secure {
		t.Fatal("Auth.Cookie.Secure default should be false")
	}
	if cfg.Auth.Cookie.SameSite != "Lax" {
		t.Fatalf("Auth.Cookie.SameSite = %q, want Lax", cfg.Auth.Cookie.SameSite)
	}
	if !cfg.Auth.CSRF.Enabled {
		t.Fatal("Auth.CSRF.Enabled default should be true")
	}
	if cfg.Auth.CSRF.CookieName != "studio_csrf" {
		t.Fatalf("Auth.CSRF.CookieName = %q, want studio_csrf", cfg.Auth.CSRF.CookieName)
	}
	if cfg.Auth.CSRF.HeaderName != "X-CSRF-Token" {
		t.Fatalf("Auth.CSRF.HeaderName = %q, want X-CSRF-Token", cfg.Auth.CSRF.HeaderName)
	}
	if cfg.Storage.Endpoint != "http://127.0.0.1:9000" {
		t.Fatalf("Storage.Endpoint = %q, want http://127.0.0.1:9000", cfg.Storage.Endpoint)
	}
	if cfg.Storage.BucketOriginals != "product-originals" {
		t.Fatalf("Storage.BucketOriginals = %q, want product-originals", cfg.Storage.BucketOriginals)
	}
	if cfg.Upload.MaxFileSizeBytes != 25*1024*1024 {
		t.Fatalf("Upload.MaxFileSizeBytes = %d, want 25MiB", cfg.Upload.MaxFileSizeBytes)
	}
	if cfg.Upload.MaxWidth != 8192 || cfg.Upload.MaxHeight != 8192 {
		t.Fatalf("Upload max dimensions = %dx%d, want 8192x8192", cfg.Upload.MaxWidth, cfg.Upload.MaxHeight)
	}
	if cfg.Upload.MaxPixels != 40000000 {
		t.Fatalf("Upload.MaxPixels = %d, want 40000000", cfg.Upload.MaxPixels)
	}
	if len(cfg.Upload.AllowedMIMETypes) != 3 || cfg.Upload.AllowedMIMETypes[0] != "image/jpeg" {
		t.Fatalf("Upload.AllowedMIMETypes = %#v, want image/jpeg,image/png,image/webp", cfg.Upload.AllowedMIMETypes)
	}
	if cfg.Provider.APIKeyEncryptionKey != defaultAPIKeyEncryptionKey {
		t.Fatal("Provider.APIKeyEncryptionKey default was not loaded")
	}
	if cfg.Provider.APIKeyEncryptionKeyID != defaultAPIKeyEncryptionKeyID {
		t.Fatalf("Provider.APIKeyEncryptionKeyID = %q, want %q", cfg.Provider.APIKeyEncryptionKeyID, defaultAPIKeyEncryptionKeyID)
	}
	if cfg.Provider.DefaultTimeout != 120*time.Second {
		t.Fatalf("Provider.DefaultTimeout = %s, want 120s", cfg.Provider.DefaultTimeout)
	}
	if cfg.Provider.MaxRetries != 2 {
		t.Fatalf("Provider.MaxRetries = %d, want 2", cfg.Provider.MaxRetries)
	}
	if cfg.Queue.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("Queue.RedisAddr = %q, want 127.0.0.1:6379", cfg.Queue.RedisAddr)
	}
	if cfg.Queue.RedisPassword != "" {
		t.Fatal("Queue.RedisPassword default should be empty")
	}
	if cfg.Queue.RedisDB != 0 {
		t.Fatalf("Queue.RedisDB = %d, want 0", cfg.Queue.RedisDB)
	}
	if cfg.Queue.TaskQueueName != "image-tasks" {
		t.Fatalf("Queue.TaskQueueName = %q, want image-tasks", cfg.Queue.TaskQueueName)
	}
	if cfg.Queue.EnqueueTimeout != 5*time.Second {
		t.Fatalf("Queue.EnqueueTimeout = %s, want 5s", cfg.Queue.EnqueueTimeout)
	}
}

func TestLoadOverrides(t *testing.T) {
	values := map[string]string{
		"APP_ENV":                      "production",
		"LOG_LEVEL":                    "warn",
		"BACKEND_HTTP_HOST":            "127.0.0.1",
		"BACKEND_HTTP_PORT":            "9090",
		"CORS_ALLOWED_ORIGINS":         "http://localhost:8080,https://studio.example.com",
		"API_READ_TIMEOUT":             "3s",
		"API_WRITE_TIMEOUT":            "4s",
		"API_SHUTDOWN_TIMEOUT":         "5s",
		"WORKER_NAME":                  "image-worker",
		"WORKER_SHUTDOWN_TIMEOUT":      "6s",
		"MYSQL_HOST":                   "mysql",
		"MYSQL_PORT":                   "3307",
		"MYSQL_DATABASE":               "studio_test",
		"MYSQL_USER":                   "studio_user",
		"MYSQL_PASSWORD":               "local-password",
		"MYSQL_CONNECT_TIMEOUT":        "7s",
		"MYSQL_MAX_OPEN_CONNS":         "12",
		"MYSQL_MAX_IDLE_CONNS":         "4",
		"MYSQL_CONN_MAX_LIFETIME":      "8m",
		"MIGRATIONS_MODE":              "disabled",
		"JWT_SIGNING_SECRET":           "0123456789abcdef0123456789abcdef",
		"JWT_ISSUER":                   "studio-test",
		"JWT_ACCESS_TOKEN_TTL_MINUTES": "30",
		"AUTH_COOKIE_NAME":             "auth_test",
		"COOKIE_DOMAIN":                ".example.com",
		"COOKIE_SECURE":                "true",
		"COOKIE_SAME_SITE":             "Strict",
		"CSRF_ENABLED":                 "false",
		"CSRF_COOKIE_NAME":             "csrf_test",
		"CSRF_HEADER_NAME":             "X-Test-CSRF",
		"MINIO_ENDPOINT":               "https://minio.example.com",
		"MINIO_REGION":                 "us-west-2",
		"MINIO_ACCESS_KEY":             "local-access",
		"MINIO_SECRET_KEY":             "local-secret",
		"MINIO_BUCKET_ORIGINALS":       "originals-test",
		"MINIO_BUCKET_GENERATED":       "generated-test",
		"MINIO_BUCKET_THUMBNAILS":      "thumbs-test",
		"UPLOAD_MAX_FILE_SIZE_MB":      "9",
		"UPLOAD_MAX_WIDTH":             "2048",
		"UPLOAD_MAX_HEIGHT":            "1536",
		"UPLOAD_MAX_PIXELS":            "3000000",
		"UPLOAD_ALLOWED_MIME_TYPES":    "image/png,image/webp",
		"API_KEY_ENCRYPTION_KEY":       "0123456789abcdef0123456789abcdef",
		"API_KEY_ENCRYPTION_KEY_ID":    "test-key-v1",
		"PROVIDER_TIMEOUT_SECONDS":     "45",
		"PROVIDER_MAX_RETRIES":         "5",
		"REDIS_ADDR":                   "redis.example.com:6380",
		"REDIS_PASSWORD":               "local-redis-password",
		"REDIS_DB":                     "2",
		"TASK_QUEUE_NAME":              "task-queue-test",
		"TASK_ENQUEUE_TIMEOUT":         "9s",
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
	if cfg.Auth.JWTSigningSecret != "0123456789abcdef0123456789abcdef" {
		t.Fatal("Auth.JWTSigningSecret override was not loaded")
	}
	if cfg.Auth.JWTIssuer != "studio-test" {
		t.Fatalf("Auth.JWTIssuer = %q, want studio-test", cfg.Auth.JWTIssuer)
	}
	if cfg.Auth.AccessTokenTTL != 30*time.Minute {
		t.Fatalf("Auth.AccessTokenTTL = %s, want 30m", cfg.Auth.AccessTokenTTL)
	}
	if cfg.Auth.Cookie.Name != "auth_test" {
		t.Fatalf("Auth.Cookie.Name = %q, want auth_test", cfg.Auth.Cookie.Name)
	}
	if cfg.Auth.Cookie.Domain != ".example.com" {
		t.Fatalf("Auth.Cookie.Domain = %q, want .example.com", cfg.Auth.Cookie.Domain)
	}
	if !cfg.Auth.Cookie.Secure {
		t.Fatal("Auth.Cookie.Secure override was not loaded")
	}
	if cfg.Auth.Cookie.SameSite != "Strict" {
		t.Fatalf("Auth.Cookie.SameSite = %q, want Strict", cfg.Auth.Cookie.SameSite)
	}
	if cfg.Auth.CSRF.Enabled {
		t.Fatal("Auth.CSRF.Enabled override was not loaded")
	}
	if cfg.Auth.CSRF.CookieName != "csrf_test" {
		t.Fatalf("Auth.CSRF.CookieName = %q, want csrf_test", cfg.Auth.CSRF.CookieName)
	}
	if cfg.Auth.CSRF.HeaderName != "X-Test-CSRF" {
		t.Fatalf("Auth.CSRF.HeaderName = %q, want X-Test-CSRF", cfg.Auth.CSRF.HeaderName)
	}
	if cfg.Storage.Endpoint != "https://minio.example.com" {
		t.Fatalf("Storage.Endpoint = %q, want https://minio.example.com", cfg.Storage.Endpoint)
	}
	if cfg.Storage.Region != "us-west-2" {
		t.Fatalf("Storage.Region = %q, want us-west-2", cfg.Storage.Region)
	}
	if cfg.Storage.AccessKey != "local-access" {
		t.Fatal("Storage.AccessKey override was not loaded")
	}
	if cfg.Storage.SecretKey != "local-secret" {
		t.Fatal("Storage.SecretKey override was not loaded")
	}
	if cfg.Storage.BucketOriginals != "originals-test" || cfg.Storage.BucketGenerated != "generated-test" || cfg.Storage.BucketThumbnails != "thumbs-test" {
		t.Fatalf("Storage buckets = %#v", cfg.Storage)
	}
	if cfg.Upload.MaxFileSizeBytes != 9*1024*1024 {
		t.Fatalf("Upload.MaxFileSizeBytes = %d, want 9MiB", cfg.Upload.MaxFileSizeBytes)
	}
	if cfg.Upload.MaxWidth != 2048 || cfg.Upload.MaxHeight != 1536 {
		t.Fatalf("Upload max dimensions = %dx%d, want 2048x1536", cfg.Upload.MaxWidth, cfg.Upload.MaxHeight)
	}
	if cfg.Upload.MaxPixels != 3000000 {
		t.Fatalf("Upload.MaxPixels = %d, want 3000000", cfg.Upload.MaxPixels)
	}
	if len(cfg.Upload.AllowedMIMETypes) != 2 || cfg.Upload.AllowedMIMETypes[0] != "image/png" || cfg.Upload.AllowedMIMETypes[1] != "image/webp" {
		t.Fatalf("Upload.AllowedMIMETypes = %#v, want image/png,image/webp", cfg.Upload.AllowedMIMETypes)
	}
	if cfg.Provider.APIKeyEncryptionKey != "0123456789abcdef0123456789abcdef" {
		t.Fatal("Provider.APIKeyEncryptionKey override was not loaded")
	}
	if cfg.Provider.APIKeyEncryptionKeyID != "test-key-v1" {
		t.Fatalf("Provider.APIKeyEncryptionKeyID = %q, want test-key-v1", cfg.Provider.APIKeyEncryptionKeyID)
	}
	if cfg.Provider.DefaultTimeout != 45*time.Second {
		t.Fatalf("Provider.DefaultTimeout = %s, want 45s", cfg.Provider.DefaultTimeout)
	}
	if cfg.Provider.MaxRetries != 5 {
		t.Fatalf("Provider.MaxRetries = %d, want 5", cfg.Provider.MaxRetries)
	}
	if cfg.Queue.RedisAddr != "redis.example.com:6380" {
		t.Fatalf("Queue.RedisAddr = %q, want redis.example.com:6380", cfg.Queue.RedisAddr)
	}
	if cfg.Queue.RedisPassword != "local-redis-password" {
		t.Fatal("Queue.RedisPassword override was not loaded")
	}
	if cfg.Queue.RedisDB != 2 {
		t.Fatalf("Queue.RedisDB = %d, want 2", cfg.Queue.RedisDB)
	}
	if cfg.Queue.TaskQueueName != "task-queue-test" {
		t.Fatalf("Queue.TaskQueueName = %q, want task-queue-test", cfg.Queue.TaskQueueName)
	}
	if cfg.Queue.EnqueueTimeout != 9*time.Second {
		t.Fatalf("Queue.EnqueueTimeout = %s, want 9s", cfg.Queue.EnqueueTimeout)
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

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "JWT_SIGNING_SECRET" {
			return "too-short", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("load returned nil error for short JWT signing secret")
	}
}

func TestLoadRejectsInvalidCookieSameSite(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		if key == "COOKIE_SAME_SITE" {
			return "Open", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("load returned nil error for invalid cookie SameSite")
	}
}

func TestLoadRejectsSameSiteNoneWithoutSecureCookie(t *testing.T) {
	_, err := load(func(key string) (string, bool) {
		switch key {
		case "COOKIE_SAME_SITE":
			return "None", true
		case "COOKIE_SECURE":
			return "false", true
		default:
			return "", false
		}
	})
	if err == nil {
		t.Fatal("load returned nil error for SameSite=None without Secure")
	}
}
