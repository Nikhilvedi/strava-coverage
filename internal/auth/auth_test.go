package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
	"github.com/stretchr/testify/assert"
)

func setupAuthTestService() *Service {
	cfg := &config.Config{
		StravaClientID:     "test_client_id",
		StravaClientSecret: "test_client_secret",
		StravaRedirectURI:  "http://localhost:8080/oauth/callback",
		DBUrl:              "test_db_url",
	}
	db := &storage.DB{} // Mock database for testing
	return NewService(cfg, db)
}

func setupAuthTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service := setupAuthTestService()
	service.SetupRoutes(router)
	return router
}

func TestAuthRoutes(t *testing.T) {
	router := setupAuthTestRouter()

	tests := []struct {
		method   string
		path     string
		status   int
		testName string
	}{
		{"GET", "/oauth/authorize", http.StatusFound, "OAuth authorize redirect"},
		{"GET", "/oauth/callback", http.StatusBadRequest, "OAuth callback without code"},
		{"GET", "/api/users/1", http.StatusInternalServerError, "Get user info"},
		{"GET", "/api/users/1/processing-status", http.StatusInternalServerError, "Get processing status"},
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

func TestOAuthAuthorizeRedirect(t *testing.T) {
	router := setupAuthTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/oauth/authorize", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)

	location := w.Header().Get("Location")
	assert.Contains(t, location, "https://www.strava.com/oauth/authorize")
	assert.Contains(t, location, "client_id=test_client_id")
	assert.Contains(t, location, "response_type=code")
	assert.Contains(t, location, "scope=read,activity:read")
}

func TestOAuthCallbackMissingCode(t *testing.T) {
	router := setupAuthTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/oauth/callback", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization code not provided")
}

func TestInvalidUserIDInUserRoute(t *testing.T) {
	router := setupAuthTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users/invalid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostDiscoverCities(t *testing.T) {
	router := setupAuthTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/users/1/discover-cities", nil)
	router.ServeHTTP(w, req)

	// Even with mock DB, this should return OK since it starts in background
	assert.Equal(t, http.StatusOK, w.Code)
}
