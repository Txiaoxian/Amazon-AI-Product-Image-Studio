package task

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/idgen"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provideradapter"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

const providerAttemptFinalizeTimeout = 5 * time.Second

type providerAttemptLedger struct {
	record          database.APICallLog
	requestMetadata map[string]any
}

func (e *ProviderRuntimeExecutor) beginProviderAttemptLedger(ctx context.Context, execution ExecutionContext, request provideradapter.ImageRequest, redactor *provideradapter.Redactor) (providerAttemptLedger, error) {
	scope, err := tenant.NewScope(execution.Task.TenantID)
	if err != nil {
		return providerAttemptLedger{}, err
	}
	startedAt := e.now()
	requestMetadata := providerAttemptRequestMetadata(execution, request, startedAt)
	record := database.APICallLog{
		ID:                   idgen.New(),
		TenantID:             scope.ID(),
		TaskID:               execution.Task.ID,
		ProviderID:           execution.Provider.ID,
		ModelID:              execution.Model.ID,
		Status:               provideradapter.APICallStatusAttempting,
		DurationMs:           0,
		RedactedRequestJSON:  provideradapter.JSONString(sanitizeProviderRuntimeMetadata(requestMetadata, redactor)),
		RedactedResponseJSON: "{}",
		CreatedAt:            startedAt,
	}
	if err := e.db.WithContext(ctx).Create(&record).Error; err != nil {
		return providerAttemptLedger{}, err
	}
	return providerAttemptLedger{record: record, requestMetadata: requestMetadata}, nil
}

func (e *ProviderRuntimeExecutor) finalizeProviderAttemptLedger(attempt providerAttemptLedger, call APICallResult, ctxErr error, redactor *provideradapter.Redactor) (APICallResult, error) {
	status := finalProviderAttemptStatus(call, ctxErr)
	durationMs := nonNegativeInt64(call.DurationMs)
	if durationMs == 0 && !attempt.record.CreatedAt.IsZero() {
		durationMs = nonNegativeInt64(e.now().Sub(attempt.record.CreatedAt).Milliseconds())
	}
	errorCode := cleanWorkerCode(call.ErrorCode, "")
	errorMessage := cleanWorkerMessage(provideradapter.SanitizeErrorMessage(call.ErrorMessage))
	if status == provideradapter.APICallStatusTimeout {
		errorCode = "TASK_TIMED_OUT"
		errorMessage = "Task execution timed out."
	}
	if status == provideradapter.APICallStatusCancelled {
		errorCode = "TASK_CANCELLED"
		errorMessage = "Task execution was cancelled."
	}
	if status == provideradapter.APICallStatusSuccess {
		errorCode = cleanWorkerCode(errorCode, "")
		if errorMessage == "Provider request failed." {
			errorMessage = ""
		}
	}

	requestMetadata := mergeProviderAttemptMetadata(attempt.requestMetadata, call.RequestMetadata)
	responseMetadata := mergeProviderAttemptMetadata(call.ResponseMetadata, map[string]any{
		"ledgerStatus": status,
		"finishedAt":   e.now().Format(time.RFC3339Nano),
		"durationMs":   durationMs,
	})
	if errorCode != "" || errorMessage != "" {
		responseMetadata["error"] = map[string]any{
			"code":    errorCode,
			"message": errorMessage,
		}
	}
	requestMetadata = sanitizeProviderRuntimeMetadata(requestMetadata, redactor)
	responseMetadata = sanitizeProviderRuntimeMetadata(responseMetadata, redactor)

	finalized := APICallResult{
		ID:               attempt.record.ID,
		Status:           status,
		DurationMs:       durationMs,
		RequestID:        cleanWorkerMessage(call.RequestID),
		HTTPStatus:       call.HTTPStatus,
		ErrorCode:        errorCode,
		ErrorMessage:     errorMessage,
		RequestMetadata:  requestMetadata,
		ResponseMetadata: responseMetadata,
	}
	finalizeCtx, cancel := context.WithTimeout(context.Background(), providerAttemptFinalizeTimeout)
	defer cancel()
	updates := map[string]any{
		"status":                 finalized.Status,
		"duration_ms":            finalized.DurationMs,
		"request_id":             finalized.RequestID,
		"http_status":            finalized.HTTPStatus,
		"error_code":             finalized.ErrorCode,
		"error_message":          finalized.ErrorMessage,
		"redacted_request_json":  provideradapter.JSONString(finalized.RequestMetadata),
		"redacted_response_json": provideradapter.JSONString(finalized.ResponseMetadata),
	}
	result := e.db.WithContext(finalizeCtx).Model(&database.APICallLog{}).
		Where("tenant_id = ? AND id = ? AND task_id = ?", attempt.record.TenantID, attempt.record.ID, attempt.record.TaskID).
		Updates(updates)
	if result.Error != nil {
		return APICallResult{}, result.Error
	}
	if result.RowsAffected == 0 && !apiCallLedgerExists(finalizeCtx, e.db, attempt.record.TenantID, attempt.record.ID, attempt.record.TaskID) {
		return APICallResult{}, ErrNotFound
	}
	return finalized, nil
}

