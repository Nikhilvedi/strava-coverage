package coverage

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	resty "github.com/go-resty/resty/v2"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// AutomationService handles background processing and webhooks
type AutomationService struct {
	DB              *storage.DB
	Config          *config.Config
	CoverageService *CoverageService
	CommentService  *CommentService
	client          *resty.Client
}

// NewAutomationService creates a new automation service
func NewAutomationService(db *storage.DB, cfg *config.Config, coverageService *CoverageService, commentService *CommentService) *AutomationService {
	return &AutomationService{
		DB:              db,
		Config:          cfg,
		CoverageService: coverageService,
		CommentService:  commentService,
		client:          resty.New(),
	}
}

// RegisterAutomationRoutes adds automation and webhook endpoints
func (s *AutomationService) RegisterAutomationRoutes(r *gin.Engine) {
	automation := r.Group("/api/automation")
	{
		automation.POST("/webhook", s.StravaWebhookHandler)
		automation.GET("/webhook", s.StravaWebhookValidation)
		automation.POST("/process-user/:userId", s.ProcessAllUserActivitiesHandler)
		automation.POST("/sync-recent/:userId", s.SyncRecentActivitiesHandler)
	}
}

// StravaWebhookEvent represents a webhook event from Strava
type StravaWebhookEvent struct {
	AspectType     string                 `json:"aspect_type"`
	EventTime      int64                  `json:"event_time"`
	ObjectID       int64                  `json:"object_id"`
	ObjectType     string                 `json:"object_type"`
	OwnerID        int64                  `json:"owner_id"`
	SubscriptionID int64                  `json:"subscription_id"`
	Updates        map[string]interface{} `json:"updates"`
}

// StravaWebhookValidation handles Strava webhook challenge validation
func (s *AutomationService) StravaWebhookValidation(c *gin.Context) {
	// Strava sends a GET request for webhook validation
	challenge := c.Query("hub.challenge")
	verifyToken := c.Query("hub.verify_token")
	mode := c.Query("hub.mode")

	// You should set a verify token in your environment
	expectedToken := "strava_webhook_verify_token" // Set this in your .env

	if mode == "subscribe" && verifyToken == expectedToken {
		c.JSON(http.StatusOK, gin.H{"hub.challenge": challenge})
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid verification token"})
	}
}

