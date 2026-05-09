package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/health"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/logger"
	"github.com/gin-gonic/gin"
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

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := newRouter(log)
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

func newRouter(log *slog.Logger) *gin.Engine {
	router := gin.New()
	router.Use(httpx.RequestID())
	router.Use(httpx.SecurityHeaders())
	router.Use(httpx.Recovery(log))
	router.Use(httpx.AccessLog(log))

	healthHandler := health.Handler("api")
	router.GET("/healthz", healthHandler)
	router.Group("/api/v1").GET("/healthz", healthHandler)

	return router
}
