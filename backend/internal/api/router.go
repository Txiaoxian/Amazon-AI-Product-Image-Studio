package api

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/health"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/queue"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/redaction"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/settings"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/sse"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/task"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/useradmin"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RouterOptions struct {
	Config              config.Config
	Logger              *slog.Logger
	HealthChecks        []health.DependencyChecker
	Database            *gorm.DB
	ObjectStore         storage.ObjectStore
	ProviderOpts        []provider.Option
	AuditReadRedactor   *redaction.Redactor
	TaskEnqueuer        queue.TaskEnqueuer
	SSEBroker           *sse.Broker
	SSEHeartbeat        time.Duration
	LifecycleContext    context.Context
	TaskEventSubscriber queue.TaskEventSubscriber
}

func NewRouter(options RouterOptions) *gin.Engine {
	router := gin.New()
	router.Use(httpx.RequestID())
	router.Use(httpx.SecurityHeaders())
	router.Use(httpx.CORS(options.Config.API.CORSAllowedOrigins))
	router.Use(httpx.Recovery(options.Logger))
	router.Use(httpx.AccessLog(options.Logger))

	objectStore := options.ObjectStore
	if objectStore == nil {
		var err error
		objectStore, err = storage.NewMinIOStore(options.Config.Storage)
		if err != nil && options.Logger != nil {
			options.Logger.Error("asset storage client setup failed", slog.String("error", err.Error()))
		}
	}

	eventBroker := options.SSEBroker
	if eventBroker == nil {
		eventBroker = sse.NewBroker(0)
	}
	eventPublisher := task.EventPublisher(eventBroker)
	if shouldStartRedisTaskEventBridge(options.Config) {
		redisEventPublisher := queue.NewRedisTaskEventPublisher(options.Config.Queue)
		eventPublisher = task.MultiEventPublisher(eventBroker, redisEventPublisher)
		taskEventSubscriber := options.TaskEventSubscriber
		if taskEventSubscriber == nil {
			taskEventSubscriber = queue.NewRedisTaskEventSubscriber(options.Config.Queue)
		}
		startTaskEventSubscriber(options.LifecycleContext, taskEventSubscriber, eventBroker, options.Logger)
	}
	taskService := task.NewService(options.Database, options.Logger, taskEnqueuer(options), task.WithEventPublisher(eventPublisher))
	settingsService := settings.NewService(options.Database, options.Logger, options.Config.Upload)

	RegisterRoutes(
		router,
		auth.NewService(options.Database, options.Config, options.Logger),
		project.NewService(options.Database, options.Logger),
		asset.NewService(options.Database, options.Logger, options.Config.Storage, options.Config.Upload, objectStore, settingsService),
		provider.NewService(options.Database, options.Logger, options.Config.Provider, options.ProviderOpts...),
		model.NewService(options.Database, options.Logger),
		taskService,
		sse.NewService(options.Database, options.Logger, eventBroker, sse.Options{HeartbeatInterval: options.SSEHeartbeat}),
		newAdminAuditUsageService(options.Database, options.Logger, options.AuditReadRedactor),
		settingsService,
		useradmin.NewService(options.Database, options.Logger),
		options.HealthChecks...,
	)

	return router
}

func RegisterRoutes(router *gin.Engine, authService *auth.Service, projectService *project.Service, assetService *asset.Service, providerService *provider.Service, modelService *model.Service, taskService *task.Service, sseService *sse.Service, adminAuditUsageService *adminAuditUsageService, settingsService *settings.Service, userAdminService *useradmin.Service, healthChecks ...health.DependencyChecker) {
	healthHandler := health.Handler("api", healthChecks...)
	router.GET("/healthz", healthHandler)

	v1 := router.Group("/api/v1")
	registerV1Routes(v1, healthHandler, authService, projectService, assetService, providerService, modelService, taskService, sseService, adminAuditUsageService, settingsService, userAdminService)
}

func registerV1Routes(v1 *gin.RouterGroup, healthHandler gin.HandlerFunc, authService *auth.Service, projectService *project.Service, assetService *asset.Service, providerService *provider.Service, modelService *model.Service, taskService *task.Service, sseService *sse.Service, adminAuditUsageService *adminAuditUsageService, settingsService *settings.Service, userAdminService *useradmin.Service) {
	v1.GET("/healthz", healthHandler)

	v1.POST("/auth/init-admin", authService.InitAdmin)
	v1.POST("/auth/login", authService.Login)

	protected := v1.Group("")
	protected.Use(authService.RequireAuth())
	protected.Use(authService.RequireCSRF())
	protected.POST("/auth/logout", authService.Logout)
	protected.GET("/me", authService.Me)
	protected.PATCH("/me/password", authService.ChangePassword)
	if projectService != nil {
		projectService.RegisterRoutes(protected)
	}
	if assetService != nil {
		assetService.RegisterRoutes(protected)
	}
	if providerService != nil {
		providerService.RegisterRoutes(protected)
	}
	if modelService != nil {
		modelService.RegisterRoutes(protected)
	}
	if taskService != nil {
		taskService.RegisterRoutes(protected)
	}
	if sseService != nil {
		sseService.RegisterRoutes(protected)
	}
	if adminAuditUsageService != nil {
		adminAuditUsageService.RegisterRoutes(protected)
	}
	if settingsService != nil {
		settingsService.RegisterRoutes(protected)
	}
	if userAdminService != nil {
		userAdminService.RegisterRoutes(protected)
	}
}

func taskEnqueuer(options RouterOptions) queue.TaskEnqueuer {
	if options.TaskEnqueuer != nil {
		return options.TaskEnqueuer
	}
	return queue.NewRedisTaskEnqueuer(options.Config.Queue)
}

func shouldStartRedisTaskEventBridge(cfg config.Config) bool {
	return !strings.EqualFold(strings.TrimSpace(cfg.AppEnv), "test")
}

func startTaskEventSubscriber(ctx context.Context, subscriber queue.TaskEventSubscriber, sink queue.TaskEventSink, log *slog.Logger) {
	if ctx == nil {
		if log != nil {
			log.Warn("task event wakeup subscriber not started", slog.String("reason", "missing lifecycle context"))
		}
		return
	}
	queue.StartTaskEventSubscriber(ctx, subscriber, sink, log)
}
