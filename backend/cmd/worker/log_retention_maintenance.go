package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/audit"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"gorm.io/gorm"
)

const (
	actionLogRetentionCleanup = "log_retention.cleanup"
)

var (
	errLogRetentionCleanupFailed = errors.New("log retention cleanup failed")
	terminalTaskStatuses         = []string{task.StatusSucceeded, task.StatusFailed, task.StatusCancelled, task.StatusTimedOut}
)

type logRetentionCleaner interface {
	CleanupLogRetention(ctx context.Context, config settings.EnabledLogRetention, now time.Time, batchLimit int) (logRetentionCleanupSummary, error)
}

type databaseLogRetentionCleaner struct {
	db *gorm.DB
}

type logRetentionCleanupSummary struct {
	OperationLogs logRetentionCategorySummary
	APICallLogs   logRetentionCategorySummary
	TaskEvents    logRetentionCategorySummary
}

type logRetentionCategorySummary struct {
	Processed int
	Deleted   int
	Failed    int
}

func newDatabaseLogRetentionCleaner(db *gorm.DB, _ *slog.Logger) *databaseLogRetentionCleaner {
	return &databaseLogRetentionCleaner{db: db}
}

func (c *databaseLogRetentionCleaner) CleanupLogRetention(ctx context.Context, config settings.EnabledLogRetention, now time.Time, batchLimit int) (logRetentionCleanupSummary, error) {
	var summary logRetentionCleanupSummary
	if c == nil || c.db == nil {
		return summary, database.ErrNilDB
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return summary, err
	}
	if batchLimit <= 0 {
		batchLimit = 100
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if config.OperationLogRetentionDays != nil {
			cutoff := now.Add(-time.Duration(*config.OperationLogRetentionDays) * 24 * time.Hour)
			summary.OperationLogs, err = cleanupOperationLogs(ctx, tx, config.TenantID, cutoff, batchLimit)
			if err != nil {
				summary.OperationLogs.Failed = summary.OperationLogs.Processed
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if config.APICallLogRetentionDays != nil {
			cutoff := now.Add(-time.Duration(*config.APICallLogRetentionDays) * 24 * time.Hour)
			summary.APICallLogs, err = cleanupAPICallLogs(ctx, tx, config.TenantID, cutoff, batchLimit)
			if err != nil {
				summary.APICallLogs.Failed = summary.APICallLogs.Processed
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if config.TaskEventRetentionDays != nil {
			cutoff := now.Add(-time.Duration(*config.TaskEventRetentionDays) * 24 * time.Hour)
			summary.TaskEvents, err = cleanupTaskEvents(ctx, tx, config.TenantID, cutoff, batchLimit)
			if err != nil {
				summary.TaskEvents.Failed = summary.TaskEvents.Processed
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return audit.NewRecorder(tx).Record(ctx, audit.Event{
			TenantID:     config.TenantID,
			Action:       actionLogRetentionCleanup,
			ResourceType: "system_settings",
			ResourceID:   settings.KeyLogRetention,
			Metadata: map[string]any{
				"key": settings.KeyLogRetention,
				"operationLogs": map[string]any{
					"processed": summary.OperationLogs.Processed,
					"deleted":   summary.OperationLogs.Deleted,
					"failed":    summary.OperationLogs.Failed,
				},
				"apiCallLogs": map[string]any{
					"processed": summary.APICallLogs.Processed,
					"deleted":   summary.APICallLogs.Deleted,
					"failed":    summary.APICallLogs.Failed,
				},
				"taskEvents": map[string]any{
					"processed": summary.TaskEvents.Processed,
					"deleted":   summary.TaskEvents.Deleted,
					"failed":    summary.TaskEvents.Failed,
				},
			},
		})
	})
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return summary, ctx.Err()
		}
		return summary, errLogRetentionCleanupFailed
	}
	return summary, ctx.Err()
}

func cleanupOperationLogs(ctx context.Context, db *gorm.DB, tenantID string, cutoff time.Time, batchLimit int) (logRetentionCategorySummary, error) {
	var ids []string
	if err := db.WithContext(ctx).
		Model(&database.OperationLog{}).
		Where("tenant_id = ? AND created_at < ?", tenantID, cutoff.UTC()).
		Order("created_at ASC, id ASC").
		Limit(batchLimit).
		Pluck("id", &ids).Error; err != nil {
		return logRetentionCategorySummary{}, err
	}
	if len(ids) == 0 {
		return logRetentionCategorySummary{}, nil
	}
	result := db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Delete(&database.OperationLog{})
	return logRetentionCategorySummary{Processed: len(ids), Deleted: int(result.RowsAffected)}, result.Error
}

func cleanupAPICallLogs(ctx context.Context, db *gorm.DB, tenantID string, cutoff time.Time, batchLimit int) (logRetentionCategorySummary, error) {
	var ids []string
	if err := db.WithContext(ctx).
		Model(&database.APICallLog{}).
		Where("tenant_id = ? AND created_at < ?", tenantID, cutoff.UTC()).
		Order("created_at ASC, id ASC").
		Limit(batchLimit).
		Pluck("id", &ids).Error; err != nil {
		return logRetentionCategorySummary{}, err
	}
	if len(ids) == 0 {
		return logRetentionCategorySummary{}, nil
	}
	result := db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Delete(&database.APICallLog{})
	return logRetentionCategorySummary{Processed: len(ids), Deleted: int(result.RowsAffected)}, result.Error
}

func cleanupTaskEvents(ctx context.Context, db *gorm.DB, tenantID string, cutoff time.Time, batchLimit int) (logRetentionCategorySummary, error) {
	var sequences []uint64
	if err := db.WithContext(ctx).
		Model(&database.TaskEvent{}).
		Joins("JOIN generation_tasks ON generation_tasks.tenant_id = task_events.tenant_id AND generation_tasks.id = task_events.task_id").
		Where("task_events.tenant_id = ? AND task_events.created_at < ? AND generation_tasks.status IN ?", tenantID, cutoff.UTC(), terminalTaskStatuses).
		Order("task_events.created_at ASC, task_events.sequence ASC").
		Limit(batchLimit).
		Pluck("task_events.sequence", &sequences).Error; err != nil {
		return logRetentionCategorySummary{}, err
	}
	if len(sequences) == 0 {
		return logRetentionCategorySummary{}, nil
	}
	result := db.WithContext(ctx).
		Where("tenant_id = ? AND sequence IN ?", tenantID, sequences).
		Delete(&database.TaskEvent{})
	return logRetentionCategorySummary{Processed: len(sequences), Deleted: int(result.RowsAffected)}, result.Error
}
