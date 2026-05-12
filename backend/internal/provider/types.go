package provider

import (
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

const (
	TypeOpenAI           = "OPENAI"
	TypeGemini           = "GEMINI"
	TypeOpenAICompatible = "OPENAI_COMPATIBLE"

	StatusEnabled  = "ENABLED"
	StatusDisabled = "DISABLED"

	TestStatusSuccess = "SUCCESS"
	TestStatusFailure = "FAILURE"

	PermissionRead   = "provider:read"
	PermissionManage = "provider:manage"

	defaultOpenAIBaseURL  = "https://api.openai.com/v1"
	defaultGeminiBaseURL  = "https://generativelanguage.googleapis.com/v1beta"
	maxProviderNameRunes  = 255
	maxProviderURLRunes   = 512
	minTimeoutSeconds     = 1
	maxTimeoutSeconds     = 300
	maxConcurrencyLimit   = 1000
	maxProbeMessageRunes  = 255
	defaultRequestTimeout = 120
)

var (
	ErrValidation       = errors.New("invalid provider request")
	ErrForbidden        = errors.New("provider access forbidden")
	ErrNotFound         = errors.New("provider not found")
	ErrEncryption       = errors.New("provider api key encryption failed")
	ErrProviderTest     = errors.New("provider test failed")
	ErrProbeUnavailable = errors.New("provider probe unavailable")
)

type ListQuery struct {
	PageNum  int
	PageSize int
	Type     string
	Status   string
}

type ListOptions struct {
	PageNum  int
	PageSize int
	Type     string
	Status   string
}

type Page struct {
	Records  []Response `json:"records"`
	Total    int64      `json:"total"`
	PageNum  int        `json:"pageNum"`
	PageSize int        `json:"pageSize"`
}

type Response struct {
	ID               string  `json:"id"`
	TenantID         string  `json:"tenantId"`
	Type             string  `json:"type"`
	Name             string  `json:"name"`
	BaseURL          string  `json:"baseUrl"`
	Status           string  `json:"status"`
	TimeoutSeconds   int     `json:"timeoutSeconds"`
	ConcurrencyLimit int     `json:"concurrencyLimit"`
	APIKeyHint       string  `json:"apiKeyHint"`
	APIKeyUpdatedAt  *string `json:"apiKeyUpdatedAt"`
	LastTestStatus   string  `json:"lastTestStatus"`
	LastTestedAt     *string `json:"lastTestedAt"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

type CreateInput struct {
	Type             string
	Name             string
	BaseURL          string
	APIKey           string
	Status           string
	TimeoutSeconds   int
	ConcurrencyLimit int
}

type UpdateInput struct {
	Name             *string
	BaseURL          *string
	APIKey           *string
	Status           *string
	TimeoutSeconds   *int
	ConcurrencyLimit *int
}

type TestResponse struct {
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
	CheckedAt  string `json:"checkedAt"`
	HTTPStatus *int   `json:"httpStatus"`
	RequestID  string `json:"requestId"`
	Message    string `json:"message"`
}

func responseFromRecord(record database.AIProvider) Response {
	return Response{
		ID:               record.ID,
		TenantID:         record.TenantID,
		Type:             record.Type,
		Name:             record.Name,
		BaseURL:          record.BaseURL,
		Status:           record.Status,
		TimeoutSeconds:   record.TimeoutSeconds,
		ConcurrencyLimit: record.ConcurrencyLimit,
		APIKeyHint:       record.APIKeyHint,
		APIKeyUpdatedAt:  optionalTime(record.APIKeyUpdatedAt),
		LastTestStatus:   record.LastTestStatus,
		LastTestedAt:     optionalTime(record.LastTestedAt),
		CreatedAt:        formatTime(record.CreatedAt),
		UpdatedAt:        formatTime(record.UpdatedAt),
	}
}

func optionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func validType(providerType string) bool {
	switch providerType {
	case TypeOpenAI, TypeGemini, TypeOpenAICompatible:
		return true
	default:
		return false
	}
}

func validStatus(status string) bool {
	switch status {
	case StatusEnabled, StatusDisabled:
		return true
	default:
		return false
	}
}

func defaultBaseURL(providerType string) string {
	switch providerType {
	case TypeOpenAI:
		return defaultOpenAIBaseURL
	case TypeGemini:
		return defaultGeminiBaseURL
	default:
		return ""
	}
}

func cleanEnum(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
