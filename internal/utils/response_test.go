package utils

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIError(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		message    string
		details    string
		wantCode   int
		wantMsg    string
		wantDetail string
	}{
		{
			name:       "Basic error",
			code:       400,
			message:    "Bad request",
			details:    "Invalid input",
			wantCode:   400,
			wantMsg:    "Bad request",
			wantDetail: "Invalid input",
		},
		{
			name:       "Server error",
			code:       500,
			message:    "Internal error",
			details:    "Database connection failed",
			wantCode:   500,
			wantMsg:    "Internal error",
			wantDetail: "Database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAPIError(tt.code, tt.message, tt.details)

			assert.Equal(t, tt.wantCode, err.Code)
			assert.Equal(t, tt.wantMsg, err.Message)
			assert.Equal(t, tt.wantDetail, err.Details)
			assert.NotEmpty(t, err.Timestamp)
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	err := NewAPIError(400, "Bad request", "Invalid input")
	expected := "Bad request: Invalid input"
	assert.Equal(t, expected, err.Error())
}

func TestErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		apiError     APIError
		expectedCode int
		expectedBody map[string]interface{}
	}{
		{
			name:         "Client error",
			apiError:     NewAPIError(400, "Bad request", "Invalid input"),
			expectedCode: 400,
			expectedBody: map[string]interface{}{
				"error": map[string]interface{}{
					"code":    float64(400),
					"message": "Bad request",
					"details": "Invalid input",
				},
			},
		},
		{
			name:         "Server error",
			apiError:     NewAPIError(500, "Internal error", "Database failed"),
			expectedCode: 500,
			expectedBody: map[string]interface{}{
				"error": map[string]interface{}{
					"code":    float64(500),
					"message": "Internal error",
					"details": "Database failed",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			ErrorResponse(c, tt.apiError)

			assert.Equal(t, tt.expectedCode, w.Code)

			var responseBody map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &responseBody)
			require.NoError(t, err)

			// Check structure (timestamp will vary)
			errorObj := responseBody["error"].(map[string]interface{})
			assert.Equal(t, tt.expectedBody["error"].(map[string]interface{})["code"], errorObj["code"])
			assert.Equal(t, tt.expectedBody["error"].(map[string]interface{})["message"], errorObj["message"])
			assert.Equal(t, tt.expectedBody["error"].(map[string]interface{})["details"], errorObj["details"])
			assert.NotEmpty(t, errorObj["timestamp"])
		})
	}
}

func TestValidationErrors(t *testing.T) {
	ve := NewValidationErrors()

	// Test empty validation errors
	assert.False(t, ve.HasErrors())
	assert.Len(t, ve.Errors, 0)

	// Test adding errors
ve.AddError("field1", "Field is required", "")
ve.AddError("field2", "Invalid format", "")

	assert.True(t, ve.HasErrors())
	assert.Len(t, ve.Errors, 2)
assert.Equal(t, "field1", ve.Errors[0].Field)
assert.Equal(t, "Field is required", ve.Errors[0].Message)
assert.Equal(t, "field2", ve.Errors[1].Field)
assert.Equal(t, "Invalid format", ve.Errors[1].Message)
}

func TestValidateRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		setupContext func() *gin.Context
		expectErrors bool
		expectedKeys []string
	}{
		{
			name: "Valid request",
			setupContext: func() *gin.Context {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("GET", "/test?page=1&limit=10", nil)
				return c
			},
			expectErrors: false,
		},
		{
			name: "Invalid page parameter",
			setupContext: func() *gin.Context {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("GET", "/test?page=invalid&limit=10", nil)
				return c
			},
			expectErrors: true,
			expectedKeys: []string{"page"},
		},
		{
			name: "Invalid limit parameter",
			setupContext: func() *gin.Context {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("GET", "/test?page=1&limit=invalid", nil)
				return c
			},
			expectErrors: true,
			expectedKeys: []string{"limit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupContext()
			validationErrors := ValidateRequest(c)

			if tt.expectErrors {
				assert.True(t, validationErrors.HasErrors())
				for _, key := range tt.expectedKeys {
					assert.Contains(t, validationErrors.Errors, key)
				}
			} else {
				assert.False(t, validationErrors.HasErrors())
			}
		})
	}
}

func TestPaginationParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		url           string
		expectedPage  int
		expectedLimit int
	}{
		{
			name:          "Default parameters",
			url:           "/test",
			expectedPage:  1,
			expectedLimit: 20,
		},
		{
			name:          "Custom parameters",
			url:           "/test?page=3&limit=50",
			expectedPage:  3,
			expectedLimit: 50,
		},
		{
			name:          "Invalid parameters use defaults",
			url:           "/test?page=0&limit=-10",
			expectedPage:  1,
			expectedLimit: 20,
		},
		{
			name:          "Limit too high gets capped",
			url:           "/test?page=1&limit=200",
			expectedPage:  1,
			expectedLimit: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", tt.url, nil)

			params := GetPaginationParams(c)

			assert.Equal(t, tt.expectedPage, params.Page)
			assert.Equal(t, tt.expectedLimit, params.Limit)
			assert.Equal(t, (tt.expectedPage-1)*tt.expectedLimit, params.Offset)
		})
	}
}

func TestSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testData := map[string]interface{}{
		"message": "Operation successful",
		"count":   5,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

SuccessResponse(c, testData)

	assert.Equal(t, 200, w.Code)

	var responseBody map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &responseBody)
	require.NoError(t, err)

assert.Equal(t, testData["message"], responseBody["data"].(map[string]interface{})["message"])
assert.Equal(t, float64(5), responseBody["data"].(map[string]interface{})["count"]) // JSON unmarshals numbers as float64
}

func TestPaginatedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testData := []map[string]interface{}{
		{"id": 1, "name": "Item 1"},
		{"id": 2, "name": "Item 2"},
	}

	params := PaginationParams{
		Page:   2,
		Limit:  10,
		Offset: 10,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := NewPaginatedResponse(testData, 25, params)
	SuccessResponse(c, resp)

	assert.Equal(t, 200, w.Code)

	var responseBody map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &responseBody)
	require.NoError(t, err)

	// Check data
	data := responseBody["data"].(map[string]interface{})["Data"].([]interface{})
	assert.Len(t, data, 2)

	// Check pagination
	assert.Equal(t, float64(2), responseBody["data"].(map[string]interface{})["Page"])
	assert.Equal(t, float64(10), responseBody["data"].(map[string]interface{})["Limit"])
	assert.Equal(t, float64(25), responseBody["data"].(map[string]interface{})["Total"])
	assert.Equal(t, float64(3), responseBody["data"].(map[string]interface{})["TotalPages"])
	assert.Equal(t, true, responseBody["data"].(map[string]interface{})["HasNext"])
	assert.Equal(t, true, responseBody["data"].(map[string]interface{})["HasPrev"])
}
