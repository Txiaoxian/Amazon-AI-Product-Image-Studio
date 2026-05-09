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
)

type Config struct {
	AppEnv   string
	LogLevel string
	API      APIConfig
	Worker   WorkerConfig
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
