package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

const (
	readRedactedValue = "[REDACTED]"

	PermissionUsageRead = "usage:read"
	PermissionAuditRead = "audit:read"

	UsageSummaryDimensionUser     = "user"
	UsageSummaryDimensionProject  = "project"
	UsageSummaryDimensionProvider = "provider"
	UsageSummaryDimensionModel    = "model"
)

var (
	ErrValidation = errors.New("invalid audit usage query")
	ErrForbidden  = errors.New("audit usage access forbidden")
	ErrNotFound   = errors.New("audit usage record not found")
)

type TimeRange struct {
	From        *time.Time
	To          *time.Time
	ToExclusive bool
}

type PageOptions struct {
	PageNum   int
	PageSize  int
	SortOrder string
	TimeRange TimeRange
}

type UsageRecordListOptions struct {
	PageOptions
	TaskID     string
	UserID     string
	ProjectID  string
	ProviderID string
	ModelID    string
}

type UsageSummaryOptions struct {
	UsageRecordListOptions
	Dimension string
}

type OperationLogListOptions struct {
	PageOptions
	ActorUserID  string
	Action       string
	ResourceType string
	ResourceID   string
}

type APICallLogListOptions struct {
	PageOptions
	TaskID     string
	UserID     string
	ProjectID  string
	ProviderID string
	ModelID    string
	Status     string
	RequestID  string
}

type UsageRecordPage struct {
	Records  []UsageRecordResponse `json:"records"`
	Total    int64                 `json:"total"`
	PageNum  int                   `json:"pageNum"`
	PageSize int                   `json:"pageSize"`
}

type UsageSummaryPage struct {
	Records  []UsageSummaryResponse `json:"records"`
	Total    int64                  `json:"total"`
	PageNum  int                    `json:"pageNum"`
	PageSize int                    `json:"pageSize"`
}

type OperationLogPage struct {
	Records  []OperationLogResponse `json:"records"`
	Total    int64                  `json:"total"`
	PageNum  int                    `json:"pageNum"`
	PageSize int                    `json:"pageSize"`
}

type APICallLogPage struct {
	Records  []APICallLogResponse `json:"records"`
	Total    int64                `json:"total"`
	PageNum  int                  `json:"pageNum"`
	PageSize int                  `json:"pageSize"`
}

type UsageSummaryRow struct {
	DimensionID     string
	Currency        string
	RecordCount     int64
	InputTokens     int64
	OutputTokens    int64
	ImageCount      int64
	EstimatedCost   string
	LatestCreatedAt string
}

type UsageRecordResponse struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenantId"`
	TaskID        string `json:"taskId"`
	UserID        string `json:"userId"`
	ProjectID     string `json:"projectId"`
	ProviderID    string `json:"providerId"`
	ModelID       string `json:"modelId"`
	InputTokens   int64  `json:"inputTokens"`
	OutputTokens  int64  `json:"outputTokens"`
	ImageCount    int    `json:"imageCount"`
	EstimatedCost string `json:"estimatedCost"`
	Currency      string `json:"currency"`
	RawUsage      any    `json:"rawUsage"`
	CreatedAt     string `json:"createdAt"`
}

type UsageSummaryResponse struct {
	Dimension       string `json:"dimension"`
	DimensionID     string `json:"dimensionId"`
	Currency        string `json:"currency"`
	RecordCount     int64  `json:"recordCount"`
	InputTokens     int64  `json:"inputTokens"`
	OutputTokens    int64  `json:"outputTokens"`
	ImageCount      int64  `json:"imageCount"`
	EstimatedCost   string `json:"estimatedCost"`
	LatestCreatedAt string `json:"latestCreatedAt"`
}

type OperationLogResponse struct {
	ID           string  `json:"id"`
	TenantID     string  `json:"tenantId"`
	ActorUserID  *string `json:"actorUserId"`
	Action       string  `json:"action"`
	ResourceType string  `json:"resourceType"`
	ResourceID   string  `json:"resourceId"`
	IP           string  `json:"ip"`
	UserAgent    string  `json:"userAgent"`
	Metadata     any     `json:"metadata"`
	CreatedAt    string  `json:"createdAt"`
}

type APICallLogResponse struct {
	ID               string `json:"id"`
	TenantID         string `json:"tenantId"`
	TaskID           string `json:"taskId"`
	ProviderID       string `json:"providerId"`
	ModelID          string `json:"modelId"`
	Status           string `json:"status"`
	DurationMs       int64  `json:"durationMs"`
	RequestID        string `json:"requestId"`
	HTTPStatus       *int   `json:"httpStatus"`
	ErrorCode        string `json:"errorCode"`
	ErrorMessage     string `json:"errorMessage"`
	RedactedRequest  any    `json:"redactedRequest"`
	RedactedResponse any    `json:"redactedResponse"`
	CreatedAt        string `json:"createdAt"`
}

