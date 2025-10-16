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
	"github.com/go-resty/resty/v2"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// InitialImportService handles bulk import of user's historical activities
type InitialImportService struct {
	DB               *storage.DB
	Config           *config.Config
	CoverageService  *CoverageService
	CommentService   *CommentService
	DetectionService *CityDetectionService
	client           *resty.Client
}

// NewInitialImportService creates a new initial import service
func NewInitialImportService(db *storage.DB, cfg *config.Config, coverageService *CoverageService, commentService *CommentService, detectionService *CityDetectionService) *InitialImportService {
	return &InitialImportService{
		DB:               db,
		Config:           cfg,
		CoverageService:  coverageService,
		CommentService:   commentService,
		DetectionService: detectionService,
		client:           resty.New(),
	}
}

// RegisterInitialImportRoutes adds initial import endpoints
func (s *InitialImportService) RegisterInitialImportRoutes(r *gin.Engine) {
	imports := r.Group("/api/import")
	{
		imports.POST("/initial/:userId", s.InitialImportHandler)
		imports.GET("/status/:userId", s.ImportStatusHandler)
		imports.POST("/process-imported/:userId", s.ProcessImportedActivitiesHandler)
	}
}

// StravaActivitySummary represents a Strava activity from the list endpoint
type StravaActivitySummary struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	Distance           float64   `json:"distance"`
	MovingTime         int       `json:"moving_time"`
	ElapsedTime        int       `json:"elapsed_time"`
	TotalElevationGain float64   `json:"total_elevation_gain"`
	Type               string    `json:"type"`
	SportType          string    `json:"sport_type"`
	StartDate          string    `json:"start_date"`
	StartDateLocal     string    `json:"start_date_local"`
	StartLatlng        []float64 `json:"start_latlng"`
	EndLatlng          []float64 `json:"end_latlng"`
	Map                struct {
		ID              string `json:"id"`
		SummaryPolyline string `json:"summary_polyline"`
		ResourceState   int    `json:"resource_state"`
	} `json:"map"`
}

// ImportStatus represents the status of an import operation
type ImportStatus struct {
	UserID             int       `json:"user_id"`
	TotalActivities    int       `json:"total_activities"`
	ImportedCount      int       `json:"imported_count"`
	ProcessedCount     int       `json:"processed_count"`
	FailedCount        int       `json:"failed_count"`
	LastImportTime     time.Time `json:"last_import_time"`
	InProgress         bool      `json:"in_progress"`
	CurrentPage        int       `json:"current_page"`
	EstimatedRemaining int       `json:"estimated_remaining"`
}

// InitialImportHandler starts the initial import process for a user
func (s *InitialImportService) InitialImportHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Check if import is already in progress
	status, err := s.getImportStatus(userID)
	if err == nil && status.InProgress {
		c.JSON(http.StatusConflict, gin.H{"error": "Import already in progress for this user"})
		return
	}

	// Start the import in background
	go s.performInitialImport(userID)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Initial import started",
		"user_id": userID,
		"note":    "This process will run in the background. Use /api/import/status/:userId to check progress",
	})
}

// performInitialImport performs the actual import process
func (s *InitialImportService) performInitialImport(userID int) {
	log.Printf("Starting initial import for user %d", userID)

	// Mark import as in progress
	s.setImportInProgress(userID, true)
	defer s.setImportInProgress(userID, false)

	// Get user's access token
	tokenPtr, err := s.DB.GetStravaToken(userID)
	if err != nil {
		log.Printf("Failed to get token for user %d: %v", userID, err)
		return
	}

	var totalImported, totalFailed int
	page := 1
	perPage := 100 // Strava's max per page

	for {
		log.Printf("Fetching page %d for user %d", page, userID)

		// Fetch activities from Strava
		activities, hasMore, err := s.fetchActivitiesPage(tokenPtr.AccessToken, page, perPage)
		if err != nil {
			log.Printf("Failed to fetch activities page %d for user %d: %v", page, userID, err)
			break
		}

		if len(activities) == 0 {
			break
		}

		// Import each activity
		for _, activity := range activities {
			// Only import running/cycling activities with GPS data
			if s.shouldImportActivity(activity) {
				err := s.importSingleActivity(userID, activity.ID, activity.Type, activity.SportType, tokenPtr.AccessToken)
				if err != nil {
					log.Printf("Failed to import activity %d: %v", activity.ID, err)
					totalFailed++
				} else {
					totalImported++
					log.Printf("Imported activity %d (%s)", activity.ID, activity.Name)
				}

				// Rate limiting - Strava allows 1000 requests per 15 minutes
				time.Sleep(100 * time.Millisecond)
			}
		}

		// Update status
		s.updateImportStatus(userID, page, totalImported, totalFailed)

		if !hasMore {
			break
		}

		page++

		// Additional safety check to prevent infinite loops
		if page > 1000 {
			log.Printf("Stopping import for user %d - reached page limit", userID)
			break
		}
	}

	log.Printf("Completed initial import for user %d: %d imported, %d failed", userID, totalImported, totalFailed)

	// Mark import as complete
	s.finalizeImportStatus(userID, totalImported, totalFailed)
}

