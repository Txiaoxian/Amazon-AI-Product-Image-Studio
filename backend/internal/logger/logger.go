package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const redactedValue = "[REDACTED]"

func New(level string, out io.Writer) (*slog.Logger, error) {
	parsedLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	if out == nil {
		out = os.Stdout
	}

	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level:       parsedLevel,
		ReplaceAttr: redactAttr,
	})
	return slog.New(handler), nil
}

func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redactedValue)
	}

	return attr
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.NewReplacer("-", "_", ".", "_").Replace(normalized)
	if normalized == "" {
		return false
	}

	sensitiveFragments := []string{
		"authorization",
		"cookie",
		"password",
		"passwd",
		"api_key",
		"apikey",
		"access_key",
		"secret_key",
		"secret",
		"token",
		"jwt",
		"credential",
		"session",
	}
	for _, fragment := range sensitiveFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}

	return false
}

func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", level)
	}
}
