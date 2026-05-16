package provideradapter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactorReplacesCurrentAPIKeyInsideNestedMetadata(t *testing.T) {
	apiKey := "relay_live_1234567890abcdef"
	redactor := NewRedactor(apiKey)

	metadata := redactor.SanitizeMetadata(map[string]any{
		"message": "provider echoed " + apiKey,
		"nested": map[string]any{
			"items": []any{
				"prefix " + apiKey + " suffix",
				map[string]any{"safe": "unchanged"},
			},
		},
		"Authorization": "Bearer " + apiKey,
		"Cookie":        "session=" + apiKey,
	})

	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	text := string(encoded)
	if strings.Contains(text, apiKey) {
		t.Fatalf("metadata leaked API key: %s", text)
	}
	if !strings.Contains(text, redactedValue) {
		t.Fatalf("metadata did not contain redacted marker: %s", text)
	}
	if strings.Contains(strings.ToLower(text), "authorization") || strings.Contains(strings.ToLower(text), "cookie") {
		t.Fatalf("metadata leaked sensitive header keys: %s", text)
	}
}

func TestDefaultRedactionPatternsStillRemoveLegacySensitiveValues(t *testing.T) {
	message := SanitizeErrorMessage("Authorization Bearer sk-secret Cookie session=abc base64 AAAA")
	if strings.Contains(strings.ToLower(message), "authorization") ||
		strings.Contains(strings.ToLower(message), "bearer") ||
		strings.Contains(strings.ToLower(message), "sk-secret") ||
		strings.Contains(strings.ToLower(message), "cookie") ||
		strings.Contains(strings.ToLower(message), "base64") {
		t.Fatalf("legacy sensitive message leaked: %q", message)
	}

	metadata := SanitizeMetadata(map[string]any{
		"api_key":  "sk-secret",
		"b64_json": "AAAA",
		"safe":     "ok",
	})
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"api_key", "sk-secret", "b64_json", "aaaa"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("legacy metadata leaked %q: %s", forbidden, encoded)
		}
	}
}
