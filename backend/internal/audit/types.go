package audit

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/redaction"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/usagecost"
)

const (
	PermissionUsageRead = "usage:read"
	PermissionAuditRead = "audit:read"

	UsageSummaryDimensionTenant   = "tenant"
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
	ImageType  string
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
	CostStatus    string `json:"costStatus"`
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
	ActorName    string  `json:"actorName"`
	ActorEmail   string  `json:"actorEmail"`
	Action       string  `json:"action"`
	ResourceType string  `json:"resourceType"`
	ResourceID   string  `json:"resourceId"`
	IP           string  `json:"ip"`
	UserAgent    string  `json:"userAgent"`
	Metadata     any     `json:"metadata"`
	CreatedAt    string  `json:"createdAt"`
}

type OperationLogRecord struct {
	ID           string
	TenantID     string
	ActorUserID  *string
	ActorName    string
	ActorEmail   string
	Action       string
	ResourceType string
	ResourceID   string
	IP           string
	UserAgent    string
	MetadataJSON string
	CreatedAt    time.Time
}

type APICallLogResponse struct {
	ID               string `json:"id"`
	TenantID         string `json:"tenantId"`
	TaskID           string `json:"taskId"`
	ProviderID       string `json:"providerId"`
	ProviderName     string `json:"providerName"`
	ModelID          string `json:"modelId"`
	ModelName        string `json:"modelName"`
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

type APICallLogRecord struct {
	database.APICallLog `gorm:"embedded"`
	ProviderName        string
	ModelName           string
}

func UsageRecordResponseFromRecord(record database.UsageRecord, redactor *redaction.Redactor) UsageRecordResponse {
	redactor = ensureRedactor(redactor)
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
		CostStatus:    record.CostStatus,
		RawUsage:      redactJSONPayload(record.RawUsageJSON, redactor),
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

func OperationLogResponseFromRecord(record OperationLogRecord, redactor *redaction.Redactor) OperationLogResponse {
	redactor = ensureRedactor(redactor)
	return OperationLogResponse{
		ID:           record.ID,
		TenantID:     record.TenantID,
		ActorUserID:  record.ActorUserID,
		ActorName:    record.ActorName,
		ActorEmail:   record.ActorEmail,
		Action:       record.Action,
		ResourceType: record.ResourceType,
		ResourceID:   record.ResourceID,
		IP:           record.IP,
		UserAgent:    record.UserAgent,
		Metadata:     redactJSONPayload(record.MetadataJSON, redactor),
		CreatedAt:    formatTime(record.CreatedAt),
	}
}

func APICallLogResponseFromRecord(record APICallLogRecord, redactor *redaction.Redactor) APICallLogResponse {
	redactor = ensureRedactor(redactor)
	errorMessage := ""
	if strings.TrimSpace(record.ErrorMessage) != "" {
		errorMessage = redactor.SanitizeErrorMessage(record.ErrorMessage)
	}
	return APICallLogResponse{
		ID:               record.ID,
		TenantID:         record.TenantID,
		TaskID:           record.TaskID,
		ProviderID:       record.ProviderID,
		ProviderName:     record.ProviderName,
		ModelID:          record.ModelID,
		ModelName:        record.ModelName,
		Status:           record.Status,
		DurationMs:       record.DurationMs,
		RequestID:        redactor.RedactString(record.RequestID),
		HTTPStatus:       record.HTTPStatus,
		ErrorCode:        redactor.RedactString(record.ErrorCode),
		ErrorMessage:     errorMessage,
		RedactedRequest:  redactJSONPayload(record.RedactedRequestJSON, redactor),
		RedactedResponse: redactJSONPayload(record.RedactedResponseJSON, redactor),
		CreatedAt:        formatTime(record.CreatedAt),
	}
}

func ValidUsageSummaryDimension(dimension string) bool {
	switch dimension {
	case UsageSummaryDimensionTenant, UsageSummaryDimensionUser, UsageSummaryDimensionProject, UsageSummaryDimensionProvider, UsageSummaryDimensionModel:
		return true
	default:
		return false
	}
}

func redactJSONPayload(raw string, redactor *redaction.Redactor) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return map[string]any{}
	}
	redacted := ensureRedactor(redactor).RedactValue(decoded)
	if redacted == nil {
		return map[string]any{}
	}
	return redacted
}

func ensureRedactor(redactor *redaction.Redactor) *redaction.Redactor {
	if redactor == nil {
		return redaction.New()
	}
	return redactor
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
	return usagecost.FormatDecimal8(value)
}
