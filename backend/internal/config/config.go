package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppEnv                = "development"
	defaultLogLevel              = "info"
	defaultAPIHost               = ""
	defaultAPIPort               = 8080
	defaultHTTPTimeout           = 15 * time.Second
	defaultAPIShutdownTimeout    = 10 * time.Second
	defaultWorkerName            = "backend-worker"
	defaultWorkerShutdownTimeout = 10 * time.Second
	defaultMySQLHost             = "127.0.0.1"
	defaultMySQLPort             = 3306
	defaultMySQLDatabase         = "amazon_ai_image_studio"
	defaultMySQLUser             = "studio_app"
	defaultDatabaseConnectTime   = 10 * time.Second
	defaultDatabaseMaxOpenConns  = 25
	defaultDatabaseMaxIdleConns  = 5
	defaultDatabaseConnLifetime  = 30 * time.Minute
	defaultMigrationsMode        = "startup-gate"
	defaultJWTSigningSecret      = "change-me-at-least-32-bytes-prod-must-replace"
	defaultJWTIssuer             = "amazon-ai-product-image-studio"
	defaultJWTAccessTokenTTL     = 60 * time.Minute
	defaultAuthCookieName        = "studio_auth"
	defaultCSRFCookieName        = "studio_csrf"
	defaultCSRFHeaderName        = "X-CSRF-Token"
	defaultCookieSameSite        = "Lax"
	defaultMinIOEndpoint         = "http://127.0.0.1:9000"
	defaultMinIORegion           = "us-east-1"
	defaultMinIOAccessKey        = "minioadmin"
	defaultMinIOSecretKey        = "change-me-local-minio-password"
	defaultMinIOBucketOriginals  = "product-originals"
	defaultMinIOBucketGenerated  = "product-generated"
	defaultMinIOBucketThumbnails = "product-thumbnails"
	defaultUploadMaxFileSizeMB   = 25
	defaultUploadMaxWidth        = 8192
	defaultUploadMaxHeight       = 8192
	defaultUploadMaxPixels       = 40000000
	defaultUploadAllowedMIMEs    = "image/jpeg,image/png,image/webp"
)

type Config struct {
	AppEnv   string
	LogLevel string
	API      APIConfig
	Worker   WorkerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Storage  StorageConfig
	Upload   UploadConfig
}

type APIConfig struct {
	Host               string
	Port               int
	Addr               string
	CORSAllowedOrigins []string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	ShutdownTimeout    time.Duration
}

type WorkerConfig struct {
	Name            string
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	ConnectTimeout  time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	MigrationsMode  string
}

type AuthConfig struct {
	JWTSigningSecret string
	JWTIssuer        string
	AccessTokenTTL   time.Duration
	Cookie           CookieConfig
	CSRF             CSRFConfig
}

type StorageConfig struct {
	Endpoint         string
	Region           string
	AccessKey        string
	SecretKey        string
	BucketOriginals  string
	BucketGenerated  string
	BucketThumbnails string
}

type UploadConfig struct {
	MaxFileSizeBytes int64
	MaxWidth         int
	MaxHeight        int
	MaxPixels        int64
	AllowedMIMETypes []string
}

type CookieConfig struct {
	Name     string
	Domain   string
	Secure   bool
	SameSite string
}

type CSRFConfig struct {
	Enabled    bool
	CookieName string
	HeaderName string
}

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

type lookupFunc func(string) (string, bool)

