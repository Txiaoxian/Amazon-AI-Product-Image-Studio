package httpx

import "github.com/gin-gonic/gin"

type SuccessResponse struct {
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
}

type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"requestId"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, SuccessResponse{
		Data:      data,
		RequestID: RequestIDFromContext(c),
	})
}

func AbortWithError(c *gin.Context, status int, code string, message string, details any) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: errorDetails(details),
		},
		RequestID: RequestIDFromContext(c),
	})
}

func errorDetails(details any) any {
	if details == nil {
		return gin.H{}
	}

	return details
}
