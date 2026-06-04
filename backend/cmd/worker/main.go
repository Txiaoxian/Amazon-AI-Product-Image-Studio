package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/logger"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"gorm.io/gorm"
)

const defaultWorkerHealthcheckFile = "/tmp/worker-ready"

func main() {
	bootstrapLog := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := loadStartupConfig()
	if err != nil {
		bootstrapLog.Error("configuration error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log, err := logger.New(cfg.LogLevel, os.Stdout)
	if err != nil {
		bootstrapLog.Error("logger configuration error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := openWorkerDatabase(cfg, log)
	if err != nil {
		log.Error("worker database startup failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(db); err != nil {
			log.Warn("worker database close failed", slog.String("error", err.Error()))
		}
	}()

	objectStore, err := storage.NewMinIOStore(cfg.Storage)
	if err != nil {
		log.Error("worker storage startup failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	executor, err := task.NewProviderRuntimeExecutor(db, log, task.ProviderRuntimeExecutorOptions{
		Provider: cfg.Provider,
		Storage:  cfg.Storage,
		Upload:   cfg.Upload,
		Store:    objectStore,
	})
	if err != nil {
		log.Error("worker provider runtime startup failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	taskQueue := queue.NewRedisReliableTaskQueue(cfg.Queue)
	processor := task.NewWorkerProcessor(db, log, task.WorkerProcessorOptions{
		Limiter:             queue.NewRedisConcurrencyLimiter(cfg.Queue),
		EventPublisher:      queue.NewRedisTaskEventPublisher(cfg.Queue),
		Executor:            executor,
		Store:               objectStore,
		StorageConfig:       cfg.Storage,
		UploadConfig:        cfg.Upload,
		ConcurrencyLeaseTTL: cfg.Queue.ConcurrencyLeaseTTL,
		GlobalConcurrency:   cfg.Queue.GlobalConcurrency,
		TenantConcurrency:   cfg.Queue.TenantConcurrency,
		UserConcurrency:     cfg.Queue.UserConcurrency,
		ProviderConcurrency: cfg.Queue.ProviderConcurrency,
		ModelConcurrency:    cfg.Queue.ModelConcurrency,
		RetryBackoff:        cfg.Queue.RetryBackoff,
		RecoveryBatch:       100,
	})
	worker := task.NewWorker(taskQueue, processor, log, task.WorkerOptions{
		RetryBackoff:     cfg.Queue.RetryBackoff,
		RecoveryInterval: cfg.Queue.RecoveryInterval,
		RecoveryBatch:    100,
		Concurrency:      cfg.Worker.Concurrency,
	})

	healthcheckFile := workerHealthcheckFile()
	if err := markWorkerReady(healthcheckFile); err != nil {
		log.Error("worker readiness failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cleanupReadyFile := func() {
		if err := removeWorkerReady(healthcheckFile); err != nil {
			log.Warn("worker readiness cleanup failed", slog.String("error", err.Error()))
		}
	}
	defer cleanupReadyFile()

	log.Info("worker starting", slog.String("name", cfg.Worker.Name), slog.Int("concurrency", cfg.Worker.Concurrency))
	log.Info("worker healthy", slog.String("name", cfg.Worker.Name), slog.String("healthcheck_file", healthcheckFile))

	cleanupService := asset.NewCleanupService(db, log, cfg.Storage, objectStore)
	retentionLoop := startRetentionMaintenanceLoop(ctx, newRetentionMaintenanceRunner(settings.NewRepository(db), cleanupService, log, retentionMaintenanceOptions{
		Interval:   cfg.Worker.RetentionMaintenanceInterval,
		BatchLimit: cfg.Worker.RetentionMaintenanceBatchLimit,
		LogCleaner: newDatabaseLogRetentionCleaner(db, log),
	}))
	runErr := worker.Run(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Worker.ShutdownTimeout)
	defer cancel()

	if err := retentionLoop.Stop(shutdownCtx); err != nil {
		log.Error("worker retention maintenance shutdown failed", slog.String("error", err.Error()))
		cleanupReadyFile()
		os.Exit(1)
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		log.Error("worker stopped with error", slog.String("error", runErr.Error()))
		cleanupReadyFile()
		os.Exit(1)
	}
	if err := stopWorker(shutdownCtx); err != nil {
		log.Error("worker shutdown failed", slog.String("error", err.Error()))
		cleanupReadyFile()
		os.Exit(1)
	}

	log.Info("worker stopped", slog.String("name", cfg.Worker.Name))
}

func loadStartupConfig() (config.Config, error) {
	return config.Load()
}

func stopWorker(context.Context) error {
	return nil
}

func openWorkerDatabase(cfg config.Config, log *slog.Logger) (*gorm.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Database.ConnectTimeout)
	defer cancel()

	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	if err := runWorkerDatabaseStartupTasks(ctx, db, cfg.Database, log, database.RunMigrations); err != nil {
		_ = database.Close(db)
		return nil, err
	}

	return db, nil
}

func runWorkerDatabaseStartupTasks(
	ctx context.Context,
	db *gorm.DB,
	cfg config.DatabaseConfig,
	log *slog.Logger,
	runMigrations func(context.Context, *gorm.DB) error,
) error {
	if cfg.MigrationsMode == "startup-gate" {
		if err := runMigrations(ctx, db); err != nil {
			return err
		}
		log.Info("worker database migrations complete")
	} else {
		log.Info("worker database migrations skipped", slog.String("mode", cfg.MigrationsMode))
	}

	return nil
}

func workerHealthcheckFile() string {
	path := strings.TrimSpace(os.Getenv("WORKER_HEALTHCHECK_FILE"))
	if path == "" {
		return defaultWorkerHealthcheckFile
	}
	return path
}

func markWorkerReady(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("worker healthcheck file path is empty")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create worker healthcheck directory: %w", err)
	}

	if err := os.WriteFile(path, []byte("ready\n"), 0o600); err != nil {
		return fmt.Errorf("write worker healthcheck file: %w", err)
	}

	return nil
}

func removeWorkerReady(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove worker healthcheck file: %w", err)
	}

	return nil
}