func load(lookup lookupFunc) (Config, error) {
	appEnv := stringFromEnv(lookup, "APP_ENV", defaultAppEnv)
	if err := validateAppEnv(appEnv); err != nil {
		return Config{}, err
	}

	logLevel := stringFromEnv(lookup, "LOG_LEVEL", defaultLogLevel)
	if err := validateLogLevel(logLevel); err != nil {
		return Config{}, err
	}

	apiHost, apiPort, apiAddr, err := apiBindFromEnv(lookup)
	if err != nil {
		return Config{}, err
	}

	corsAllowedOrigins, err := corsAllowedOriginsFromEnv(lookup)
	if err != nil {
		return Config{}, err
	}

	apiReadTimeout, err := durationFromEnv(lookup, "API_READ_TIMEOUT", defaultHTTPTimeout)
	if err != nil {
		return Config{}, err
	}

	apiWriteTimeout, err := durationFromEnv(lookup, "API_WRITE_TIMEOUT", defaultHTTPTimeout)
	if err != nil {
		return Config{}, err
	}

	apiShutdownTimeout, err := durationFromEnv(lookup, "API_SHUTDOWN_TIMEOUT", defaultAPIShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	workerShutdownTimeout, err := durationFromEnv(lookup, "WORKER_SHUTDOWN_TIMEOUT", defaultWorkerShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	database, err := databaseConfigFromEnv(lookup)
	if err != nil {
		return Config{}, err
	}

	auth, err := authConfigFromEnv(lookup)
	if err != nil {
		return Config{}, err
	}

	storage, err := storageConfigFromEnv(lookup)
	if err != nil {
		return Config{}, err
	}

	upload, err := uploadConfigFromEnv(lookup)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppEnv:   appEnv,
		LogLevel: logLevel,
		API: APIConfig{
			Host:               apiHost,
			Port:               apiPort,
			Addr:               apiAddr,
			CORSAllowedOrigins: corsAllowedOrigins,
			ReadTimeout:        apiReadTimeout,
			WriteTimeout:       apiWriteTimeout,
			ShutdownTimeout:    apiShutdownTimeout,
		},
		Worker: WorkerConfig{
			Name:            stringFromEnv(lookup, "WORKER_NAME", defaultWorkerName),
			ShutdownTimeout: workerShutdownTimeout,
		},
		Database: database,
		Auth:     auth,
		Storage:  storage,
		Upload:   upload,
	}, nil
}

func stringFromEnv(lookup lookupFunc, key, fallback string) string {
	value, ok := lookup(key)
	if !ok {
		return fallback
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	return value
}

func validateAppEnv(appEnv string) error {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "local", "development", "test", "staging", "production":
		return nil
	default:
		return fmt.Errorf("invalid APP_ENV %q", appEnv)
	}
}

func validateLogLevel(logLevel string) error {
	switch strings.ToLower(strings.TrimSpace(logLevel)) {
	case "", "debug", "info", "warn", "warning", "error":
		return nil
	default:
		return fmt.Errorf("invalid LOG_LEVEL %q", logLevel)
	}
}

func apiBindFromEnv(lookup lookupFunc) (string, int, string, error) {
	apiAddr := stringFromEnv(lookup, "API_ADDR", "")
	if apiAddr != "" {
		host, portString, err := net.SplitHostPort(apiAddr)
		if err != nil {
			return "", 0, "", fmt.Errorf("invalid API_ADDR: %w", err)
		}

		port, err := parsePort("API_ADDR", portString)
		if err != nil {
			return "", 0, "", err
		}

		if err := validateHost("API_ADDR", host); err != nil {
			return "", 0, "", err
		}

		return host, port, net.JoinHostPort(host, strconv.Itoa(port)), nil
	}

	host := stringFromEnv(lookup, "BACKEND_HTTP_HOST", defaultAPIHost)
	if err := validateHost("BACKEND_HTTP_HOST", host); err != nil {
		return "", 0, "", err
	}

	port, err := intFromEnv(lookup, "BACKEND_HTTP_PORT", defaultAPIPort)
	if err != nil {
		return "", 0, "", err
	}

	return host, port, net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func validateHost(key string, host string) error {
	if strings.TrimSpace(host) != host {
		return fmt.Errorf("invalid %s: host must not contain surrounding whitespace", key)
	}

	for _, r := range host {
		if r <= 31 || r == 127 {
			return fmt.Errorf("invalid %s: host contains control character", key)
		}
	}

	return nil
}

func intFromEnv(lookup lookupFunc, key string, fallback int) (int, error) {
	raw := stringFromEnv(lookup, key, "")
	if raw == "" {
		return fallback, nil
	}

	return parsePort(key, raw)
}

func parsePort(key, raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: must be an integer", key)
	}

	if value < 1 || value > 65535 {
		return 0, fmt.Errorf("invalid %s: must be between 1 and 65535", key)
	}

	return value, nil
}

func databaseConfigFromEnv(lookup lookupFunc) (DatabaseConfig, error) {
	host := stringFromEnv(lookup, "MYSQL_HOST", defaultMySQLHost)
	if err := validateHost("MYSQL_HOST", host); err != nil {
		return DatabaseConfig{}, err
	}

	port, err := intFromEnv(lookup, "MYSQL_PORT", defaultMySQLPort)
	if err != nil {
		return DatabaseConfig{}, err
	}

	name := stringFromEnv(lookup, "MYSQL_DATABASE", defaultMySQLDatabase)
	if err := validateRequiredValue("MYSQL_DATABASE", name); err != nil {
		return DatabaseConfig{}, err
	}

	user := stringFromEnv(lookup, "MYSQL_USER", defaultMySQLUser)
	if err := validateRequiredValue("MYSQL_USER", user); err != nil {
		return DatabaseConfig{}, err
	}

	connectTimeout, err := durationFromEnv(lookup, "MYSQL_CONNECT_TIMEOUT", defaultDatabaseConnectTime)
	if err != nil {
		return DatabaseConfig{}, err
	}

	connLifetime, err := durationFromEnv(lookup, "MYSQL_CONN_MAX_LIFETIME", defaultDatabaseConnLifetime)
	if err != nil {
		return DatabaseConfig{}, err
	}

	maxOpenConns, err := positiveIntFromEnv(lookup, "MYSQL_MAX_OPEN_CONNS", defaultDatabaseMaxOpenConns)
	if err != nil {
		return DatabaseConfig{}, err
	}

	maxIdleConns, err := positiveIntFromEnv(lookup, "MYSQL_MAX_IDLE_CONNS", defaultDatabaseMaxIdleConns)
	if err != nil {
		return DatabaseConfig{}, err
	}
	if maxIdleConns > maxOpenConns {
		return DatabaseConfig{}, fmt.Errorf("invalid MYSQL_MAX_IDLE_CONNS: must be less than or equal to MYSQL_MAX_OPEN_CONNS")
	}

	migrationsMode, err := migrationsModeFromEnv(lookup)
	if err != nil {
		return DatabaseConfig{}, err
	}

	return DatabaseConfig{
		Host:            host,
		Port:            port,
		Name:            name,
		User:            user,
		Password:        stringFromEnv(lookup, "MYSQL_PASSWORD", ""),
		ConnectTimeout:  connectTimeout,
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connLifetime,
		MigrationsMode:  migrationsMode,
	}, nil
}

func validateRequiredValue(key string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("invalid %s: value is required", key)
	}

	for _, r := range value {
		if r <= 31 || r == 127 {
			return fmt.Errorf("invalid %s: value contains control character", key)
		}
	}

	return nil
}