// fetchActivitiesPage fetches a page of activities from Strava
func (s *InitialImportService) fetchActivitiesPage(accessToken string, page, perPage int) ([]StravaActivitySummary, bool, error) {
	resp, err := s.client.R().
		SetAuthToken(accessToken).
		SetQueryParam("page", strconv.Itoa(page)).
		SetQueryParam("per_page", strconv.Itoa(perPage)).
		Get("https://www.strava.com/api/v3/athlete/activities")

	if err != nil {
		return nil, false, fmt.Errorf("HTTP request failed: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, false, fmt.Errorf("strava API error: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	var activities []StravaActivitySummary
	err = json.Unmarshal(resp.Body(), &activities)
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse activities: %v", err)
	}

	// If we got less than perPage, there are no more pages
	hasMore := len(activities) == perPage

	return activities, hasMore, nil
}

// shouldImportActivity determines if an activity should be imported
func (s *InitialImportService) shouldImportActivity(activity StravaActivitySummary) bool {
	// Only import activities with GPS data
	if len(activity.StartLatlng) == 0 || activity.Map.SummaryPolyline == "" {
		return false
	}

	// Focus on running and cycling activities
	validTypes := []string{"Run", "Ride", "Walk", "Hike", "TrailRun", "VirtualRun", "VirtualRide"}
	for _, validType := range validTypes {
		if activity.Type == validType || activity.SportType == validType {
			return true
		}
	}

	return false
}

// importSingleActivity imports a single activity with streams
func (s *InitialImportService) importSingleActivity(userID int, activityID int64, activityType, sportType, accessToken string) error {
	// Check if activity already exists
	var count int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM activities WHERE strava_activity_id = $1", activityID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing activity: %v", err)
	}
	if count > 0 {
		return nil // Already imported
	}

	// Fetch activity streams
	resp, err := s.client.R().
		SetAuthToken(accessToken).
		SetQueryParam("keys", "latlng,time,distance").
		Get(fmt.Sprintf("https://www.strava.com/api/v3/activities/%d/streams", activityID))

	if err != nil {
		return fmt.Errorf("failed to fetch streams: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("strava API error: %d", resp.StatusCode())
	}

	// Parse streams
	latlngData, err := s.parseActivityStreams(resp.Body())
	if err != nil {
		return fmt.Errorf("failed to parse streams: %v", err)
	}

	if len(latlngData) == 0 {
		return fmt.Errorf("no GPS data found")
	}

	// Convert to WKT LINESTRING
	var points []string
	for _, ll := range latlngData {
		if len(ll) == 2 {
			points = append(points, fmt.Sprintf("%f %f", ll[1], ll[0])) // WKT: lon lat
		}
	}
	linestring := fmt.Sprintf("LINESTRING(%s)", strings.Join(points, ", "))

	// Insert into database
	query := `
		INSERT INTO activities (
			user_id, 
			strava_activity_id, 
			path,
			activity_type,
			sport_type,
			city_id,
			coverage_percentage,
			comment_posted,
			created_at,
			updated_at
		) VALUES (
			$1, $2, ST_GeomFromText($3, 4326),
			$4, $5,
			NULL, NULL, false,
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP
		)`

	_, err = s.DB.Exec(query, userID, activityID, linestring, activityType, sportType)
	return err
}

// parseActivityStreams parses Strava activity streams response
func (s *InitialImportService) parseActivityStreams(body []byte) ([][]float64, error) {
	var rawJSON []map[string]interface{}
	if err := json.Unmarshal(body, &rawJSON); err != nil {
		return nil, err
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

	return latlngData, nil
}

// ImportStatusHandler returns the current import status for a user
func (s *InitialImportService) ImportStatusHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	status, err := s.getImportStatus(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No import status found"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ProcessImportedActivitiesHandler processes all imported activities for coverage
func (s *InitialImportService) ProcessImportedActivitiesHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Start processing in background
	go s.processAllImportedActivities(userID)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Processing started for imported activities",
		"user_id": userID,
	})
}

// processAllImportedActivities calculates coverage for all imported activities
func (s *InitialImportService) processAllImportedActivities(userID int) {
	log.Printf("Processing imported activities for user %d", userID)

	// Get all activities without coverage
	query := `
		SELECT strava_activity_id 
		FROM activities 
		WHERE user_id = $1 AND coverage_percentage IS NULL
		ORDER BY created_at ASC`

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

		// Calculate coverage for this activity
		err := s.calculateActivityCoverage(activityID, userID)
		if err != nil {
			log.Printf("Failed to calculate coverage for activity %d: %v", activityID, err)
			failed++
		} else {
			processed++
		}

		// Rate limiting
		time.Sleep(100 * time.Millisecond)

		if processed%10 == 0 {
			log.Printf("Processed %d activities for user %d", processed, userID)
		}
	}

	log.Printf("Completed processing for user %d: %d processed, %d failed", userID, processed, failed)
}

