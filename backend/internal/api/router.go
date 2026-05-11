package api

import (
	"log/slog"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/auth"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/health"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RouterOptions struct {
	Config       config.Config
	Logger       *slog.Logger
	HealthChecks []health.DependencyChecker
	Database     *gorm.DB
}

func NewRouter(options RouterOptions) *gin.Engine {
	router := gin.New()
	router.Use(httpx.RequestID())
	router.Use(httpx.SecurityHeaders())
	router.Use(httpx.CORS(options.Config.API.CORSAllowedOrigins))
	router.Use(httpx.Recovery(options.Logger))
	router.Use(httpx.AccessLog(options.Logger))

	RegisterRoutes(router, auth.NewService(options.Database, options.Config, options.Logger), options.HealthChecks...)

	return router
}

func RegisterRoutes(router *gin.Engine, authService *auth.Service, healthChecks ...health.DependencyChecker) {
	healthHandler := health.Handler("api", healthChecks...)
	router.GET("/healthz", healthHandler)

	v1 := router.Group("/api/v1")
	registerV1Routes(v1, healthHandler, authService)
}

func registerV1Routes(v1 *gin.RouterGroup, healthHandler gin.HandlerFunc, authService *auth.Service) {
	v1.GET("/healthz", healthHandler)

	v1.POST("/auth/init-admin", authService.InitAdmin)
	v1.POST("/auth/login", authService.Login)

	protected := v1.Group("")
	protected.Use(authService.RequireAuth())
	protected.Use(authService.RequireCSRF())
	protected.POST("/auth/logout", authService.Logout)
	protected.GET("/me", authService.Me)
	protected.PATCH("/me/password", authService.ChangePassword)
}
