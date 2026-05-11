package api

import (
	"log/slog"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/health"
	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/gin-gonic/gin"
)

type RouterOptions struct {
	Config       config.Config
	Logger       *slog.Logger
	HealthChecks []health.DependencyChecker
}

func NewRouter(options RouterOptions) *gin.Engine {
	router := gin.New()
	router.Use(httpx.RequestID())
	router.Use(httpx.SecurityHeaders())
	router.Use(httpx.CORS(options.Config.API.CORSAllowedOrigins))
	router.Use(httpx.Recovery(options.Logger))
	router.Use(httpx.AccessLog(options.Logger))

	RegisterRoutes(router, options.HealthChecks...)

	return router
}

func RegisterRoutes(router *gin.Engine, healthChecks ...health.DependencyChecker) {
	healthHandler := health.Handler("api", healthChecks...)
	router.GET("/healthz", healthHandler)

	v1 := router.Group("/api/v1")
	registerV1Routes(v1, healthHandler)
}

func registerV1Routes(v1 *gin.RouterGroup, healthHandler gin.HandlerFunc) {
	v1.GET("/healthz", healthHandler)
}
