package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewRejectsUnsupportedLevel(t *testing.T) {
	_, err := New("verbose", nil)
	if err == nil {
		t.Fatal("New returned nil error for unsupported level")
	}
}

func TestNewHonorsLogLevel(t *testing.T) {
	var buf bytes.Buffer
	log, err := New("warn", &buf)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	log.Info("hidden")
	log.Warn("shown", slog.String("request_id", "req_test"))

	output := buf.String()
	if strings.Contains(output, "hidden") {
		t.Fatalf("info log was emitted at warn level: %s", output)
	}
	if !strings.Contains(output, "shown") {
		t.Fatalf("warn log was not emitted: %s", output)
	}
}

func TestNewRedactsSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	log, err := New("info", &buf)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	log.Info("request",
		slog.String("authorization", "Bearer secret-token"),
		slog.String("Cookie", "session=secret-cookie"),
		slog.String("password", "secret-password"),
		slog.String("api_key", "secret-api-key"),
		slog.String("refreshToken", "secret-refresh-token"),
		slog.String("public_field", "visible"),
	)

	output := buf.String()
	for _, secret := range []string{
		"Bearer secret-token",
		"secret-cookie",
		"secret-password",
		"secret-api-key",
		"secret-refresh-token",
	} {
		if strings.Contains(output, secret) {
			t.Fatalf("log leaked sensitive value %q in output %s", secret, output)
		}
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("log output does not contain redaction marker: %s", output)
	}
	if !strings.Contains(output, "visible") {
		t.Fatalf("log output removed non-sensitive field: %s", output)
	}
}
