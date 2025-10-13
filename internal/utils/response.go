package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger provides structured logging functionality
type Logger struct {
	prefix string
}

// NewLogger creates a new logger with a prefix
func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

// Info logs an info message
func (l *Logger) Info(message string, args ...interface{}) {
	log.Printf("[INFO] [%s] %s", l.prefix, fmt.Sprintf(message, args...))
}

// Error logs an error message
func (l *Logger) Error(message string, args ...interface{}) {
	log.Printf("[ERROR] [%s] %s", l.prefix, fmt.Sprintf(message, args...))
}

// Debug logs a debug message
func (l *Logger) Debug(message string, args ...interface{}) {
	log.Printf("[DEBUG] [%s] %s", l.prefix, fmt.Sprintf(message, args...))
}

// Warn logs a warning message
func (l *Logger) Warn(message string, args ...interface{}) {
	log.Printf("[WARN] [%s] %s", l.prefix, fmt.Sprintf(message, args...))
}

// APIError represents a structured API error
type APIError struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"request_id,omitempty"`
}

// Error implements the error interface
func (e APIError) Error() string {
	return fmt.Sprintf("API Error %d: %s", e.Code, e.Message)
}

// NewAPIError creates a new API error
func NewAPIError(code int, message, details string) APIError {
	return APIError{
		Code:      code,
		Message:   message,
		Details:   details,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// Common API errors
var (
	ErrBadRequest         = NewAPIError(http.StatusBadRequest, "Bad Request", "")
	ErrUnauthorized       = NewAPIError(http.StatusUnauthorized, "Unauthorized", "")
	ErrForbidden          = NewAPIError(http.StatusForbidden, "Forbidden", "")
	ErrNotFound           = NewAPIError(http.StatusNotFound, "Not Found", "")
	ErrConflict           = NewAPIError(http.StatusConflict, "Conflict", "")
	ErrInternalServer     = NewAPIError(http.StatusInternalServerError, "Internal Server Error", "")
	ErrServiceUnavailable = NewAPIError(http.StatusServiceUnavailable, "Service Unavailable", "")
)

// ErrorResponse sends a structured error response
func ErrorResponse(c *gin.Context, err APIError) {
	if requestID := c.GetString("request_id"); requestID != "" {
		err.RequestID = requestID
	}

	c.Header("Content-Type", "application/json")
	c.JSON(err.Code, err)
}

// SuccessResponse sends a structured success response
func SuccessResponse(c *gin.Context, data interface{}) {
	response := gin.H{
		"success":   true,
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if requestID := c.GetString("request_id"); requestID != "" {
		response["request_id"] = requestID
	}

	c.JSON(http.StatusOK, response)
}

// ValidationError represents a field validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

// Error implements the error interface
func (ve ValidationErrors) Error() string {
	return fmt.Sprintf("Validation failed: %d errors", len(ve.Errors))
}

// NewValidationErrors creates new validation errors
func NewValidationErrors() ValidationErrors {
	return ValidationErrors{Errors: make([]ValidationError, 0)}
}

// AddError adds a validation error
func (ve *ValidationErrors) AddError(field, message, value string) {
	ve.Errors = append(ve.Errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// HasErrors returns true if there are validation errors
func (ve ValidationErrors) HasErrors() bool {
	return len(ve.Errors) > 0
}

// ValidateRequest validates common request parameters
func ValidateRequest(c *gin.Context) ValidationErrors {
	errors := NewValidationErrors()

	// Validate common parameters
	if userID := c.Param("userId"); userID != "" {
		if !isValidID(userID) {
			errors.AddError("userId", "Invalid user ID format", userID)
		}
	}

	if cityID := c.Param("cityId"); cityID != "" {
		if !isValidID(cityID) {
			errors.AddError("cityId", "Invalid city ID format", cityID)
		}
	}

	if activityID := c.Param("activityId"); activityID != "" {
		if !isValidID(activityID) {
			errors.AddError("activityId", "Invalid activity ID format", activityID)
		}
	}

	return errors
}

// isValidID checks if an ID is valid (positive integer)
func isValidID(id string) bool {
	if id == "" {
		return false
	}

	// Check if it's a valid positive integer
	for _, char := range id {
		if char < '0' || char > '9' {
			return false
		}
	}

	return len(id) <= 19 // Max int64
}

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Limit  int `form:"limit" json:"limit"`
	Offset int `form:"offset" json:"offset"`
	Page   int `form:"page" json:"page"`
}

// GetPaginationParams extracts and validates pagination parameters
func GetPaginationParams(c *gin.Context) PaginationParams {
	var params PaginationParams

	// Bind query parameters
	c.ShouldBindQuery(&params)

	// Set defaults and validate
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 1000 {
		params.Limit = 1000
	}

	if params.Offset < 0 {
		params.Offset = 0
	}

	if params.Page > 0 {
		params.Offset = (params.Page - 1) * params.Limit
	}

	return params
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int         `json:"total"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	Page       int         `json:"page"`
	TotalPages int         `json:"total_pages"`
	HasNext    bool        `json:"has_next"`
	HasPrev    bool        `json:"has_prev"`
}

// NewPaginatedResponse creates a paginated response
func NewPaginatedResponse(data interface{}, total int, params PaginationParams) PaginatedResponse {
	totalPages := (total + params.Limit - 1) / params.Limit
	currentPage := (params.Offset / params.Limit) + 1

	return PaginatedResponse{
		Data:       data,
		Total:      total,
		Limit:      params.Limit,
		Offset:     params.Offset,
		Page:       currentPage,
		TotalPages: totalPages,
		HasNext:    currentPage < totalPages,
		HasPrev:    currentPage > 1,
	}
}

// SafeJSONUnmarshal safely unmarshals JSON with proper error handling
func SafeJSONUnmarshal(data []byte, v interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("empty JSON data")
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// SafeJSONMarshal safely marshals JSON with proper error handling
func SafeJSONMarshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return data, nil
}
