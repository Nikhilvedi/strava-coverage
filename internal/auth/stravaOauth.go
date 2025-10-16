package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	resty "github.com/go-resty/resty/v2"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// StravaTokenResponse represents the OAuth token response from Strava
type StravaTokenResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	ExpiresAt    int64  `json:"expires_at"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Athlete      struct {
		ID int64 `json:"id"`
	} `json:"athlete"`
}

// Service handles Strava OAuth authentication
type Service struct {
	config        *config.Config
	client        *resty.Client
	db            *storage.DB
	autoProcessor *AutoProcessor
}

// NewService creates a new auth service
func NewService(cfg *config.Config, db *storage.DB) *Service {
	return &Service{
		config:        cfg,
		client:        resty.New(),
		db:            db,
		autoProcessor: NewAutoProcessor(db, cfg),
	}
}

// SetupRoutes configures the OAuth routes
func (s *Service) SetupRoutes(r *gin.Engine) {
	auth := r.Group("/oauth")
	{
		auth.GET("/authorize", s.handleAuthorize)
		auth.GET("/callback", s.handleCallback)
	}

	// User info routes
	users := r.Group("/api/users")
	{
		users.GET("/:id", s.GetUserHandler)
		users.GET("/:id/processing-status", s.GetProcessingStatusHandler)
		users.POST("/:id/discover-cities", s.DiscoverCitiesHandler)
	}
}

// handleAuthorize redirects the user to Strava's authorization page
func (s *Service) handleAuthorize(c *gin.Context) {
	url := fmt.Sprintf(
		"https://www.strava.com/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=activity:read_all,activity:read",
		s.config.StravaClientID,
		s.config.StravaRedirectURI,
	)
	c.Redirect(http.StatusFound, url)
}

// handleCallback processes the OAuth callback from Strava
func (s *Service) handleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}

	// Exchange auth code for token
	resp, err := s.client.R().
		SetFormData(map[string]string{
			"client_id":     s.config.StravaClientID,
			"client_secret": s.config.StravaClientSecret,
			"code":          code,
			"grant_type":    "authorization_code",
		}).
		Post("https://www.strava.com/oauth/token")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	var tokenResp StravaTokenResponse
	if err := json.Unmarshal(resp.Body(), &tokenResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token response"})
		return
	}

	// Fetch athlete details from Strava API to get their actual name
	athleteResp, err := s.client.R().
		SetHeader("Authorization", "Bearer "+tokenResp.AccessToken).
		Get("https://www.strava.com/api/v3/athlete")

	var athleteName string
	if err != nil {
		fmt.Printf("Failed to fetch athlete details: %v\n", err)
		athleteName = "Strava User" // Fallback
	} else {
		var athlete struct {
			FirstName string `json:"firstname"`
			LastName  string `json:"lastname"`
		}
		if err := json.Unmarshal(athleteResp.Body(), &athlete); err != nil {
			fmt.Printf("Failed to parse athlete details: %v\n", err)
			athleteName = "Strava User" // Fallback
		} else {
			athleteName = fmt.Sprintf("%s %s", athlete.FirstName, athlete.LastName)
		}
	}

	// Get or create user in the database
	user, err := s.db.GetUserByStravaID(tokenResp.Athlete.ID)
	if err != nil {
		// If user doesn't exist, create them
		user, err = s.db.CreateUser(tokenResp.Athlete.ID, athleteName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	} else {
		// User exists, update their name if it's different or empty
		if user.Name != athleteName && athleteName != "Strava User" {
			if err := s.db.UpdateUserName(user.ID, athleteName); err != nil {
				fmt.Printf("Failed to update user name: %v\n", err)
			} else {
				user.Name = athleteName // Update the local copy
			}
		}
	}

	token := &storage.StravaToken{
		UserID:       user.ID,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Unix(tokenResp.ExpiresAt, 0),
	}

	if err := s.db.UpsertStravaToken(user.ID, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store token"})
		return
	}

	// Start automatic processing in background (non-blocking)
	go func() {
		if err := s.autoProcessor.ProcessUserOnLogin(user.ID, tokenResp.AccessToken); err != nil {
			fmt.Printf("Auto-processing failed for user %d: %v\n", user.ID, err)
		}
	}()

	// Redirect to frontend OAuth callback with user data
	frontendURL := fmt.Sprintf(
		"http://localhost:3000/oauth/callback?user_id=%d&user_name=%s&strava_id=%d&success=true",
		user.ID,
		user.Name, // Use the name we already fetched and stored/updated
		user.StravaID,
	)

	c.Redirect(http.StatusFound, frontendURL)
}

// GetProcessingStatusHandler returns the processing status for a user
func (s *Service) GetProcessingStatusHandler(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Check if user has activities
	var activityCount, citiesCount int
	err = s.db.QueryRow("SELECT COUNT(*) FROM activities WHERE user_id = $1", userID).Scan(&activityCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check activity count"})
		return
	}

	err = s.db.QueryRow(`
		SELECT COUNT(DISTINCT city_id) 
		FROM activities 
		WHERE user_id = $1 AND city_id IS NOT NULL`, userID).Scan(&citiesCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check cities count"})
		return
	}

	// Check if coverage has been calculated
	var coverageCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) 
		FROM activities 
		WHERE user_id = $1 AND coverage_percentage IS NOT NULL`, userID).Scan(&coverageCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check coverage count"})
		return
	}

	status := "not_started"
	if activityCount > 0 {
		if citiesCount > 0 {
			if coverageCount > 0 {
				status = "completed"
			} else {
				status = "calculating_coverage"
			}
		} else {
			status = "mapping_cities"
		}
	} else {
		status = "importing_activities"
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":        userID,
		"status":         status,
		"activity_count": activityCount,
		"cities_count":   citiesCount,
		"coverage_count": coverageCount,
	})
}

// DiscoverCitiesHandler manually triggers city discovery for a user
func (s *Service) DiscoverCitiesHandler(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Run city discovery in background
	go func() {
		log.Printf("Starting manual city discovery for user %d", userID)
		if err := s.autoProcessor.mapActivitiesToCities(userID); err != nil {
			log.Printf("City discovery failed for user %d: %v", userID, err)
		} else {
			log.Printf("City discovery completed for user %d", userID)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "City discovery started for user " + userIDStr,
		"user_id": userID,
	})
}