func UsageRecordResponseFromRecord(record database.UsageRecord) UsageRecordResponse {
	return UsageRecordResponse{
		ID:            record.ID,
		TenantID:      record.TenantID,
		TaskID:        record.TaskID,
		UserID:        record.UserID,
		ProjectID:     record.ProjectID,
		ProviderID:    record.ProviderID,
		ModelID:       record.ModelID,
		InputTokens:   record.InputTokens,
		OutputTokens:  record.OutputTokens,
		ImageCount:    record.ImageCount,
		EstimatedCost: record.EstimatedCost,
		Currency:      record.Currency,
		RawUsage:      redactJSONPayload(record.RawUsageJSON),
		CreatedAt:     formatTime(record.CreatedAt),
	}
}

func UsageSummaryResponseFromRow(dimension string, row UsageSummaryRow) UsageSummaryResponse {
	return UsageSummaryResponse{
		Dimension:       dimension,
		DimensionID:     row.DimensionID,
		Currency:        row.Currency,
		RecordCount:     row.RecordCount,
		InputTokens:     row.InputTokens,
		OutputTokens:    row.OutputTokens,
		ImageCount:      row.ImageCount,
		EstimatedCost:   formatDecimal(row.EstimatedCost),
		LatestCreatedAt: formatTimeString(row.LatestCreatedAt),
	}
}

func OperationLogResponseFromRecord(record database.OperationLog) OperationLogResponse {
	return OperationLogResponse{
		ID:           record.ID,
		TenantID:     record.TenantID,
		ActorUserID:  record.ActorUserID,
		Action:       record.Action,
		ResourceType: record.ResourceType,
		ResourceID:   record.ResourceID,
		IP:           record.IP,
		UserAgent:    record.UserAgent,
		Metadata:     redactJSONPayload(record.MetadataJSON),
		CreatedAt:    formatTime(record.CreatedAt),
	}
}

func APICallLogResponseFromRecord(record database.APICallLog) APICallLogResponse {
	errorMessage := ""
	if strings.TrimSpace(record.ErrorMessage) != "" {
		errorMessage = sanitizeErrorMessage(record.ErrorMessage)
	}
	return APICallLogResponse{
		ID:               record.ID,
		TenantID:         record.TenantID,
		TaskID:           record.TaskID,
		ProviderID:       record.ProviderID,
		ModelID:          record.ModelID,
		Status:           record.Status,
		DurationMs:       record.DurationMs,
		RequestID:        redactString(record.RequestID),
		HTTPStatus:       record.HTTPStatus,
		ErrorCode:        redactString(record.ErrorCode),
		ErrorMessage:     errorMessage,
		RedactedRequest:  redactJSONPayload(record.RedactedRequestJSON),
		RedactedResponse: redactJSONPayload(record.RedactedResponseJSON),
		CreatedAt:        formatTime(record.CreatedAt),
	}
}

func ValidUsageSummaryDimension(dimension string) bool {
	switch dimension {
	case UsageSummaryDimensionUser, UsageSummaryDimensionProject, UsageSummaryDimensionProvider, UsageSummaryDimensionModel:
		return true
	default:
		return false
	}
}

func redactJSONPayload(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return map[string]any{}
	}
	redacted := redactValue(decoded)
	if redacted == nil {
		return map[string]any{}
	}
	return redacted
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, item := range typed {
			if readSensitiveKey(key) || redactString(key) != key {
				continue
			}
			clean[key] = redactValue(item)
		}
		return clean
	case map[string]string:
		clean := make(map[string]any, len(typed))
		for key, item := range typed {
			if readSensitiveKey(key) || redactString(key) != key {
				continue
			}
			clean[key] = redactValue(item)
		}
		return clean
	case []any:
		clean := make([]any, 0, len(typed))
		for _, item := range typed {
			clean = append(clean, redactValue(item))
		}
		return clean
	case []string:
		clean := make([]any, 0, len(typed))
		for _, item := range typed {
			clean = append(clean, redactValue(item))
		}
		return clean
	case string:
		return redactString(typed)
	default:
		return value
	}
}

func redactString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if looksSensitiveString(strings.ToLower(value)) {
		return readRedactedValue
	}
	if utf8.RuneCountInString(value) > 512 {
		return string([]rune(value)[:512])
	}
	return value
}

func sanitizeErrorMessage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if looksSensitiveString(strings.ToLower(value)) {
		return "Provider error message redacted."
	}
	if utf8.RuneCountInString(value) > 512 {
		return string([]rune(value)[:512])
	}
	return value
}

func readSensitiveKey(key string) bool {
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

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatTimeString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	return value
}

func formatDecimal(value string) string {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 {
		return "0.00000000"
	}
	return fmt.Sprintf("%.8f", parsed)
}
