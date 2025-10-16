package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func setupTestRouter() *gin.Engine {
	cfg := &config.Config{
		StravaClientID:     "test_client_id",
		StravaClientSecret: "test_client_secret",
		StravaRedirectURI:  "http://localhost:8080/oauth/callback",
		DBUrl:              "test_db_url",
	}

	// For tests, we use a mock DB which will cause most DB operations to fail
	// This is expected behavior for unit tests
	db := &storage.DB{}
	return setupRouter(cfg, db)
}

func TestHealthCheck(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Contains(t, response, "timestamp")
	assert.Equal(t, "1.0.0", response["version"])
}

func TestOAuthAuthorizeEndpoint(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/oauth/authorize", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)

	location := w.Header().Get("Location")
	assert.Contains(t, location, "https://www.strava.com/oauth/authorize")
	assert.Contains(t, location, "client_id=test_client_id")
}

func TestCoverageEndpoints(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		skip       bool
		reason     string
	}{
		{"Health Check", "GET", "/api/health", http.StatusOK, false, ""},
		{"OAuth Authorize", "GET", "/oauth/authorize", http.StatusFound, false, ""},
		{"City Search", "GET", "/api/cities/search?q=london", http.StatusOK, true, "Requires DB connection"},
		{"City Search - No Query", "GET", "/api/cities/search", http.StatusBadRequest, false, ""},
		{"Recalculate All", "POST", "/api/coverage/recalculate-all", http.StatusOK, true, "Starts background process with DB"},
		{"Get Coverage Status - Not Found", "GET", "/api/coverage/recalculate-status/nonexistent", http.StatusNotFound, true, "Requires DB connection"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skipf("Skipping %s: %s", tt.name, tt.reason)
				return
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "Endpoint: %s %s", tt.method, tt.path)

			// For JSON endpoints, verify we can parse the response
			if w.Header().Get("Content-Type") == "application/json; charset=utf-8" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err, "Response should be valid JSON")
			}
		})
	}
}

func TestCORSHeaders(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
}

func TestErrorHandling(t *testing.T) {
	router := setupTestRouter()

	// Test various error conditions
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantError  string
	}{
		{"Invalid Route", "GET", "/api/nonexistent", http.StatusNotFound, ""},
		{"Invalid User ID", "GET", "/api/users/invalid", http.StatusBadRequest, ""},
		{"Invalid Activity ID", "POST", "/api/coverage/calculate/invalid", http.StatusBadRequest, "Invalid activity ID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantError != "" && w.Body.Len() > 0 {
				bodyStr := w.Body.String()
				assert.Contains(t, bodyStr, tt.wantError)
			}
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	// This test verifies that the application can be created without panicking
	// The actual graceful shutdown would be tested in integration tests
	assert.NotPanics(t, func() {
		setupTestRouter()
	})
}

// Benchmark tests for performance
func BenchmarkHealthCheck(b *testing.B) {
	router := setupTestRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/health", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkRouterSetup(b *testing.B) {
	cfg := &config.Config{
		StravaClientID:     "test_client_id",
		StravaClientSecret: "test_client_secret",
		StravaRedirectURI:  "http://localhost:8080/oauth/callback",
		DBUrl:              "test_db_url",
	}
	db := &storage.DB{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		setupRouter(cfg, db)
	}
}