func positiveIntFromEnv(lookup lookupFunc, key string, fallback int) (int, error) {
	raw := stringFromEnv(lookup, key, "")
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: must be an integer", key)
	}
	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: must be positive", key)
	}

	return value, nil
}

func positiveInt64FromEnv(lookup lookupFunc, key string, fallback int64) (int64, error) {
	raw := stringFromEnv(lookup, key, "")
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: must be an integer", key)
	}
	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: must be positive", key)
	}

	return value, nil
}

func migrationsModeFromEnv(lookup lookupFunc) (string, error) {
	mode := strings.ToLower(stringFromEnv(lookup, "MIGRATIONS_MODE", defaultMigrationsMode))
	switch mode {
	case "startup-gate", "disabled":
		return mode, nil
	default:
		return "", fmt.Errorf("invalid MIGRATIONS_MODE %q", mode)
	}
}

func authConfigFromEnv(lookup lookupFunc) (AuthConfig, error) {
	signingSecret := stringFromEnv(lookup, "JWT_SIGNING_SECRET", defaultJWTSigningSecret)
	if len(signingSecret) < 32 {
		return AuthConfig{}, fmt.Errorf("invalid JWT_SIGNING_SECRET: must be at least 32 characters")
	}
	if err := validateRequiredValue("JWT_SIGNING_SECRET", signingSecret); err != nil {
		return AuthConfig{}, err
	}

	issuer := stringFromEnv(lookup, "JWT_ISSUER", defaultJWTIssuer)
	if err := validateRequiredValue("JWT_ISSUER", issuer); err != nil {
		return AuthConfig{}, err
	}

	ttlMinutes, err := positiveIntFromEnv(lookup, "JWT_ACCESS_TOKEN_TTL_MINUTES", int(defaultJWTAccessTokenTTL/time.Minute))
	if err != nil {
		return AuthConfig{}, err
	}

	cookieName := stringFromEnv(lookup, "AUTH_COOKIE_NAME", defaultAuthCookieName)
	if err := validateCookieName("AUTH_COOKIE_NAME", cookieName); err != nil {
		return AuthConfig{}, err
	}

	cookieDomain := stringFromEnv(lookup, "COOKIE_DOMAIN", "")
	if err := validateCookieDomain(cookieDomain); err != nil {
		return AuthConfig{}, err
	}

	cookieSecure, err := boolFromEnv(lookup, "COOKIE_SECURE", false)
	if err != nil {
		return AuthConfig{}, err
	}

	sameSite, err := cookieSameSiteFromEnv(lookup)
	if err != nil {
		return AuthConfig{}, err
	}
	if strings.EqualFold(sameSite, "None") && !cookieSecure {
		return AuthConfig{}, fmt.Errorf("invalid COOKIE_SAME_SITE: None requires COOKIE_SECURE=true")
	}

	csrfEnabled, err := boolFromEnv(lookup, "CSRF_ENABLED", true)
	if err != nil {
		return AuthConfig{}, err
	}

	csrfCookieName := stringFromEnv(lookup, "CSRF_COOKIE_NAME", defaultCSRFCookieName)
	if err := validateCookieName("CSRF_COOKIE_NAME", csrfCookieName); err != nil {
		return AuthConfig{}, err
	}
	if csrfCookieName == cookieName {
		return AuthConfig{}, fmt.Errorf("invalid CSRF_COOKIE_NAME: must differ from AUTH_COOKIE_NAME")
	}

	csrfHeaderName := stringFromEnv(lookup, "CSRF_HEADER_NAME", defaultCSRFHeaderName)
	if err := validateHeaderName("CSRF_HEADER_NAME", csrfHeaderName); err != nil {
		return AuthConfig{}, err
	}

	return AuthConfig{
		JWTSigningSecret: signingSecret,
		JWTIssuer:        issuer,
		AccessTokenTTL:   time.Duration(ttlMinutes) * time.Minute,
		Cookie: CookieConfig{
			Name:     cookieName,
			Domain:   cookieDomain,
			Secure:   cookieSecure,
			SameSite: sameSite,
		},
		CSRF: CSRFConfig{
			Enabled:    csrfEnabled,
			CookieName: csrfCookieName,
			HeaderName: csrfHeaderName,
		},
	}, nil
}

