package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/api"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/health"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := openDatabase(cfg, log)
	if err != nil {
		log.Error("database startup failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(db); err != nil {
			log.Warn("database close failed", slog.String("error", err.Error()))
		}
	}()

	router := newRouter(cfg, log, db, database.NewHealthChecker(db))
	server := &http.Server{
		Addr:         cfg.API.Addr,
		Handler:      router,
		ReadTimeout:  cfg.API.ReadTimeout,
		WriteTimeout: cfg.API.WriteTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.API.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("api shutdown failed", slog.String("error", err.Error()))
		}
	}()

	log.Info("api starting", slog.String("addr", cfg.API.Addr))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("api stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("api stopped")
}

func loadStartupConfig() (config.Config, error) {
	return config.Load()
}

func newRouter(cfg config.Config, log *slog.Logger, db *gorm.DB, healthChecks ...health.DependencyChecker) *gin.Engine {
	return api.NewRouter(api.RouterOptions{
		Config:       cfg,
		Logger:       log,
		HealthChecks: healthChecks,
		Database:     db,
	})
}

func openDatabase(cfg config.Config, log *slog.Logger) (*gorm.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Database.ConnectTimeout)
	defer cancel()

	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	if cfg.Database.MigrationsMode == "startup-gate" {
		if err := database.RunMigrations(ctx, db); err != nil {
			_ = database.Close(db)
			return nil, err
		}
		log.Info("database migrations complete")
	} else {
		log.Info("database migrations skipped", slog.String("mode", cfg.Database.MigrationsMode))
	}

	return db, nil
}
