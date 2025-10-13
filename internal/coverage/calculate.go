package coverage

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
	"github.com/nikhilvedi/strava-coverage/internal/utils"
)

// CoverageService handles coverage calculation operations
type CoverageService struct {
	DB *storage.DB
}

// NewCoverageService creates a new coverage service
func NewCoverageService(db *storage.DB) *CoverageService {
	return &CoverageService{DB: db}
}

// RegisterCoverageRoutes adds coverage calculation endpoints
func (s *CoverageService) RegisterCoverageRoutes(r *gin.Engine) {
	coverage := r.Group("/api/coverage")
	{
		coverage.POST("/calculate/:activityId", s.CalculateCoverageHandler)
		coverage.GET("/user/:userId/city/:cityId", s.GetUserCityCoverageHandler)
		coverage.GET("/activity/:activityId", s.GetActivityCoverageHandler)
	}
}

// CoverageResult represents the result of a coverage calculation
type CoverageResult struct {
	ActivityID      int64   `json:"activity_id"`
	CityID          int     `json:"city_id"`
	CityName        string  `json:"city_name"`
	CoveragePercent float64 `json:"coverage_percent"`
	NewStreetsKm    float64 `json:"new_streets_km"`
	TotalStreetsKm  float64 `json:"total_streets_km"`
	UniqueStreetsKm float64 `json:"unique_streets_km"`
}

// CalculateCoverageHandler calculates coverage for a specific activity
func (s *CoverageService) CalculateCoverageHandler(c *gin.Context) {
	logger := utils.NewLogger("CoverageService")

	activityIDStr := c.Param("activityId")
	activityID, err := strconv.ParseInt(activityIDStr, 10, 64)
	if err != nil {
		apiErr := utils.NewAPIError(400, "Invalid activity ID", "Activity ID must be a valid integer")
		utils.ErrorResponse(c, apiErr)
		return
	}

	logger.Info("Calculating coverage for activity %d", activityID)

	// First, check if the activity exists and get its details
	var userID int
	var activityPath []byte
	query := `SELECT user_id, ST_AsBinary(path) FROM activities WHERE strava_activity_id = $1`
	err = s.DB.QueryRow(query, activityID).Scan(&userID, &activityPath)
	if err != nil {
		if err == sql.ErrNoRows {
			apiErr := utils.NewAPIError(404, "Activity not found", fmt.Sprintf("No activity found with ID %d", activityID))
			utils.ErrorResponse(c, apiErr)
		} else {
			logger.Error("Failed to fetch activity %d: %v", activityID, err)
			apiErr := utils.NewAPIError(500, "Database error", "Failed to retrieve activity data")
			utils.ErrorResponse(c, apiErr)
		}
		return
	}

	// Find which city this activity intersects with
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
		if err == sql.ErrNoRows {
			apiErr := utils.NewAPIError(404, "No city intersection", "Activity does not intersect with any tracked city")
			utils.ErrorResponse(c, apiErr)
		} else {
			logger.Error("Failed to find intersecting city for activity %d: %v", activityID, err)
			apiErr := utils.NewAPIError(500, "Database error", "Failed to find intersecting city")
			utils.ErrorResponse(c, apiErr)
		}
		return
	}

	logger.Info("Activity %d intersects with city %s (ID: %d)", activityID, cityName, cityID)

	// Calculate coverage using a grid-based approach
	result, err := s.calculateGridBasedCoverage(userID, activityID, cityID, cityName)
	if err != nil {
		logger.Error("Failed to calculate coverage for activity %d: %v", activityID, err)
		apiErr := utils.NewAPIError(500, "Coverage calculation failed", "Unable to calculate street coverage")
		utils.ErrorResponse(c, apiErr)
		return
	}

	// Update the activity with the calculated coverage
	updateQuery := `
		UPDATE activities 
		SET city_id = $1, coverage_percentage = $2, updated_at = CURRENT_TIMESTAMP
		WHERE strava_activity_id = $3`

	_, err = s.DB.Exec(updateQuery, cityID, result.CoveragePercent, activityID)
	if err != nil {
		fmt.Printf("Warning: Failed to update activity coverage: %v\n", err)
	}

	c.JSON(http.StatusOK, result)
}

