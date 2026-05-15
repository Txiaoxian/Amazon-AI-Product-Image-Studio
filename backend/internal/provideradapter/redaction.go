package provideradapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const redactedValue = "[REDACTED]"

func RedactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, item := range typed {
			if sensitiveKey(key) {
				continue
			}
			clean[key] = RedactValue(item)
		}
		return clean
	case []any:
		clean := make([]any, 0, len(typed))
		for _, item := range typed {
			clean = append(clean, RedactValue(item))
		}
		return clean
	case []string:
		clean := make([]any, 0, len(typed))
		for _, item := range typed {
			clean = append(clean, RedactValue(item))
		}
		return clean
	case string:
		return RedactString(typed)
	default:
		return value
	}
}

func RedactString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if looksSensitiveString(lower) {
		return redactedValue
	}
	if utf8.RuneCountInString(value) > 512 {
		return string([]rune(value)[:512])
	}
	return value
}

func SanitizeErrorMessage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Provider request failed."
	}
	if looksSensitiveString(strings.ToLower(value)) {
		return "Provider error message redacted."
	}
	if utf8.RuneCountInString(value) > 512 {
		value = string([]rune(value)[:512])
	}
	return value
}

func SanitizeMetadata(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	redacted, ok := RedactValue(value).(map[string]any)
	if !ok || redacted == nil {
		return map[string]any{}
	}
	return redacted
}

func JSONString(value map[string]any) string {
	if value == nil {
		value = map[string]any{}
	}
	encoded, err := json.Marshal(SanitizeMetadata(value))
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func ErrorCode(value string, fallback string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		value = fallback
	}
	if value == "" {
		value = "PROVIDER_ERROR"
	}
	if len(value) > 128 {
		value = value[:128]
	}
	return value
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{
		"authorization",
		"cookie",
		"api_key",
		"apikey",
		"api-key",
		"secret",
		"password",
		"token",
		"bearer",
		"b64_json",
		"base64",
		"inline_data",
		"inlinedata",
		"bytes",
		"raw",
	} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func looksSensitiveString(lower string) bool {
	if strings.Contains(lower, "sk-") || strings.Contains(lower, "bearer ") {
		return true
	}
	for _, marker := range []string{
		"authorization",
		"cookie",
		"api_key",
		"apikey",
		"secret",
		"base64",
		"b64_json",
		"-----begin",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	if len(lower) > 2048 {
		return true
	}
	return false
}

func cleanRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, r := range value {
		if r < 33 || r > 126 {
			return ""
		}
	}
	return value
}

func statusCodePointer(status int) *int {
	copied := status
	return &copied
}

func providerHTTPError(status int, body []byte) ProviderError {
	message := fmt.Sprintf("Provider returned HTTP %d.", status)
	if len(body) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err == nil {
			if errorValue, ok := parsed["error"]; ok {
				message = fmt.Sprint(RedactValue(errorValue))
			} else if messageValue, ok := parsed["message"]; ok {
				message = fmt.Sprint(RedactValue(messageValue))
			}
		}
	}
	message = SanitizeErrorMessage(message)
	return ProviderError{
		Code:       "PROVIDER_HTTP_ERROR",
		Message:    message,
		HTTPStatus: statusCodePointer(status),
		Retryable:  status == 429 || status >= 500,
	}
}