func DefaultStorageConfig() StorageConfig {
	return StorageConfig{
		Endpoint:         defaultMinIOEndpoint,
		Region:           defaultMinIORegion,
		AccessKey:        defaultMinIOAccessKey,
		SecretKey:        defaultMinIOSecretKey,
		BucketOriginals:  defaultMinIOBucketOriginals,
		BucketGenerated:  defaultMinIOBucketGenerated,
		BucketThumbnails: defaultMinIOBucketThumbnails,
	}
}

func DefaultUploadConfig() UploadConfig {
	return UploadConfig{
		MaxFileSizeBytes: int64(defaultUploadMaxFileSizeMB) * 1024 * 1024,
		MaxWidth:         defaultUploadMaxWidth,
		MaxHeight:        defaultUploadMaxHeight,
		MaxPixels:        defaultUploadMaxPixels,
		AllowedMIMETypes: []string{"image/jpeg", "image/png", "image/webp"},
	}
}

func NormalizeStorageConfig(storage StorageConfig) StorageConfig {
	defaults := DefaultStorageConfig()
	if strings.TrimSpace(storage.Endpoint) == "" {
		storage.Endpoint = defaults.Endpoint
	}
	if strings.TrimSpace(storage.Region) == "" {
		storage.Region = defaults.Region
	}
	if strings.TrimSpace(storage.AccessKey) == "" {
		storage.AccessKey = defaults.AccessKey
	}
	if strings.TrimSpace(storage.SecretKey) == "" {
		storage.SecretKey = defaults.SecretKey
	}
	if strings.TrimSpace(storage.BucketOriginals) == "" {
		storage.BucketOriginals = defaults.BucketOriginals
	}
	if strings.TrimSpace(storage.BucketGenerated) == "" {
		storage.BucketGenerated = defaults.BucketGenerated
	}
	if strings.TrimSpace(storage.BucketThumbnails) == "" {
		storage.BucketThumbnails = defaults.BucketThumbnails
	}
	return storage
}

