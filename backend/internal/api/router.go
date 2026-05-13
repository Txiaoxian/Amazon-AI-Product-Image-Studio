package api

import (
	"log/slog"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/asset"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/health"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/model"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/project"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RouterOptions struct {
	Config       config.Config
	Logger       *slog.Logger
	HealthChecks []health.DependencyChecker
	Database     *gorm.DB
	ObjectStore  storage.ObjectStore
	ProviderOpts []provider.Option
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

	RegisterRoutes(
		router,
		auth.NewService(options.Database, options.Config, options.Logger),
		project.NewService(options.Database, options.Logger),
		asset.NewService(options.Database, options.Logger, options.Config.Storage, options.Config.Upload, objectStore),
		provider.NewService(options.Database, options.Logger, options.Config.Provider, options.ProviderOpts...),
		model.NewService(options.Database, options.Logger),
		options.HealthChecks...,
	)

	return router
}

func RegisterRoutes(router *gin.Engine, authService *auth.Service, projectService *project.Service, assetService *asset.Service, providerService *provider.Service, modelService *model.Service, healthChecks ...health.DependencyChecker) {
	healthHandler := health.Handler("api", healthChecks...)
	router.GET("/healthz", healthHandler)

	v1 := router.Group("/api/v1")
	registerV1Routes(v1, healthHandler, authService, projectService, assetService, providerService, modelService)
}

func registerV1Routes(v1 *gin.RouterGroup, healthHandler gin.HandlerFunc, authService *auth.Service, projectService *project.Service, assetService *asset.Service, providerService *provider.Service, modelService *model.Service) {
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
}
