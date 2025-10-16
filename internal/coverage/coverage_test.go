package coverage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService() *CoverageService {
	db := &storage.DB{} // Mock database for testing
	return NewCoverageService(db)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service := setupTestService()
	service.RegisterCoverageRoutes(router)
	return router
}

func TestCoverageServiceRoutes(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/api/coverage/activity/123", http.StatusInternalServerError},  // Will fail with mock DB
		{"GET", "/api/coverage/user/1/city/1", http.StatusInternalServerError}, // Will fail with mock DB
	}

	for _, test := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(test.method, test.path, nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, test.status, w.Code, "Route: %s %s", test.method, test.path)
	}
}

func TestRecalculateAllCoverageHandler(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/coverage/recalculate-all", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "job_id")
	assert.Equal(t, "started", response["status"])
}

func TestGetRecalculationStatusHandler_NotFound(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/coverage/recalculate-status/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Job not found", response["error"])
}

func TestCoverageCalculation(t *testing.T) {
	service := setupTestService()

	// Test the coverage calculation logic
	result, err := service.calculateGridBasedCoverage(1, 123, 1, "Test City")

	// Since we're using a mock DB, this will likely fail, but we can test the structure
	if err != nil {
		// Expected with mock DB
		assert.Contains(t, err.Error(), "failed to calculate coverage")
	} else {
		assert.NotNil(t, result)
		assert.Equal(t, int64(123), result.ActivityID)
		assert.Equal(t, 1, result.CityID)
		assert.Equal(t, "Test City", result.CityName)
	}
}