// StravaWebhookHandler processes incoming webhook events from Strava
func (s *AutomationService) StravaWebhookHandler(c *gin.Context) {
	var event StravaWebhookEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook payload"})
		return
	}

	log.Printf("Received webhook event: %+v", event)

	// Only process activity creation events
	if event.ObjectType == "activity" && event.AspectType == "create" {
		go s.processNewActivity(event.ObjectID, event.OwnerID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event received"})
}

// processNewActivity handles a new activity from webhook
func (s *AutomationService) processNewActivity(activityID int64, athleteID int64) {
	log.Printf("Processing new activity %d for athlete %d", activityID, athleteID)

	// Find user by Strava athlete ID
	var userID int
	err := s.DB.QueryRow("SELECT id FROM users WHERE strava_id = $1", athleteID).Scan(&userID)
	if err != nil {
		log.Printf("User not found for athlete ID %d: %v", athleteID, err)
		return
	}

	// Import the activity
	err = s.importActivityByID(activityID, userID)
	if err != nil {
		log.Printf("Failed to import activity %d: %v", activityID, err)
		return
	}

	// Calculate coverage
	result, err := s.calculateAndStoreCoverage(activityID)
	if err != nil {
		log.Printf("Failed to calculate coverage for activity %d: %v", activityID, err)
		return
	}

	// Post comment if coverage was calculated
	if result != nil && result.CoveragePercent > 0 {
		err = s.postCoverageComment(activityID, result)
		if err != nil {
			log.Printf("Failed to post comment for activity %d: %v", activityID, err)
		} else {
			log.Printf("Successfully processed activity %d: %.1f%% coverage in %s",
				activityID, result.CoveragePercent, result.CityName)
		}
	}
}

// importActivityByID imports a specific activity by its Strava ID
func (s *AutomationService) importActivityByID(activityID int64, userID int) error {
	// Get user's access token
	tokenPtr, err := s.DB.GetStravaToken(userID)
	if err != nil {
		return fmt.Errorf("no access token for user %d: %v", userID, err)
	}

	// Fetch activity streams from Strava
	resp, err := s.client.R().
		SetAuthToken(tokenPtr.AccessToken).
		SetQueryParam("keys", "latlng").
		Get(fmt.Sprintf("https://www.strava.com/api/v3/activities/%d/streams", activityID))

	if err != nil || resp.StatusCode() != 200 {
		return fmt.Errorf("failed to fetch activity stream: %v, status: %d", err, resp.StatusCode())
	}

	// Parse the response (reuse logic from import.go)
	var rawJSON []map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &rawJSON); err != nil {
		return fmt.Errorf("failed to parse stream response: %v", err)
	}

	var latlngData [][]float64
	for _, stream := range rawJSON {
		if stream["type"] == "latlng" {
			if dataRaw, ok := stream["data"].([]interface{}); ok {
				for _, pointRaw := range dataRaw {
					if point, ok := pointRaw.([]interface{}); ok && len(point) == 2 {
						lat, latOk := point[0].(float64)
						lng, lngOk := point[1].(float64)
						if latOk && lngOk {
							latlngData = append(latlngData, []float64{lat, lng})
						}
					}
				}
			}
		}
	}

	// Handle activities with or without GPS data
	var query string

	if len(latlngData) == 0 {
		// Indoor activity - no GPS data
		query = `
			INSERT INTO activities (
				user_id, 
				strava_activity_id, 
				path,
				city_id,
				coverage_percentage,
				comment_posted,
				created_at,
				updated_at
			) VALUES (
				$1, $2, NULL,
				NULL, NULL, false,
				CURRENT_TIMESTAMP,
				CURRENT_TIMESTAMP
			) ON CONFLICT (strava_activity_id) DO NOTHING`

		_, err = s.DB.Exec(query, userID, activityID)
	} else {
		// Outdoor activity - has GPS data
		// Convert to WKT LINESTRING
		var points []string
		for _, ll := range latlngData {
			if len(ll) == 2 {
				points = append(points, fmt.Sprintf("%f %f", ll[1], ll[0])) // WKT: lon lat
			}
		}
		linestring := fmt.Sprintf("LINESTRING(%s)", strings.Join(points, ", "))

		query = `
			INSERT INTO activities (
				user_id, 
				strava_activity_id, 
				path,
				city_id,
				coverage_percentage,
				comment_posted,
				created_at,
				updated_at
			) VALUES (
				$1, $2, ST_GeomFromText($3, 4326),
				NULL, NULL, false,
				CURRENT_TIMESTAMP,
				CURRENT_TIMESTAMP
			) ON CONFLICT (strava_activity_id) DO NOTHING`

		_, err = s.DB.Exec(query, userID, activityID, linestring)
	}

	return err
}

// calculateAndStoreCoverage calculates coverage for an activity
func (s *AutomationService) calculateAndStoreCoverage(activityID int64) (*CoverageResult, error) {
	// Get activity details
	var userID int
	query := `SELECT user_id FROM activities WHERE strava_activity_id = $1`
	err := s.DB.QueryRow(query, activityID).Scan(&userID)
	if err != nil {
		return nil, err
	}

	// Find intersecting city
	cityQuery := `
		SELECT c.id, c.name
		FROM cities c, activities a
		WHERE a.strava_activity_id = $1
		AND ST_Intersects(a.path, c.boundary)
		ORDER BY ST_Length(ST_Intersection(a.path, c.boundary)) DESC
		LIMIT 1`

	var cityID int
	var cityName string
	err = s.DB.QueryRow(cityQuery, activityID).Scan(&cityID, &cityName)
	if err != nil {
		return nil, err // Activity doesn't intersect with tracked cities
	}

	// Calculate coverage
	result, err := s.CoverageService.calculateGridBasedCoverage(userID, activityID, cityID, cityName)
	if err != nil {
		return nil, err
	}

	// Update activity with coverage data
	updateQuery := `
		UPDATE activities 
		SET city_id = $1, coverage_percentage = $2, updated_at = CURRENT_TIMESTAMP
		WHERE strava_activity_id = $3`

	_, err = s.DB.Exec(updateQuery, cityID, result.CoveragePercent, activityID)
	if err != nil {
		log.Printf("Warning: Failed to update activity coverage: %v", err)
	}

	return result, nil
}

