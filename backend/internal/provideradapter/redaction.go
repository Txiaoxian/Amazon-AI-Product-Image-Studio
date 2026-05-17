package provideradapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const redactedValue = "[REDACTED]"
const minSecretRunes = 8

type Redactor struct {
	secrets []string
}

func NewRedactor(secrets ...string) *Redactor {
	seen := map[string]struct{}{}
	redactor := &Redactor{}
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if utf8.RuneCountInString(secret) < minSecretRunes {
			continue
		}
		if _, ok := seen[secret]; ok {
			continue
		}
		seen[secret] = struct{}{}
		redactor.secrets = append(redactor.secrets, secret)
	}
	return redactor
}

func RedactValue(value any) any {
	return NewRedactor().RedactValue(value)
}

func (r *Redactor) RedactValue(value any) any {
	if r == nil {
		r = NewRedactor()
	}
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, item := range typed {
			if sensitiveKey(key) || r.RedactString(key) != key {
				continue
			}
			clean[key] = r.RedactValue(item)
		}
		return clean
	case []any:
		clean := make([]any, 0, len(typed))
		for _, item := range typed {
			clean = append(clean, r.RedactValue(item))
		}
		return clean
	case []string:
		clean := make([]any, 0, len(typed))
		for _, item := range typed {
			clean = append(clean, r.RedactValue(item))
		}
		return clean
	case string:
		return r.RedactString(typed)
	default:
		return value
	}
}

func RedactString(value string) string {
	return NewRedactor().RedactString(value)
}

func (r *Redactor) RedactString(value string) string {
	if r == nil {
		r = NewRedactor()
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = r.redactKnownSecrets(value)
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
	return NewRedactor().SanitizeErrorMessage(value)
}

func (r *Redactor) SanitizeErrorMessage(value string) string {
	if r == nil {
		r = NewRedactor()
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "Provider request failed."
	}
	value = r.redactKnownSecrets(value)
	if looksSensitiveString(strings.ToLower(value)) {
		return "Provider error message redacted."
	}
	if utf8.RuneCountInString(value) > 512 {
		value = string([]rune(value)[:512])
	}
	return value
}

func SanitizeMetadata(value map[string]any) map[string]any {
	return NewRedactor().SanitizeMetadata(value)
}

func (r *Redactor) SanitizeMetadata(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if r == nil {
		r = NewRedactor()
	}
	redacted, ok := r.RedactValue(value).(map[string]any)
	if !ok || redacted == nil {
		return map[string]any{}
	}
	return redacted
}

func JSONString(value map[string]any) string {
	return NewRedactor().JSONString(value)
}

func (r *Redactor) JSONString(value map[string]any) string {
	if value == nil {
		value = map[string]any{}
	}
	if r == nil {
		r = NewRedactor()
	}
	encoded, err := json.Marshal(r.SanitizeMetadata(value))
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func (r *Redactor) SanitizeAPICall(call APICall) APICall {
	if r == nil {
		r = NewRedactor()
	}
	if strings.TrimSpace(call.ErrorMessage) != "" {
		call.ErrorMessage = r.SanitizeErrorMessage(call.ErrorMessage)
	}
	call.RequestMetadata = r.SanitizeMetadata(call.RequestMetadata)
	call.ResponseMetadata = r.SanitizeMetadata(call.ResponseMetadata)
	return call
}

func (r *Redactor) redactKnownSecrets(value string) string {
	if r == nil {
		return value
	}
	for _, secret := range r.secrets {
		value = strings.ReplaceAll(value, secret, redactedValue)
	}
	return value
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

func providerHTTPError(status int, body []byte, redactor *Redactor) ProviderError {
	if redactor == nil {
		redactor = NewRedactor()
	}
	message := fmt.Sprintf("Provider returned HTTP %d.", status)
	if len(body) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err == nil {
			if errorValue, ok := parsed["error"]; ok {
				message = fmt.Sprint(redactor.RedactValue(errorValue))
			} else if messageValue, ok := parsed["message"]; ok {
				message = fmt.Sprint(redactor.RedactValue(messageValue))
			}
		}
	}
	message = redactor.SanitizeErrorMessage(message)
	return ProviderError{
		Code:       "PROVIDER_HTTP_ERROR",
		Message:    message,
		HTTPStatus: statusCodePointer(status),
		Retryable:  status == 429 || status >= 500,
	}
}