// calculateGridBasedCoverage calculates coverage using a simplified distance-based approach
func (s *CoverageService) calculateGridBasedCoverage(userID int, activityID int64, cityID int, cityName string) (*CoverageResult, error) {
	// Simplified coverage calculation based on total distance covered within the city
	// This is much faster than grid-based approach

	coverageQuery := `
		WITH 
		-- Get city area for normalization
		city_info AS (
			SELECT 
				ST_Area(ST_Transform(boundary, 3857)) / 1000000 as area_km2
			FROM cities WHERE id = $1
		),
		-- Calculate total distance of user activities in this city
		user_distance AS (
			SELECT 
				COALESCE(SUM(ST_Length(ST_Transform(ST_Intersection(a.path, c.boundary), 3857)) / 1000), 0) as total_km
			FROM activities a
			JOIN cities c ON c.id = $2
			WHERE a.user_id = $3 AND a.city_id = $2
		),
		-- Estimate total "explorable" distance in city (rough approximation)
		city_explorable AS (
			SELECT 
				-- Rough estimate: 10-15 km of roads per km² in urban areas
				area_km2 * 12 as estimated_roads_km
			FROM city_info
		)
		SELECT 
			ce.estimated_roads_km as total_streets_km,
			ud.total_km as covered_streets_km,
			CASE 
				WHEN ce.estimated_roads_km > 0 THEN 
					LEAST((ud.total_km / ce.estimated_roads_km) * 100, 100)
				ELSE 0 
			END as coverage_percentage
		FROM city_explorable ce, user_distance ud`

	var totalStreetsKm, coveredStreetsKm, coveragePercent float64

	err := s.DB.QueryRow(coverageQuery, cityID, cityID, userID).Scan(
		&totalStreetsKm, &coveredStreetsKm, &coveragePercent)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate coverage: %v", err)
	}

	result := &CoverageResult{
		ActivityID:      activityID,
		CityID:          cityID,
		CityName:        cityName,
		CoveragePercent: coveragePercent,
		NewStreetsKm:    0, // Not calculated in simplified approach
		TotalStreetsKm:  totalStreetsKm,
		UniqueStreetsKm: coveredStreetsKm,
	}

	return result, nil
}

// GetUserCityCoverageHandler returns overall coverage for a user in a specific city
func (s *CoverageService) GetUserCityCoverageHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	cityIDStr := c.Param("cityId")

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	cityID, err := strconv.Atoi(cityIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid city ID"})
		return
	}

	// Super simplified approach - just calculate based on activity distance vs city size
	query := `
		SELECT 
			c.name,
			ST_Area(ST_Transform(c.boundary, 3857)) / 1000000 * 12 as estimated_roads_km,
			COALESCE(SUM(ST_Length(ST_Transform(a.path, 3857)) / 1000), 0) as covered_km
		FROM cities c
		LEFT JOIN activities a ON a.city_id = c.id AND a.user_id = $2
		WHERE c.id = $1
		GROUP BY c.id, c.name, c.boundary`

	var cityName string
	var totalStreetsKm, coveredStreetsKm float64

	err = s.DB.QueryRow(query, cityID, userID).Scan(&cityName, &totalStreetsKm, &coveredStreetsKm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate coverage"})
		return
	}

	// Calculate coverage percentage
	coveragePercent := float64(0)
	if totalStreetsKm > 0 {
		coveragePercent = (coveredStreetsKm / totalStreetsKm) * 100
		if coveragePercent > 100 {
			coveragePercent = 100
		}
	}

	result := map[string]interface{}{
		"user_id":            userID,
		"city_id":            cityID,
		"city_name":          cityName,
		"coverage_percent":   coveragePercent,
		"total_streets_km":   totalStreetsKm,
		"covered_streets_km": coveredStreetsKm,
	}

	c.JSON(http.StatusOK, result)
}

// GetActivityCoverageHandler returns coverage information for a specific activity
func (s *CoverageService) GetActivityCoverageHandler(c *gin.Context) {
	activityIDStr := c.Param("activityId")
	activityID, err := strconv.ParseInt(activityIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid activity ID"})
		return
	}

	query := `
		SELECT 
			a.strava_activity_id,
			a.city_id,
			c.name as city_name,
			a.coverage_percentage
		FROM activities a
		LEFT JOIN cities c ON a.city_id = c.id
		WHERE a.strava_activity_id = $1`

	var cityID sql.NullInt64
	var cityName sql.NullString
	var coveragePercent sql.NullFloat64

	err = s.DB.QueryRow(query, activityID).Scan(&activityID, &cityID, &cityName, &coveragePercent)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Activity not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity coverage"})
		}
		return
	}

	result := map[string]interface{}{
		"activity_id": activityID,
	}

	if cityID.Valid {
		result["city_id"] = cityID.Int64
	}
	if cityName.Valid {
		result["city_name"] = cityName.String
	}
	if coveragePercent.Valid {
		result["coverage_percent"] = coveragePercent.Float64
	}

	c.JSON(http.StatusOK, result)
}
