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
	if cfg.API.ReadTimeout != 15*time.Second {
		t.Fatalf("API.ReadTimeout = %s, want 15s", cfg.API.ReadTimeout)
	}
	if cfg.Worker.Name != "backend-worker" {
		t.Fatalf("Worker.Name = %q, want backend-worker", cfg.Worker.Name)
	}
}

func TestLoadOverrides(t *testing.T) {
	values := map[string]string{
		"APP_ENV":                 "production",
		"LOG_LEVEL":               "warn",
		"API_ADDR":                ":9090",
		"API_READ_TIMEOUT":        "3s",
		"API_WRITE_TIMEOUT":       "4s",
		"API_SHUTDOWN_TIMEOUT":    "5s",
		"WORKER_NAME":             "image-worker",
		"WORKER_SHUTDOWN_TIMEOUT": "6s",
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
	if cfg.API.Addr != ":9090" {
		t.Fatalf("API.Addr = %q, want :9090", cfg.API.Addr)
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
