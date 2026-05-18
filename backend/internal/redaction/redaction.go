package redaction

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const RedactedValue = "[REDACTED]"
const minSecretRunes = 8

type Redactor struct {
	secrets []string
}

func New(secrets ...string) *Redactor {
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
	return New().RedactValue(value)
}

func (r *Redactor) RedactValue(value any) any {
	r = ensure(r)
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
	case map[string]string:
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
	return New().RedactString(value)
}

func (r *Redactor) RedactString(value string) string {
	r = ensure(r)
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = r.redactKnownSecrets(value)
	if looksSensitiveString(strings.ToLower(value)) {
		return RedactedValue
	}
	if utf8.RuneCountInString(value) > 512 {
		return string([]rune(value)[:512])
	}
	return value
}

func SanitizeErrorMessage(value string) string {
	return New().SanitizeErrorMessage(value)
}

func (r *Redactor) SanitizeErrorMessage(value string) string {
	r = ensure(r)
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
	return New().SanitizeMetadata(value)
}

func (r *Redactor) SanitizeMetadata(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	r = ensure(r)
	redacted, ok := r.RedactValue(value).(map[string]any)
	if !ok || redacted == nil {
		return map[string]any{}
	}
	return redacted
}

func JSONString(value map[string]any) string {
	return New().JSONString(value)
}

func (r *Redactor) JSONString(value map[string]any) string {
	if value == nil {
		value = map[string]any{}
	}
	r = ensure(r)
	encoded, err := json.Marshal(r.SanitizeMetadata(value))
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func (r *Redactor) redactKnownSecrets(value string) string {
	r = ensure(r)
	for _, secret := range r.secrets {
		value = strings.ReplaceAll(value, secret, RedactedValue)
	}
	return value
}

func ensure(redactor *Redactor) *Redactor {
	if redactor == nil {
		return New()
	}
	return redactor
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
	return len(lower) > 2048
}