func NormalizeUploadConfig(upload UploadConfig) UploadConfig {
	defaults := DefaultUploadConfig()
	if upload.MaxFileSizeBytes <= 0 {
		upload.MaxFileSizeBytes = defaults.MaxFileSizeBytes
	}
	if upload.MaxWidth <= 0 {
		upload.MaxWidth = defaults.MaxWidth
	}
	if upload.MaxHeight <= 0 {
		upload.MaxHeight = defaults.MaxHeight
	}
	if upload.MaxPixels <= 0 {
		upload.MaxPixels = defaults.MaxPixels
	}
	if len(upload.AllowedMIMETypes) == 0 {
		upload.AllowedMIMETypes = defaults.AllowedMIMETypes
	}
	return upload
}

func storageConfigFromEnv(lookup lookupFunc) (StorageConfig, error) {
	storage := NormalizeStorageConfig(StorageConfig{
		Endpoint:         stringFromEnv(lookup, "MINIO_ENDPOINT", ""),
		Region:           stringFromEnv(lookup, "MINIO_REGION", ""),
		AccessKey:        stringFromEnv(lookup, "MINIO_ACCESS_KEY", ""),
		SecretKey:        stringFromEnv(lookup, "MINIO_SECRET_KEY", ""),
		BucketOriginals:  stringFromEnv(lookup, "MINIO_BUCKET_ORIGINALS", ""),
		BucketGenerated:  stringFromEnv(lookup, "MINIO_BUCKET_GENERATED", ""),
		BucketThumbnails: stringFromEnv(lookup, "MINIO_BUCKET_THUMBNAILS", ""),
	})

	if err := validateStorageEndpoint(storage.Endpoint); err != nil {
		return StorageConfig{}, err
	}
	for key, value := range map[string]string{
		"MINIO_REGION":            storage.Region,
		"MINIO_ACCESS_KEY":        storage.AccessKey,
		"MINIO_SECRET_KEY":        storage.SecretKey,
		"MINIO_BUCKET_ORIGINALS":  storage.BucketOriginals,
		"MINIO_BUCKET_GENERATED":  storage.BucketGenerated,
		"MINIO_BUCKET_THUMBNAILS": storage.BucketThumbnails,
	} {
		if err := validateRequiredValue(key, value); err != nil {
			return StorageConfig{}, err
		}
	}

	return storage, nil
}

func uploadConfigFromEnv(lookup lookupFunc) (UploadConfig, error) {
	maxFileSizeMB, err := positiveInt64FromEnv(lookup, "UPLOAD_MAX_FILE_SIZE_MB", defaultUploadMaxFileSizeMB)
	if err != nil {
		return UploadConfig{}, err
	}
	maxWidth, err := positiveIntFromEnv(lookup, "UPLOAD_MAX_WIDTH", defaultUploadMaxWidth)
	if err != nil {
		return UploadConfig{}, err
	}
	maxHeight, err := positiveIntFromEnv(lookup, "UPLOAD_MAX_HEIGHT", defaultUploadMaxHeight)
	if err != nil {
		return UploadConfig{}, err
	}
	maxPixels, err := positiveInt64FromEnv(lookup, "UPLOAD_MAX_PIXELS", defaultUploadMaxPixels)
	if err != nil {
		return UploadConfig{}, err
	}
	allowed, err := allowedMIMETypesFromEnv(lookup)
	if err != nil {
		return UploadConfig{}, err
	}

	return UploadConfig{
		MaxFileSizeBytes: maxFileSizeMB * 1024 * 1024,
		MaxWidth:         maxWidth,
		MaxHeight:        maxHeight,
		MaxPixels:        maxPixels,
		AllowedMIMETypes: allowed,
	}, nil
}

func validateStorageEndpoint(endpoint string) error {
	if err := validateRequiredValue("MINIO_ENDPOINT", endpoint); err != nil {
		return err
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid MINIO_ENDPOINT: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid MINIO_ENDPOINT: scheme must be http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid MINIO_ENDPOINT: host is required")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("invalid MINIO_ENDPOINT: user info, path, query, and fragment are not allowed")
	}
	return nil
}

