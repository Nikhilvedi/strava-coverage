package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nikhilvedi/strava-coverage/internal/utils"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// LoggingMiddleware provides structured request/response logging
func LoggingMiddleware() gin.HandlerFunc {
	logger := utils.NewLogger("HTTP")

	return func(c *gin.Context) {
		startTime := time.Now()

		// Log request
		logger.Info("Request started - Method: %s, Path: %s, IP: %s",
			c.Request.Method, c.Request.URL.Path, c.ClientIP())

		c.Next()

		// Log response
		duration := time.Since(startTime)
		status := c.Writer.Status()

		logLevel := "Info"
		if status >= 400 {
			logLevel = "Error"
		} else if status >= 300 {
			logLevel = "Warn"
		}

		message := "Request completed - Method: %s, Path: %s, Status: %d, Duration: %v"
		switch logLevel {
		case "Error":
			logger.Error(message, c.Request.Method, c.Request.URL.Path, status, duration)
		case "Warn":
			logger.Warn(message, c.Request.Method, c.Request.URL.Path, status, duration)
		default:
			logger.Info(message, c.Request.Method, c.Request.URL.Path, status, duration)
		}
	}
}

// ErrorHandlingMiddleware provides centralized error handling
func ErrorHandlingMiddleware() gin.HandlerFunc {
	logger := utils.NewLogger("ErrorHandler")

	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered: %v", err)

				apiErr := utils.NewAPIError(500, "Internal server error", "An unexpected error occurred")
				utils.ErrorResponse(c, apiErr)
				c.Abort()
			}
		}()

		c.Next()

		// Handle any errors that were set during request processing
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			logger.Error("Request error: %v", err.Error())

			// Convert to API error if not already
			if apiErr, ok := err.Err.(utils.APIError); ok {
				utils.ErrorResponse(c, apiErr)
			} else {
				apiErr := utils.NewAPIError(500, "Internal server error", err.Error())
				utils.ErrorResponse(c, apiErr)
			}
		}
	}
}

// CORSMiddleware handles CORS headers
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// ValidationMiddleware validates request parameters
func ValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		validationErrors := utils.ValidateRequest(c)

		if validationErrors.HasErrors() {
			apiErr := utils.NewAPIError(400, "Validation failed", "Request parameters are invalid")
			c.JSON(400, gin.H{
				"error":             apiErr,
				"validation_errors": validationErrors.Errors,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
