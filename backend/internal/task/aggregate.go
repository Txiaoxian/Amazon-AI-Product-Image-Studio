package task

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/tenant"
	"gorm.io/gorm"
)

const (
	maxDiagnosticsErrorMessageRunes = 200
	maxDiagnosticsLimit             = 50
	defaultDiagnosticsLimit         = 10
)

// StatusAggregate holds tenant-scoped task counts grouped by status.
type StatusAggregate struct {
	StatusCounts   map[string]int64  `json:"statusCounts"`
	TotalTasks     int64             `json:"totalTasks"`
	RecentFailures []RecentFailure   `json:"recentFailures"`
}

// RecentFailure is a sanitized task failure sample for diagnostics.
// It intentionally omits prompt, parameters, Provider payloads, input/output
// asset IDs, object keys, and raw error details.
type RecentFailure struct {
	TaskID       string `json:"taskId"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	FinishedAt   string `json:"finishedAt,omitempty"`
}

// QueryTaskStatusAggregate returns tenant-scoped task status counts and
// bounded recent failure samples. It uses read-only queries and does not
// modify any task state.
func QueryTaskStatusAggregate(ctx context.Context, db *gorm.DB, scope tenant.Scope, limit int) (StatusAggregate, error) {
	if db == nil {
		return StatusAggregate{}, database.ErrNilDB
	}
	if !scope.Valid() {
		return StatusAggregate{}, tenant.ErrMissingTenantID
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || limit > maxDiagnosticsLimit {
		limit = defaultDiagnosticsLimit
	}

	// Query status counts
	type statusRow struct {
		Status string
		Count  int64
	}
	var rows []statusRow
	if err := db.WithContext(ctx).
		Model(&database.GenerationTask{}).
		Select("status, COUNT(*) AS count").
		Where("tenant_id = ?", scope.ID()).
		Group("status").
		Find(&rows).Error; err != nil {
		return StatusAggregate{}, err
	}

	statusCounts := make(map[string]int64)
	var total int64
	for _, row := range rows {
		statusCounts[row.Status] = row.Count
		total += row.Count
	}
	// Ensure all known statuses are present
	for _, s := range []string{StatusQueued, StatusRunning, StatusSucceeded, StatusFailed, StatusCancelled, StatusRetrying, StatusTimedOut} {
		if _, ok := statusCounts[s]; !ok {
			statusCounts[s] = 0
		}
	}

	// Query recent failures
	failureStatuses := []string{StatusFailed, StatusTimedOut}
	var taskRecords []database.GenerationTask
	if err := db.WithContext(ctx).
		Model(&database.GenerationTask{}).
		Select("id, status, error_code, error_message, finished_at").
		Where("tenant_id = ? AND status IN ?", scope.ID(), failureStatuses).
		Order("updated_at DESC, id DESC").
		Limit(limit).
		Find(&taskRecords).Error; err != nil {
		return StatusAggregate{}, err
	}

	recentFailures := make([]RecentFailure, 0, len(taskRecords))
	for _, record := range taskRecords {
		recentFailures = append(recentFailures, RecentFailure{
			TaskID:       record.ID,
			Status:       record.Status,
			ErrorCode:    sanitizeDiagnosticsString(record.ErrorCode, maxDiagnosticsErrorMessageRunes),
			ErrorMessage: sanitizeDiagnosticsString(record.ErrorMessage, maxDiagnosticsErrorMessageRunes),
			FinishedAt:   formatOptionalDiagnosticsTime(record.FinishedAt),
		})
	}

	return StatusAggregate{
		StatusCounts:   statusCounts,
		TotalTasks:     total,
		RecentFailures: recentFailures,
	}, nil
}

// sanitizeDiagnosticsString truncates a string to maxRunes and removes
// control characters for safe diagnostic output.
func sanitizeDiagnosticsString(s string, maxRunes int) string {
	if s == "" {
		return ""
	}
	// Remove control characters
	cleaned := make([]rune, 0, utf8.RuneCountInString(s))
	for _, r := range s {
		if r < 32 || r == 127 {
			continue
		}
		cleaned = append(cleaned, r)
	}
	if maxRunes > 0 && len(cleaned) > maxRunes {
		cleaned = cleaned[:maxRunes]
	}
	return string(cleaned)
}

func formatOptionalDiagnosticsTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
