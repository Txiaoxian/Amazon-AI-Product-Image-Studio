package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	RequestIDHeader = "X-Request-ID"
	csrfHeaderName  = "X-CSRF-Token"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := cleanRequestID(c.GetHeader(RequestIDHeader))
		if requestID == "" {
			requestID = newRequestID()
		}

		c.Set("request_id", requestID)
		c.Writer.Header().Set(RequestIDHeader, requestID)
		c.Next()
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		headers.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		_, isAllowed := allowed[origin]
		if origin != "" && isAllowed {
			headers := c.Writer.Header()
			headers.Set("Access-Control-Allow-Origin", origin)
			headers.Set("Access-Control-Allow-Credentials", "true")
			headers.Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, "+csrfHeaderName)
			headers.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			appendVary(headers, "Origin")
		}

		if c.Request.Method == http.MethodOptions && origin != "" {
			if isAllowed {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}

			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

func Recovery(log *slog.Logger) gin.HandlerFunc {
	log = fallbackLogger(log)

	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recovered", slog.String("request_id", RequestIDFromContext(c)))
				if c.Writer.Written() {
					c.Abort()
					return
				}

				AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
			}
		}()

		c.Next()
	}
}

func AccessLog(log *slog.Logger) gin.HandlerFunc {
	log = fallbackLogger(log)

	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		log.Info("http request",
			slog.String("request_id", RequestIDFromContext(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.FullPath()),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)
	}
}

func RequestIDFromContext(c *gin.Context) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}

	requestID, ok := value.(string)
	if !ok {
		return ""
	}

	return requestID
}

func cleanRequestID(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return ""
	}

	for _, r := range requestID {
		if r < 33 || r > 126 {
			return ""
		}
	}

	return requestID
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}

	return "req_" + hex.EncodeToString(bytes[:])
}

func appendVary(headers http.Header, value string) {
	existing := headers.Get("Vary")
	if existing == "" {
		headers.Set("Vary", value)
		return
	}

	for _, part := range strings.Split(existing, ",") {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return
		}
	}

	headers.Set("Vary", existing+", "+value)
}

func fallbackLogger(log *slog.Logger) *slog.Logger {
	if log != nil {
		return log
	}

	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