func providerAttemptRequestMetadata(execution ExecutionContext, request provideradapter.ImageRequest, startedAt time.Time) map[string]any {
	return map[string]any{
		"tenantId":            execution.Task.TenantID,
		"userId":              execution.Task.CreatedBy,
		"projectId":           execution.Task.ProjectID,
		"taskId":              execution.Task.ID,
		"providerId":          execution.Provider.ID,
		"providerType":        execution.Provider.Type,
		"modelId":             execution.Model.ID,
		"modelName":           execution.Model.ModelName,
		"attempt":             execution.Task.Attempt,
		"operation":           request.Operation,
		"imageType":           execution.Task.ImageType,
		"outputCount":         providerAttemptOutputCount(request.Parameters),
		"referenceImageCount": len(request.InputImages),
		"promptCharacters":    len(execution.Task.Prompt),
		"startedAt":           startedAt.Format(time.RFC3339Nano),
		"ledgerStatus":        provideradapter.APICallStatusAttempting,
		"parameters":          request.Parameters,
	}
}

func finalProviderAttemptStatus(call APICallResult, ctxErr error) string {
	switch {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return provideradapter.APICallStatusTimeout
	case errors.Is(ctxErr, context.Canceled):
		return provideradapter.APICallStatusCancelled
	}
	status := strings.ToUpper(strings.TrimSpace(call.Status))
	switch status {
	case provideradapter.APICallStatusSuccess:
		return provideradapter.APICallStatusSuccess
	case provideradapter.APICallStatusTimeout:
		return provideradapter.APICallStatusTimeout
	case provideradapter.APICallStatusCancelled:
		return provideradapter.APICallStatusCancelled
	default:
		return provideradapter.APICallStatusFailure
	}
}

func mergeProviderAttemptMetadata(base map[string]any, extra map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func providerAttemptOutputCount(parameters map[string]any) int {
	if parameters == nil {
		return 1
	}
	for _, key := range []string{"outputCount", "n"} {
		value, ok := parameters[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			if typed > 0 {
				return typed
			}
		case int64:
			if typed > 0 {
				return int(typed)
			}
		case float64:
			if typed > 0 {
				return int(typed)
			}
		}
	}
	return 1
}

func apiCallLedgerExists(ctx context.Context, db *gorm.DB, tenantID string, id string, taskID string) bool {
	if db == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(id) == "" || strings.TrimSpace(taskID) == "" {
		return false
	}
	var count int64
	if err := db.WithContext(ctx).Model(&database.APICallLog{}).
		Where("tenant_id = ? AND id = ? AND task_id = ?", tenantID, id, taskID).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}
