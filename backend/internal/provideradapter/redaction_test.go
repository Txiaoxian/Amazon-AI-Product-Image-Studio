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

func TestRedactorDropsCurrentAPIKeyInsideNestedMetadataKeys(t *testing.T) {
	apiKey := "relay_live_1234567890abcdef"
	redactor := NewRedactor(apiKey)

	metadata := redactor.SanitizeMetadata(map[string]any{
		"nested": map[string]any{
			apiKey:             "exact key must be dropped",
			"prefix_" + apiKey: "containing key must be dropped",
			"safe":             "unchanged",
		},
	})

	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	text := string(encoded)
	if strings.Contains(text, apiKey) {
		t.Fatalf("metadata key leaked API key: %s", text)
	}
	nested, ok := metadata["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested metadata = %#v, want map", metadata["nested"])
	}
	if len(nested) != 1 || nested["safe"] != "unchanged" {
		t.Fatalf("nested metadata = %#v, want only safe key", nested)
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

func TestRuntimeRedactionDropsNestedSecretsHeadersAndImagePayloadsTogether(t *testing.T) {
	apiKey := "fake-secret-for-redaction-test"
	redactor := NewRedactor(apiKey)

	metadata := redactor.SanitizeMetadata(map[string]any{
		"safe": "keep",
		"nested": map[string]any{
			apiKey:                 "secret key name must be dropped",
			"error_body_" + apiKey: "secret key fragment must be dropped",
			"safe":                 "keep nested",
		},
		"headers": map[string]any{
			"Authorization": "Bearer " + apiKey,
			"Cookie":        "studio_auth=" + apiKey,
		},
		"images": []any{
			map[string]any{"b64_json": "data:image/png;base64,AAAA"},
			map[string]any{"inline_data": map[string]any{"mime_type": "image/png", "data": "AAAA"}},
		},
		"message": "provider echoed " + apiKey,
	})

	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{strings.ToLower(apiKey), "authorization", "cookie", "b64_json", "inline_data", "data:image", "base64"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("runtime metadata leaked %q: %s", forbidden, encoded)
		}
	}
	nested, ok := metadata["nested"].(map[string]any)
	if !ok || len(nested) != 1 || nested["safe"] != "keep nested" {
		t.Fatalf("nested metadata = %#v, want only safe entry", metadata["nested"])
	}
	if metadata["safe"] != "keep" {
		t.Fatalf("safe metadata was removed: %#v", metadata)
	}
}
