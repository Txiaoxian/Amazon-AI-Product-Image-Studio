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
