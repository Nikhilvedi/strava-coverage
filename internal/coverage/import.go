package coverage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"

	// "github.com/jmoiron/sqlx"
	"github.com/go-resty/resty/v2"
)

type ImportService struct {
	DB     *storage.DB
	Config *config.Config
}

func NewImportService(db *storage.DB, cfg *config.Config) *ImportService {
	return &ImportService{DB: db, Config: cfg}
}

// RegisterImportRoutes adds the import endpoint
func (s *ImportService) RegisterImportRoutes(r *gin.Engine) {
	r.POST("/api/import_activity/:id", s.ImportActivityHandler)
}

// ImportActivityHandler fetches activity stream and stores geometry
func (s *ImportService) ImportActivityHandler(c *gin.Context) {
	activityID := c.Param("id")
	userID := c.Query("user_id")
	if activityID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing activity ID or user_id"})
		return
	}

	// Get access token for user
	var token storage.StravaToken
	err := storage.GetTokenByUserID(s.DB, &token, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No token for user"})
		return
	}

	client := resty.New()
	resp, err := client.R().
		SetAuthToken(token.AccessToken).
		SetQueryParam("keys", "latlng"). // Changed from types to keys - this is the correct parameter name
		Get(fmt.Sprintf("https://www.strava.com/api/v3/activities/%s/streams", activityID))
	if err != nil || resp.StatusCode() != 200 {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Failed to fetch activity stream: %v, status: %d", err, resp.StatusCode())})
		return
	}

	// Debug logging
	respBody := resp.Body()
	fmt.Printf("Strava API Response Status: %d\n", resp.StatusCode())
	fmt.Printf("Strava API Response Headers: %v\n", resp.Header())
	fmt.Printf("Strava API Response Body: %s\n", string(respBody))

	// Try parsing as generic JSON first to see the structure
	fmt.Printf("Raw response: %s\n", string(respBody))

	var rawJSON []map[string]interface{}
	if err := json.Unmarshal(respBody, &rawJSON); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse JSON: %v", err)})
		return
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

	if len(latlngData) == 0 {
		fmt.Printf("No valid latlng data found in stream\n")
		fmt.Printf("Raw JSON structure: %+v\n", rawJSON)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid latlng stream found"})
		return
	}

	if len(latlngData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No latlng stream found"})
		return
	}

	// Convert to WKT LINESTRING
	var points []string
	for _, ll := range latlngData {
		if len(ll) == 2 {
			points = append(points, fmt.Sprintf("%f %f", ll[1], ll[0])) // WKT: lon lat
		}
	}
	linestring := fmt.Sprintf("LINESTRING(%s)", strings.Join(points, ", "))

	// Insert into activities
	query := `INSERT INTO activities (
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
		NULL, -- city_id will be updated later
		NULL, -- coverage_percentage will be calculated later
		false,
		CURRENT_TIMESTAMP,
		CURRENT_TIMESTAMP
	)`
	fmt.Printf("Executing query with userID=%s, activityID=%s\n", userID, activityID)
	fmt.Printf("Linestring sample: %.100s...\n", linestring)

	// First verify that the user exists
	var exists bool
	err = s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE strava_id = $1)", userID).Scan(&exists)
	if err != nil {
		fmt.Printf("Database error checking user: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to verify user: %v", err)})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get internal user ID from strava_id
	var userIDInt int
	err = s.DB.QueryRow("SELECT id FROM users WHERE strava_id = $1", userID).Scan(&userIDInt)
	if err != nil {
		fmt.Printf("Database error getting user ID: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user ID: %v", err)})
		return
	}

	// Convert activityID to int64
	var activityIDInt int64
	if _, err := fmt.Sscanf(activityID, "%d", &activityIDInt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid activity ID format: %v", err)})
		return
	}

	// Verify the activity doesn't already exist
	err = s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM activities WHERE strava_activity_id = $1)", activityIDInt).Scan(&exists)
	if err != nil {
		fmt.Printf("Database error checking activity: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to check activity: %v", err)})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Activity already exists"})
		return
	}

	_, err = s.DB.Exec(query, userIDInt, activityIDInt, linestring)
	if err != nil {
		fmt.Printf("Database error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to insert activity: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Activity imported", "linestring": linestring})
}
