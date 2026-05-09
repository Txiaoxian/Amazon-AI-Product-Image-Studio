package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultAppEnv                = "development"
	defaultLogLevel              = "info"
	defaultAPIAddr               = ":8080"
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
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
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
		AppEnv:   stringFromEnv(lookup, "APP_ENV", defaultAppEnv),
		LogLevel: stringFromEnv(lookup, "LOG_LEVEL", defaultLogLevel),
		API: APIConfig{
			Addr:            stringFromEnv(lookup, "API_ADDR", defaultAPIAddr),
			ReadTimeout:     apiReadTimeout,
			WriteTimeout:    apiWriteTimeout,
			ShutdownTimeout: apiShutdownTimeout,
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
