package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeMetadataRecursivelyRemovesSensitiveKeys(t *testing.T) {
	clean := sanitizeMetadata(map[string]any{
		"result": "success",
		"headers": map[string]any{
			"Authorization": "Bearer secret",
			"safe":          "ok",
		},
		"items": []any{
			map[string]any{
				"apiKey": "sk-secret",
				"name":   "provider",
			},
		},
		"nestedStrings": map[string]string{
			"cookie": "studio_auth=secret",
			"note":   "kept",
		},
	})

	encoded, err := json.Marshal(clean)
	if err != nil {
		t.Fatalf("marshal sanitized metadata: %v", err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"authorization", "bearer secret", "apikey", "sk-secret", "cookie", "studio_auth"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("sanitized metadata contains %q: %s", forbidden, string(encoded))
		}
	}
	for _, expected := range []string{"success", "safe", "provider", "kept"} {
		if !strings.Contains(lower, expected) {
			t.Fatalf("sanitized metadata missing %q: %s", expected, string(encoded))
		}
	}
}
