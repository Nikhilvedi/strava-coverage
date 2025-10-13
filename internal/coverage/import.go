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
		SetQueryParam("types", "latlng").
		Get(fmt.Sprintf("https://www.strava.com/api/v3/activities/%s/streams", activityID))
	if err != nil || resp.StatusCode() != 200 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch activity stream"})
		return
	}

	var streams []struct {
		Type string      `json:"type"`
		Data [][]float64 `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &streams); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid stream response"})
		return
	}

	var latlngs [][]float64
	for _, s := range streams {
		if s.Type == "latlng" {
			latlngs = s.Data
			break
		}
	}
	if len(latlngs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No latlng stream found"})
		return
	}

	// Convert to WKT LINESTRING
	var points []string
	for _, ll := range latlngs {
		if len(ll) == 2 {
			points = append(points, fmt.Sprintf("%f %f", ll[1], ll[0])) // WKT: lon lat
		}
	}
	linestring := fmt.Sprintf("LINESTRING(%s)", strings.Join(points, ", "))

	// Insert into activities
	query := `INSERT INTO activities (user_id, strava_activity_id, path) VALUES ($1, $2, ST_GeomFromText($3, 4326))`
	_, err = s.DB.Exec(query, userID, activityID, linestring)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert activity"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Activity imported", "linestring": linestring})
}
