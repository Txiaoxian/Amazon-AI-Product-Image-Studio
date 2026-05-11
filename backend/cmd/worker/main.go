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

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/logger"
)

const defaultWorkerHealthcheckFile = "/tmp/worker-ready"

func main() {
	bootstrapLog := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load()
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

	log.Info("worker starting", slog.String("name", cfg.Worker.Name))
	log.Info("worker healthy", slog.String("name", cfg.Worker.Name), slog.String("healthcheck_file", healthcheckFile))

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Worker.ShutdownTimeout)
	defer cancel()

	if err := stopWorker(shutdownCtx); err != nil {
		log.Error("worker shutdown failed", slog.String("error", err.Error()))
		cleanupReadyFile()
		os.Exit(1)
	}

	log.Info("worker stopped", slog.String("name", cfg.Worker.Name))
}

func stopWorker(context.Context) error {
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
