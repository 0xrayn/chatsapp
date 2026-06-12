package middleware

import (
	"time"

	"chatapp/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDKey = "X-Request-ID"

// RequestID attaches a unique request ID to every request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDKey)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header(RequestIDKey, requestID)
		c.Next()
	}
}

// RequestLogger logs every HTTP request with zerolog
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		requestID, _ := c.Get("request_id")

		event := logger.Log.Info()
		if status >= 500 {
			event = logger.Log.Error()
		} else if status >= 400 {
			event = logger.Log.Warn()
		}

		if query != "" {
			path = path + "?" + query
		}

		event.
			Str("request_id", requestID.(string)).
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", status).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Str("user_agent", c.Request.UserAgent()).
			Msg("HTTP Request")
	}
}

// Recovery middleware with zerolog
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID, _ := c.Get("request_id")
				logger.Log.Error().
					Interface("error", err).
					Str("request_id", requestID.(string)).
					Str("path", c.Request.URL.Path).
					Msg("Panic recovered")

				c.AbortWithStatusJSON(500, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}
