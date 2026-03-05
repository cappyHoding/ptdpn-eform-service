package middleware

import (
	"time"

	"github.com/cappyHoding/ptdpn-eform-service/pkg/logger"
	"github.com/cappyHoding/ptdpn-eform-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger logs every incoming HTTP request and its outcome.
// This is different from audit logging — this captures all traffic,
// while audit logs capture business events.
//
// Logged fields:
//   - method, path, status, latency, client IP, request ID, user agent
func RequestLogger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process the request — this runs the actual handler
		c.Next()

		// After handler completes, log the result
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		requestID := c.GetString("X-Request-ID")

		// Choose log level based on status code
		// 5xx errors get Error level (will trigger alerts in production)
		// 4xx errors get Warn level
		// 2xx/3xx get Info level
		logFn := log.Info
		if statusCode >= 500 {
			logFn = log.Error
		} else if statusCode >= 400 {
			logFn = log.Warn
		}

		// Build the full path including query string for debugging
		fullPath := path
		if query != "" {
			fullPath = path + "?" + query
		}

		logFn("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", fullPath),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.String("request_id", requestID),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}

// Recovery catches panics in route handlers and returns a proper 500 response
// instead of crashing the server.
//
// WHY IS THIS NEEDED?
// In Go, an unrecovered panic in a goroutine kills the entire program.
// In a web server context, we want to catch panics in individual request
// handlers, log them, and keep the server running for other requests.
func Recovery(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace for debugging
				log.Error("PANIC RECOVERED",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.String("request_id", c.GetString("X-Request-ID")),
				)

				// Return a clean 500 response — never expose the panic details to clients
				response.InternalError(c, "")
				c.Abort()
			}
		}()

		c.Next()
	}
}