func allowedMIMETypesFromEnv(lookup lookupFunc) ([]string, error) {
	raw := stringFromEnv(lookup, "UPLOAD_ALLOWED_MIME_TYPES", defaultUploadAllowedMIMEs)
	parts := strings.Split(raw, ",")
	allowed := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		mimeType := strings.ToLower(strings.TrimSpace(part))
		if mimeType == "" {
			continue
		}
		if !uploadMIMEAllowed(mimeType) {
			return nil, fmt.Errorf("invalid UPLOAD_ALLOWED_MIME_TYPES: unsupported MIME type %q", mimeType)
		}
		if _, ok := seen[mimeType]; ok {
			continue
		}
		seen[mimeType] = struct{}{}
		allowed = append(allowed, mimeType)
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("invalid UPLOAD_ALLOWED_MIME_TYPES: at least one MIME type is required")
	}
	return allowed, nil
}

func uploadMIMEAllowed(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func boolFromEnv(lookup lookupFunc, key string, fallback bool) (bool, error) {
	raw := stringFromEnv(lookup, key, "")
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseBool(strings.ToLower(raw))
	if err != nil {
		return false, fmt.Errorf("invalid %s: must be a boolean", key)
	}

	return value, nil
}

func cookieSameSiteFromEnv(lookup lookupFunc) (string, error) {
	raw := stringFromEnv(lookup, "COOKIE_SAME_SITE", defaultCookieSameSite)
	switch strings.ToLower(raw) {
	case "lax":
		return "Lax", nil
	case "strict":
		return "Strict", nil
	case "none":
		return "None", nil
	default:
		return "", fmt.Errorf("invalid COOKIE_SAME_SITE %q: must be Lax, Strict, or None", raw)
	}
}

func validateCookieName(key, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("invalid %s: value is required", key)
	}
	for _, r := range name {
		if r <= 32 || r >= 127 || strings.ContainsRune("()<>@,;:\\\"/[]?={}", r) {
			return fmt.Errorf("invalid %s: contains an illegal cookie name character", key)
		}
	}
	return nil
}

func validateCookieDomain(domain string) error {
	if domain == "" {
		return nil
	}
	if strings.TrimSpace(domain) != domain {
		return fmt.Errorf("invalid COOKIE_DOMAIN: domain must not contain surrounding whitespace")
	}
	for _, r := range domain {
		if r <= 31 || r == 127 || r == '/' || r == ':' {
			return fmt.Errorf("invalid COOKIE_DOMAIN: contains an illegal character")
		}
	}
	return nil
}

func validateHeaderName(key, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("invalid %s: value is required", key)
	}
	for _, r := range name {
		if r <= 32 || r >= 127 || strings.ContainsRune("()<>@,;:\\\"/[]?={}", r) {
			return fmt.Errorf("invalid %s: contains an illegal header name character", key)
		}
	}
	return nil
}

func corsAllowedOriginsFromEnv(lookup lookupFunc) ([]string, error) {
	raw := stringFromEnv(lookup, "CORS_ALLOWED_ORIGINS", "")
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}

		normalized, err := normalizeCORSOrigin(origin)
		if err != nil {
			return nil, err
		}

		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		origins = append(origins, normalized)
	}

	return origins, nil
}

func normalizeCORSOrigin(origin string) (string, error) {
	if origin == "*" {
		return "", fmt.Errorf("invalid CORS_ALLOWED_ORIGINS: wildcard origins are not allowed")
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("invalid CORS_ALLOWED_ORIGINS origin %q: %w", origin, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid CORS_ALLOWED_ORIGINS origin %q: scheme must be http or https", origin)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid CORS_ALLOWED_ORIGINS origin %q: host is required", origin)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("invalid CORS_ALLOWED_ORIGINS origin %q: user info is not allowed", origin)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("invalid CORS_ALLOWED_ORIGINS origin %q: path, query, and fragment are not allowed", origin)
	}

	return parsed.Scheme + "://" + parsed.Host, nil
}

func durationFromEnv(lookup lookupFunc, key string, fallback time.Duration) (time.Duration, error) {
	raw := stringFromEnv(lookup, key, "")
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("invalid duration for %s: must be positive", key)
	}

	return value, nil
}
