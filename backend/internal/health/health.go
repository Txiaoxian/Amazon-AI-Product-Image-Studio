package health

import (
	"context"
	"net/http"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/gin-gonic/gin"
)

const dependencyTimeout = 2 * time.Second

type Status struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

type DependencyChecker interface {
	Name() string
	Check(context.Context) error
}

func Handler(service string, checkers ...DependencyChecker) gin.HandlerFunc {
	if service == "" {
		service = "backend"
	}

	return func(c *gin.Context) {
		status := Status{
			Status:  "ok",
			Service: service,
		}

		httpStatus := http.StatusOK
		if len(checkers) > 0 {
			status.Dependencies = make(map[string]string, len(checkers))
		}

		for _, checker := range checkers {
			if checker == nil {
				continue
			}

			name := checker.Name()
			if name == "" {
				name = "dependency"
			}

			checkCtx, cancel := context.WithTimeout(c.Request.Context(), dependencyTimeout)
			err := checker.Check(checkCtx)
			cancel()

			if err != nil {
				status.Status = "degraded"
				status.Dependencies[name] = "unhealthy"
				httpStatus = http.StatusServiceUnavailable
				continue
			}

			status.Dependencies[name] = "ok"
		}

		httpx.JSON(c, httpStatus, status)
	}
}
