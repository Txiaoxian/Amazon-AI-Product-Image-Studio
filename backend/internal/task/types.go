package task

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/thumbnail"
)

const (
	TypeImageGeneration = "IMAGE_GENERATION"
	TypeImageEdit       = "IMAGE_EDIT"

	StatusQueued    = "QUEUED"
	StatusRunning   = "RUNNING"
	StatusSucceeded = "SUCCEEDED"
	StatusFailed    = "FAILED"
	StatusCancelled = "CANCELLED"
	StatusRetrying  = "RETRYING"
	StatusTimedOut  = "TIMED_OUT"

	EventTaskQueued    = "TASK_QUEUED"
	EventTaskStarted   = "TASK_STARTED"
	EventTaskProgress  = "TASK_PROGRESS"
	EventImageOutput   = "IMAGE_OUTPUT"
	EventUsageRecorded = "USAGE_RECORDED"
	EventTaskFailed    = "TASK_FAILED"
	EventTaskCompleted = "TASK_COMPLETED"
	EventTaskCancelled = "TASK_CANCELLED"
	EventTaskRetried   = "TASK_RETRIED"
	EventTaskTimedOut  = "TASK_TIMED_OUT"

	PermissionRead   = "task:read"
	PermissionCreate = "task:create"
	PermissionCancel = "task:cancel"
	PermissionRetry  = "task:retry"

	defaultPageNum         = 1
	defaultPageSize        = 20
	maxPageSize            = 100
	defaultMaxAttempts     = 3
	defaultTaskTimeout     = 30 * time.Minute
	maxPromptRunes         = 10000
	maxImageTypeRunes      = 64
	maxReferenceAssetIDs   = 16
	maxParameterKeyRunes   = 64
	maxParameterValueRunes = 128
)

var (
	ErrValidation        = errors.New("invalid task request")
	ErrMalformedRequest  = errors.New("malformed task request")
	ErrForbidden         = errors.New("task access forbidden")
	ErrNotFound          = errors.New("task not found")
	ErrConflict          = errors.New("task state conflict")
	ErrQueueUnavailable  = errors.New("task queue unavailable")
	ErrInvalidTransition = errors.New("invalid task status transition")
)

type ListQuery struct {
	PageNum  int
	PageSize int
	Status   string
	Type     string
}

type ListOptions struct {
	PageNum  int
	PageSize int
	Status   string
	Type     string
}

type HistoryQuery struct {
	PageNum  int
	PageSize int
	Kind     string
}

type HistoryOptions struct {
	PageNum  int
	PageSize int
	Kind     string
}

type Page struct {
	Records  []Response `json:"records"`
	Total    int64      `json:"total"`
	PageNum  int        `json:"pageNum"`
	PageSize int        `json:"pageSize"`
}

type HistoryPage struct {
	Records  []HistoryRecord `json:"records"`
	Total    int64           `json:"total"`
	PageNum  int             `json:"pageNum"`
	PageSize int             `json:"pageSize"`
}

type HistoryRecord struct {
	Asset AssetResponse `json:"asset"`
	Task  Response      `json:"task"`
}

type AssetResponse struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenantId"`
	ProjectID    string `json:"projectId"`
	Kind         string `json:"kind"`
	Category     string `json:"category"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mimeType"`
	FileSize     int64  `json:"fileSize"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	ThumbnailURL string `json:"thumbnailUrl"`
	PreviewURL   string `json:"previewUrl"`
	IsFavorite   bool   `json:"isFavorite"`
	CreatedBy    string `json:"createdBy"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type CreateInput struct {
	Type          string
	Prompt        string
	ProviderID    string
	ModelID       string
	ImageType     string
	InputAssetIDs []string
	Parameters    map[string]any
}

type Response struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenantId"`
	ProjectID      string         `json:"projectId"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	Prompt         string         `json:"prompt"`
	ProviderID     string         `json:"providerId"`
	ModelID        string         `json:"modelId"`
	ImageType      string         `json:"imageType"`
	Parameters     map[string]any `json:"parameters"`
	InputAssetIDs  []string       `json:"inputAssetIds"`
	OutputAssetIDs []string       `json:"outputAssetIds"`
	Attempt        int            `json:"attempt"`
	MaxAttempts    int            `json:"maxAttempts"`
	QueuedAt       *string        `json:"queuedAt"`
	StartedAt      *string        `json:"startedAt"`
	FinishedAt     *string        `json:"finishedAt"`
	TimeoutAt      *string        `json:"timeoutAt"`
	ErrorCode      string         `json:"errorCode"`
	ErrorMessage   string         `json:"errorMessage"`
	CreatedBy      string         `json:"createdBy"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
}

func responseFromRecord(record database.GenerationTask, outputAssetIDs []string) (Response, error) {
	var parameters map[string]any
	if err := json.Unmarshal([]byte(record.ParamsJSON), &parameters); err != nil {
		return Response{}, err
	}
	if parameters == nil {
		parameters = map[string]any{}
	}

	var inputAssetIDs []string
	if err := json.Unmarshal([]byte(record.InputAssetIDsJSON), &inputAssetIDs); err != nil {
		return Response{}, err
	}
	if inputAssetIDs == nil {
		inputAssetIDs = []string{}
	}
	if outputAssetIDs == nil {
		outputAssetIDs = []string{}
	}

	return Response{
		ID:             record.ID,
		TenantID:       record.TenantID,
		ProjectID:      record.ProjectID,
		Type:           record.Type,
		Status:         record.Status,
		Prompt:         record.Prompt,
		ProviderID:     record.ProviderID,
		ModelID:        record.ModelID,
		ImageType:      record.ImageType,
		Parameters:     parameters,
		InputAssetIDs:  inputAssetIDs,
		OutputAssetIDs: outputAssetIDs,
		Attempt:        record.Attempt,
		MaxAttempts:    record.MaxAttempts,
		QueuedAt:       optionalTime(record.QueuedAt),
		StartedAt:      optionalTime(record.StartedAt),
		FinishedAt:     optionalTime(record.FinishedAt),
		TimeoutAt:      optionalTime(record.TimeoutAt),
		ErrorCode:      record.ErrorCode,
		ErrorMessage:   record.ErrorMessage,
		CreatedBy:      record.CreatedBy,
		CreatedAt:      formatTime(record.CreatedAt),
		UpdatedAt:      formatTime(record.UpdatedAt),
	}, nil
}

func assetResponseFromRecord(record database.ImageAsset) AssetResponse {
	return AssetResponse{
		ID:           record.ID,
		TenantID:     record.TenantID,
		ProjectID:    record.ProjectID,
		Kind:         record.Kind,
		Category:     record.Category,
		Filename:     record.Filename,
		MimeType:     record.MimeType,
		FileSize:     record.SizeBytes,
		Width:        record.Width,
		Height:       record.Height,
		ThumbnailURL: thumbnailURL(record),
		PreviewURL:   "/api/v1/assets/" + record.ID + "/download",
		IsFavorite:   record.IsFavorite,
		CreatedBy:    record.CreatedBy,
		CreatedAt:    formatTime(record.CreatedAt),
		UpdatedAt:    formatTime(record.UpdatedAt),
	}
}

func thumbnailURL(record database.ImageAsset) string {
	if record.ThumbnailObjectKey == nil || strings.TrimSpace(*record.ThumbnailObjectKey) == "" {
		return ""
	}
	return thumbnail.URL(record.ID)
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