// calculateActivityCoverage calculates coverage for a specific activity
func (s *InitialImportService) calculateActivityCoverage(activityID int64, userID int) error {
	// Find intersecting cities for this activity
	intersections, err := s.findIntersectingCities(activityID)
	if err != nil || len(intersections) == 0 {
		return fmt.Errorf("activity doesn't intersect with any tracked cities")
	}

	// Use the city with the longest intersection (first in sorted list)
	primaryCity := intersections[0]

	// Calculate coverage
	result, err := s.CoverageService.calculateGridBasedCoverage(userID, activityID, primaryCity.CityID, primaryCity.CityName)
	if err != nil {
		return err
	}

	// Update activity with coverage data
	updateQuery := `
		UPDATE activities 
		SET city_id = $1, coverage_percentage = $2, updated_at = CURRENT_TIMESTAMP
		WHERE strava_activity_id = $3`

	_, err = s.DB.Exec(updateQuery, primaryCity.CityID, result.CoveragePercent, activityID)
	return err
}

// findIntersectingCities finds all cities that intersect with an activity
func (s *InitialImportService) findIntersectingCities(activityID int64) ([]CityIntersection, error) {
	query := `
		SELECT 
			c.id,
			c.name,
			c.country_code,
			ST_Length(ST_Transform(ST_Intersection(a.path, c.boundary), 3857)) / 1000 as intersection_km,
			(ST_Length(ST_Transform(ST_Intersection(a.path, c.boundary), 3857)) / 
			 ST_Length(ST_Transform(a.path, 3857))) * 100 as percentage_of_activity
		FROM cities c, activities a
		WHERE a.strava_activity_id = $1
		AND ST_Intersects(a.path, c.boundary)
		AND ST_Length(ST_Intersection(a.path, c.boundary)) > 0
		ORDER BY intersection_km DESC`

	rows, err := s.DB.Query(query, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to query intersecting cities: %v", err)
	}
	defer rows.Close()

	var intersections []CityIntersection
	for rows.Next() {
		var intersection CityIntersection
		err := rows.Scan(&intersection.CityID, &intersection.CityName, &intersection.CountryCode,
			&intersection.IntersectionLength, &intersection.PercentageOfActivity)
		if err != nil {
			continue
		}
		intersections = append(intersections, intersection)
	}

	return intersections, nil
} // Helper functions for import status management
func (s *InitialImportService) getImportStatus(userID int) (*ImportStatus, error) {
	query := `
		SELECT 
			user_id, total_activities, imported_count, processed_count, failed_count,
			COALESCE(last_import_time, '1970-01-01'::timestamp), in_progress, current_page, estimated_remaining
		FROM import_status 
		WHERE user_id = $1`

	var status ImportStatus
	err := s.DB.QueryRow(query, userID).Scan(
		&status.UserID, &status.TotalActivities, &status.ImportedCount,
		&status.ProcessedCount, &status.FailedCount, &status.LastImportTime,
		&status.InProgress, &status.CurrentPage, &status.EstimatedRemaining)

	if err != nil {
		return nil, err
	}

	return &status, nil
}

func (s *InitialImportService) setImportInProgress(userID int, inProgress bool) {
	if inProgress {
		// Initialize or reset import status
		query := `
			INSERT INTO import_status (user_id, in_progress, started_at, current_page)
			VALUES ($1, true, CURRENT_TIMESTAMP, 1)
			ON CONFLICT (user_id) 
			DO UPDATE SET 
				in_progress = true,
				started_at = CURRENT_TIMESTAMP,
				current_page = 1,
				completed_at = NULL,
				error_message = NULL,
				updated_at = CURRENT_TIMESTAMP`
		_, err := s.DB.Exec(query, userID)
		if err != nil {
			log.Printf("Failed to set import in progress for user %d: %v", userID, err)
		}
	} else {
		// Mark as completed
		query := `
			UPDATE import_status 
			SET in_progress = false, completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = $1`
		_, err := s.DB.Exec(query, userID)
		if err != nil {
			log.Printf("Failed to mark import complete for user %d: %v", userID, err)
		}
	}
}

func (s *InitialImportService) updateImportStatus(userID, currentPage, imported, failed int) {
	query := `
		UPDATE import_status 
		SET 
			current_page = $2,
			imported_count = $3,
			failed_count = $4,
			last_import_time = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1`

	_, err := s.DB.Exec(query, userID, currentPage, imported, failed)
	if err != nil {
		log.Printf("Failed to update import status for user %d: %v", userID, err)
	}
}

func (s *InitialImportService) finalizeImportStatus(userID, imported, failed int) {
	query := `
		UPDATE import_status 
		SET 
			imported_count = $2,
			failed_count = $3,
			in_progress = false,
			completed_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1`

	_, err := s.DB.Exec(query, userID, imported, failed)
	if err != nil {
		log.Printf("Failed to finalize import status for user %d: %v", userID, err)
	}
	log.Printf("Import completed for user %d: imported %d, failed %d", userID, imported, failed)
}
