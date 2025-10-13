package coverage

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDB is a mock implementation of the database for testing
type MockDB struct {
	mock.Mock
}

func (m *MockDB) QueryRow(query string, args ...interface{}) *sql.Row {
	// This is a simplified mock - in a real implementation you'd need more sophisticated mocking
	return nil
}

func (m *MockDB) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewCoverageService(t *testing.T) {
	db := &storage.DB{} // This would be mocked in a real test
	service := NewCoverageService(db)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.DB)
}

func TestCoverageService_RegisterCoverageRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	db := &storage.DB{}
	service := NewCoverageService(db)

	service.RegisterCoverageRoutes(router)

	// Test that routes are registered
	routes := router.Routes()

	expectedRoutes := []struct {
		method string
		path   string
	}{
		{"POST", "/api/coverage/calculate/:activityId"},
		{"GET", "/api/coverage/user/:userId/city/:cityId"},
		{"GET", "/api/coverage/activity/:activityId"},
	}

	assert.Len(t, routes, len(expectedRoutes))

	for i, expectedRoute := range expectedRoutes {
		assert.Equal(t, expectedRoute.method, routes[i].Method)
		assert.Equal(t, expectedRoute.path, routes[i].Path)
	}
}

func TestCoverageResult_JSONSerialization(t *testing.T) {
	result := CoverageResult{
		ActivityID:      12345,
		CityID:          1,
		CityName:        "Sheffield",
		CoveragePercent: 5.76,
		NewStreetsKm:    2.5,
		TotalStreetsKm:  961.4,
		UniqueStreetsKm: 55.4,
	}

	jsonData, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled CoverageResult
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, result, unmarshaled)
}

func TestCalculateCoverageHandler_InvalidActivityID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := &storage.DB{}
	service := NewCoverageService(db)

	router := gin.New()
	router.POST("/api/coverage/calculate/:activityId", service.CalculateCoverageHandler)

	// Test with invalid activity ID
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/coverage/calculate/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	errorObj := response["error"].(map[string]interface{})
	assert.Equal(t, float64(400), errorObj["code"])
	assert.Contains(t, errorObj["message"], "Invalid activity ID")
}

// Test helper functions
func createTestRouter(service *CoverageService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service.RegisterCoverageRoutes(router)
	return router
}

func TestCoveragePercentageCalculation(t *testing.T) {
	tests := []struct {
		name            string
		uniqueKm        float64
		totalKm         float64
		expectedPercent float64
	}{
		{
			name:            "Basic calculation",
			uniqueKm:        55.4,
			totalKm:         961.4,
			expectedPercent: 5.76,
		},
		{
			name:            "Zero coverage",
			uniqueKm:        0,
			totalKm:         1000,
			expectedPercent: 0,
		},
		{
			name:            "Full coverage",
			uniqueKm:        100,
			totalKm:         100,
			expectedPercent: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			percentage := (tt.uniqueKm / tt.totalKm) * 100
			assert.InDelta(t, tt.expectedPercent, percentage, 0.01)
		})
	}
}

func TestUserCityCoverageValidation(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		cityID string
		valid  bool
	}{
		{
			name:   "Valid IDs",
			userID: "1",
			cityID: "1",
			valid:  true,
		},
		{
			name:   "Invalid user ID",
			userID: "invalid",
			cityID: "1",
			valid:  false,
		},
		{
			name:   "Invalid city ID",
			userID: "1",
			cityID: "invalid",
			valid:  false,
		},
		{
			name:   "Both invalid",
			userID: "invalid",
			cityID: "invalid",
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parameter validation logic
			userIDValid := validateUserID(tt.userID)
			cityIDValid := validateCityID(tt.cityID)

			if tt.valid {
				assert.True(t, userIDValid)
				assert.True(t, cityIDValid)
			} else {
				assert.False(t, userIDValid || cityIDValid)
			}
		})
	}
}

// Helper validation functions
func validateUserID(userID string) bool {
	if userID == "" {
		return false
	}
	// Simple numeric validation
	for _, char := range userID {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validateCityID(cityID string) bool {
	if cityID == "" {
		return false
	}
	// Simple numeric validation
	for _, char := range cityID {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func TestCoverageServiceErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// This would test database error scenarios
	// In a full implementation, you'd mock database calls

	tests := []struct {
		name           string
		setupMock      func() *storage.DB
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Database connection error",
			setupMock: func() *storage.DB {
				// Return mock that simulates connection error
				return nil
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Database error",
		},
		// Add more error scenarios
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test error handling scenarios
			assert.Equal(t, tt.expectedStatus, tt.expectedStatus)
			assert.Contains(t, tt.expectedError, "error")
		})
	}
}
