package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/logger"
)

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

	log.Info("worker starting", slog.String("name", cfg.Worker.Name))
	log.Info("worker healthy", slog.String("name", cfg.Worker.Name))

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Worker.ShutdownTimeout)
	defer cancel()

	if err := stopWorker(shutdownCtx); err != nil {
		log.Error("worker shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("worker stopped", slog.String("name", cfg.Worker.Name))
}

func stopWorker(context.Context) error {
	return nil
}
