package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
)

type retentionCleaner interface {
	PurgeDeletedAssets(ctx context.Context, tenantID string, cutoff time.Time, options asset.PurgeOptions) (asset.PurgeResult, error)
}

type retentionMaintenanceOptions struct {
	Interval   time.Duration
	BatchLimit int
	Now        func() time.Time
	LogCleaner logRetentionCleaner
}

type retentionMaintenanceRunner struct {
	repo       settings.Repository
	cleaner    retentionCleaner
	logCleaner logRetentionCleaner
	log        *slog.Logger
	interval   time.Duration
	batchLimit int
	now        func() time.Time
}

type retentionMaintenanceLoop struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func newRetentionMaintenanceRunner(repo settings.Repository, cleaner retentionCleaner, log *slog.Logger, options retentionMaintenanceOptions) *retentionMaintenanceRunner {
	if log == nil {
		log = slog.Default()
	}
	if options.Interval <= 0 {
		options.Interval = time.Hour
	}
	if options.BatchLimit <= 0 {
		options.BatchLimit = 100
	}
	if options.Now == nil {
		options.Now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &retentionMaintenanceRunner{
		repo:       repo,
		cleaner:    cleaner,
		logCleaner: options.LogCleaner,
		log:        log,
		interval:   options.Interval,
		batchLimit: options.BatchLimit,
		now:        options.Now,
	}
}

func startRetentionMaintenanceLoop(ctx context.Context, runner *retentionMaintenanceRunner) *retentionMaintenanceLoop {
	if ctx == nil {
		ctx = context.Background()
	}
	loopCtx, cancel := context.WithCancel(ctx)
	loop := &retentionMaintenanceLoop{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go func() {
		defer close(loop.done)
		if runner != nil {
			runner.run(loopCtx)
		}
	}()
	return loop
}

func (l *retentionMaintenanceLoop) Stop(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	l.once.Do(l.cancel)
	select {
	case <-l.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *retentionMaintenanceRunner) run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.runOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		r.log.Warn("storage retention maintenance run failed", slog.String("error_kind", retentionMaintenanceErrorKind(err)))
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.runOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				r.log.Warn("storage retention maintenance run failed", slog.String("error_kind", retentionMaintenanceErrorKind(err)))
			}
		}
	}
}

func (r *retentionMaintenanceRunner) runOnce(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil {
		return nil
	}

	if r.cleaner != nil {
		configs, invalid, err := settings.LoadEnabledStorageRetentions(ctx, r.repo)
		if err != nil {
			return err
		}
		for _, damaged := range invalid {
			r.log.Warn("storage retention setting invalid",
				slog.String("tenant_id", damaged.TenantID),
				slog.String("error_kind", retentionMaintenanceErrorKind(damaged.Err)),
			)
		}

		for _, config := range configs {
			if err := ctx.Err(); err != nil {
				return err
			}
			cutoff := r.now().UTC().Add(-time.Duration(config.DeletedAssetRetentionDays) * 24 * time.Hour)
			result, err := r.cleaner.PurgeDeletedAssets(ctx, config.TenantID, cutoff, asset.PurgeOptions{BatchLimit: r.batchLimit})
			if err != nil {
				if errors.Is(err, context.Canceled) && ctx.Err() != nil {
					return ctx.Err()
				}
				r.log.Warn("storage retention cleanup failed",
					slog.String("tenant_id", config.TenantID),
					slog.Int("processed", result.Processed),
					slog.Int("purged", result.Purged),
					slog.Int("failed", result.Failed),
					slog.String("error_kind", retentionMaintenanceErrorKind(err)),
				)
				continue
			}
		}
	}

	if r.logCleaner != nil {
		configs, invalid, err := settings.LoadEnabledLogRetentions(ctx, r.repo)
		if err != nil {
			return err
		}
		for _, damaged := range invalid {
			r.log.Warn("log retention setting invalid",
				slog.String("tenant_id", damaged.TenantID),
				slog.String("error_kind", retentionMaintenanceErrorKind(damaged.Err)),
			)
		}

		for _, config := range configs {
			if err := ctx.Err(); err != nil {
				return err
			}
			result, err := r.logCleaner.CleanupLogRetention(ctx, config, r.now().UTC(), r.batchLimit)
			if err != nil {
				if errors.Is(err, context.Canceled) && ctx.Err() != nil {
					return ctx.Err()
				}
				r.log.Warn("log retention cleanup failed",
					slog.String("tenant_id", config.TenantID),
					slog.Int("operation_processed", result.OperationLogs.Processed),
					slog.Int("operation_deleted", result.OperationLogs.Deleted),
					slog.Int("operation_failed", result.OperationLogs.Failed),
					slog.Int("api_call_processed", result.APICallLogs.Processed),
					slog.Int("api_call_deleted", result.APICallLogs.Deleted),
					slog.Int("api_call_failed", result.APICallLogs.Failed),
					slog.Int("task_event_processed", result.TaskEvents.Processed),
					slog.Int("task_event_deleted", result.TaskEvents.Deleted),
					slog.Int("task_event_failed", result.TaskEvents.Failed),
					slog.String("error_kind", retentionMaintenanceErrorKind(err)),
				)
				continue
			}
		}
	}
	return ctx.Err()
}

func retentionMaintenanceErrorKind(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline_exceeded"
	case errors.Is(err, settings.ErrStoredStorageRetentionInvalid):
		return "invalid_setting"
	case errors.Is(err, settings.ErrStoredLogRetentionInvalid):
		return "invalid_setting"
	case errors.Is(err, asset.ErrCleanupFailed):
		return "cleanup_failed"
	case errors.Is(err, errLogRetentionCleanupFailed):
		return "cleanup_failed"
	default:
		return "internal_error"
	}
}
