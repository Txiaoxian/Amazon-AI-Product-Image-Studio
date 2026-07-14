package provideradapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/redaction"
)

const redactedValue = redaction.RedactedValue

type Redactor struct {
	shared *redaction.Redactor
}

func NewRedactor(secrets ...string) *Redactor {
	return &Redactor{shared: redaction.New(secrets...)}
}

func RedactValue(value any) any {
	return redaction.RedactValue(value)
}

func (r *Redactor) RedactValue(value any) any {
	return r.sharedRedactor().RedactValue(value)
}

func RedactString(value string) string {
	return redaction.RedactString(value)
}

func (r *Redactor) RedactString(value string) string {
	return r.sharedRedactor().RedactString(value)
}

func SanitizeErrorMessage(value string) string {
	return redaction.SanitizeErrorMessage(value)
}

func (r *Redactor) SanitizeErrorMessage(value string) string {
	return r.sharedRedactor().SanitizeErrorMessage(value)
}

func SanitizeMetadata(value map[string]any) map[string]any {
	return redaction.SanitizeMetadata(value)
}

func (r *Redactor) SanitizeMetadata(value map[string]any) map[string]any {
	return r.sharedRedactor().SanitizeMetadata(value)
}

func JSONString(value map[string]any) string {
	return redaction.JSONString(value)
}

func (r *Redactor) JSONString(value map[string]any) string {
	return r.sharedRedactor().JSONString(value)
}

func (r *Redactor) SanitizeAPICall(call APICall) APICall {
	if strings.TrimSpace(call.ErrorMessage) != "" {
		call.ErrorMessage = r.SanitizeErrorMessage(call.ErrorMessage)
	}
	call.RequestMetadata = r.SanitizeMetadata(call.RequestMetadata)
	call.ResponseMetadata = r.SanitizeMetadata(call.ResponseMetadata)
	return call
}

func (r *Redactor) sharedRedactor() *redaction.Redactor {
	if r == nil || r.shared == nil {
		return redaction.New()
	}
	return r.shared
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
	providerCode := ""
	if len(body) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err == nil {
			if errorValue, ok := parsed["error"].(map[string]any); ok {
				providerCode = stringValue(errorValue["code"])
				if providerMessage := stringValue(errorValue["message"]); providerMessage != "" {
					message = providerMessage
				}
			} else if errorValue, ok := parsed["error"].(string); ok && strings.TrimSpace(errorValue) != "" {
				message = errorValue
			} else if messageValue := stringValue(parsed["message"]); messageValue != "" {
				message = messageValue
			}
		}
	}
	if isInsufficientQuotaCode(providerCode) {
		return ProviderError{
			Code:       "PROVIDER_INSUFFICIENT_QUOTA",
			Message:    "Provider account quota is insufficient.",
			HTTPStatus: statusCodePointer(status),
			Retryable:  false,
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

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func isInsufficientQuotaCode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "insufficient_user_quota", "insufficient_quota", "billing_hard_limit_reached":
		return true
	default:
		return false
	}
}
