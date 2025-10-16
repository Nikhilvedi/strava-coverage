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

func setupCityTestService() *CityService {
	db := &storage.DB{} // Mock database for testing
	return NewCityService(db)
}

func setupCityTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service := setupCityTestService()
	service.RegisterCityRoutes(router)
	return router
}

func TestCityServiceRoutes(t *testing.T) {
	router := setupCityTestRouter()

	tests := []struct {
		method   string
		path     string
		status   int
		testName string
	}{
		{"GET", "/api/cities", http.StatusInternalServerError, "Get all cities"},
		{"GET", "/api/cities/user/1", http.StatusInternalServerError, "Get user cities"},
		{"GET", "/api/cities/user/1/coverage", http.StatusInternalServerError, "Get user cities with coverage"},
		{"GET", "/api/cities/search?q=london", http.StatusOK, "Search cities"}, // This should work even with mock DB
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(test.method, test.path, nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, test.status, w.Code, "Route: %s %s", test.method, test.path)
		})
	}
}

func TestSearchCitiesHandler(t *testing.T) {
	router := setupCityTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/cities/search?q=london", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "cities")
	assert.Contains(t, response, "count")
}

func TestSearchCitiesHandler_EmptyQuery(t *testing.T) {
	router := setupCityTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/cities/search", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Search query is required", response["error"])
}

func TestInvalidUserID(t *testing.T) {
	router := setupCityTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/cities/user/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invalid user ID", response["error"])
}
