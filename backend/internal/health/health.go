package health

import (
	"net/http"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/httpx"
	"github.com/gin-gonic/gin"
)

type Status struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func Handler(service string) gin.HandlerFunc {
	if service == "" {
		service = "backend"
	}

	return func(c *gin.Context) {
		httpx.JSON(c, http.StatusOK, Status{
			Status:  "ok",
			Service: service,
		})
	}
}