// postCoverageComment posts a coverage comment for an activity
func (s *AutomationService) postCoverageComment(activityID int64, result *CoverageResult) error {
	// Get activity details
	var userID int
	query := `SELECT user_id FROM activities WHERE strava_activity_id = $1`
	err := s.DB.QueryRow(query, activityID).Scan(&userID)
	if err != nil {
		return err
	}

	// Get user's access token
	tokenPtr, err := s.DB.GetStravaToken(userID)
	if err != nil {
		return err
	}

	// Generate and post comment
	commentText := s.CommentService.generateCoverageComment(result.CityName, result.CoveragePercent, userID)
	err = s.CommentService.postStravaComment(activityID, commentText, tokenPtr.AccessToken)
	if err != nil {
		return err
	}

	// Mark as commented
	_, err = s.DB.Exec("UPDATE activities SET comment_posted = true WHERE strava_activity_id = $1", activityID)
	return err
}

// ProcessAllUserActivitiesHandler processes all activities for a user
func (s *AutomationService) ProcessAllUserActivitiesHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Process in background
	go s.processAllUserActivities(userID)

	c.JSON(http.StatusAccepted, gin.H{"message": "Processing started for user activities"})
}

// processAllUserActivities processes all activities for a user
func (s *AutomationService) processAllUserActivities(userID int) {
	log.Printf("Processing all activities for user %d", userID)

	// Get activities without coverage
	query := `
		SELECT strava_activity_id 
		FROM activities 
		WHERE user_id = $1 AND coverage_percentage IS NULL
		ORDER BY created_at DESC`

	rows, err := s.DB.Query(query, userID)
	if err != nil {
		log.Printf("Failed to fetch activities for user %d: %v", userID, err)
		return
	}
	defer rows.Close()

	var processed, failed int
	for rows.Next() {
		var activityID int64
		if err := rows.Scan(&activityID); err != nil {
			failed++
			continue
		}

		result, err := s.calculateAndStoreCoverage(activityID)
		if err != nil {
			log.Printf("Failed to calculate coverage for activity %d: %v", activityID, err)
			failed++
			continue
		}

		if result != nil && result.CoveragePercent > 0 {
			err = s.postCoverageComment(activityID, result)
			if err != nil {
				log.Printf("Failed to post comment for activity %d: %v", activityID, err)
			}
		}

		processed++
		time.Sleep(time.Second) // Rate limiting
	}

	log.Printf("Completed processing for user %d: %d processed, %d failed", userID, processed, failed)
}

// SyncRecentActivitiesHandler syncs recent activities from Strava
func (s *AutomationService) SyncRecentActivitiesHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get recent activities from Strava API
	activities, err := s.fetchRecentActivities(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch activities: %v", err)})
		return
	}

	// Process each activity
	var imported, failed int
	for _, activity := range activities {
		err := s.importActivityByID(activity.ID, userID)
		if err != nil {
			log.Printf("Failed to import activity %d: %v", activity.ID, err)
			failed++
			continue
		}
		imported++
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Sync completed",
		"imported": imported,
		"failed":   failed,
		"total":    len(activities),
	})
}

// StravaActivity represents a basic Strava activity
type StravaActivity struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// fetchRecentActivities fetches recent activities from Strava
func (s *AutomationService) fetchRecentActivities(userID int) ([]StravaActivity, error) {
	tokenPtr, err := s.DB.GetStravaToken(userID)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.R().
		SetAuthToken(tokenPtr.AccessToken).
		SetQueryParam("per_page", "30").
		Get("https://www.strava.com/api/v3/athlete/activities")

	if err != nil || resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to fetch activities: %v, status: %d", err, resp.StatusCode())
	}

	var activities []StravaActivity
	err = json.Unmarshal(resp.Body(), &activities)
	return activities, err
}
