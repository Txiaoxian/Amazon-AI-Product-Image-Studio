package task

import (
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provideradapter"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/redaction"
)

func sanitizeProviderRuntimeMetadata(value map[string]any, redactor *provideradapter.Redactor) map[string]any {
	if redactor == nil {
		redactor = provideradapter.NewRedactor()
	}
	clean := redactor.SanitizeMetadata(value)
	filtered, ok := filterProviderRuntimeValue(clean).(map[string]any)
	if !ok || filtered == nil {
		return map[string]any{}
	}
	return filtered
}

func filterProviderRuntimeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, item := range typed {
			if forbiddenProviderMetadataKey(key) {
				continue
			}
			clean[key] = filterProviderRuntimeValue(item)
		}
		return clean
	case []any:
		clean := make([]any, 0, len(typed))
		for _, item := range typed {
			clean = append(clean, filterProviderRuntimeValue(item))
		}
		return clean
	case string:
		return filterProviderRuntimeString(typed)
	default:
		return value
	}
}

func forbiddenProviderMetadataKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.NewReplacer("_", "", "-", "").Replace(normalized)
	for _, marker := range []string{
		"objectkey",
		"bucket",
		"signedurl",
		"presignedurl",
		"downloadurl",
		"minio",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func filterProviderRuntimeString(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(lower, "x-amz-signature") ||
		strings.Contains(lower, "tenants/") ||
		strings.Contains(lower, "minio") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") {
		return redaction.RedactedValue
	}
	return value
}
