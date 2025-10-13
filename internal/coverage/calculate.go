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

// calculateGridBasedCoverage calculates coverage using a simplified grid approach
func (s *CoverageService) calculateGridBasedCoverage(userID int, activityID int64, cityID int, cityName string) (*CoverageResult, error) {
	// Create a 100m x 100m grid within the city boundary and calculate coverage
	// This is a simplified approach that doesn't require OSM street data

	coverageQuery := `
		WITH 
		-- Create a grid of cells within the city boundary
		city_bounds AS (
			SELECT boundary FROM cities WHERE id = $1
		),
		grid AS (
			SELECT 
				ST_SetSRID(ST_MakePoint(x, y), 4326) as center,
				ST_Buffer(ST_SetSRID(ST_MakePoint(x, y), 4326)::geography, 50)::geometry as cell
			FROM city_bounds cb,
			generate_series(
				ST_XMin(cb.boundary)::numeric, 
				ST_XMax(cb.boundary)::numeric, 
				0.001  -- ~100m grid spacing
			) as x,
			generate_series(
				ST_YMin(cb.boundary)::numeric, 
				ST_YMax(cb.boundary)::numeric, 
				0.001  -- ~100m grid spacing  
			) as y
			WHERE ST_Within(ST_SetSRID(ST_MakePoint(x, y), 4326), cb.boundary)
		),
		-- Find grid cells covered by current activity
		current_activity_coverage AS (
			SELECT DISTINCT g.center
			FROM grid g, activities a
			WHERE a.strava_activity_id = $2
			AND ST_DWithin(g.center::geography, a.path::geography, 50)
		),
		-- Find all grid cells covered by user's activities in this city
		user_total_coverage AS (
			SELECT DISTINCT g.center
			FROM grid g, activities a
			WHERE a.user_id = $3
			AND a.city_id = $4
			AND ST_DWithin(g.center::geography, a.path::geography, 50)
		),
		-- Calculate statistics
		stats AS (
			SELECT 
				COUNT(*) as total_grid_cells,
				COUNT(CASE WHEN utc.center IS NOT NULL THEN 1 END) as covered_cells,
				COUNT(CASE WHEN cac.center IS NOT NULL THEN 1 END) as new_cells
			FROM grid g
			LEFT JOIN user_total_coverage utc ON ST_Equals(g.center, utc.center)
			LEFT JOIN current_activity_coverage cac ON ST_Equals(g.center, cac.center)
		)
		SELECT 
			total_grid_cells,
			covered_cells,
			new_cells,
			CASE 
				WHEN total_grid_cells > 0 THEN (covered_cells::float / total_grid_cells::float) * 100 
				ELSE 0 
			END as coverage_percentage
		FROM stats`

	var totalCells, coveredCells, newCells int
	var coveragePercent float64

	err := s.DB.QueryRow(coverageQuery, cityID, activityID, userID, cityID).Scan(
		&totalCells, &coveredCells, &newCells, &coveragePercent)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate grid coverage: %v", err)
	}

	// Convert cells to approximate kilometers (each cell represents ~100m x 100m = 0.01 km²)
	cellSizeKm := 0.1 // Each cell represents ~100m of "street"

	result := &CoverageResult{
		ActivityID:      activityID,
		CityID:          cityID,
		CityName:        cityName,
		CoveragePercent: coveragePercent,
		NewStreetsKm:    float64(newCells) * cellSizeKm,
		TotalStreetsKm:  float64(totalCells) * cellSizeKm,
		UniqueStreetsKm: float64(coveredCells) * cellSizeKm,
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

	// Get city name and calculate total coverage
	query := `
		WITH 
		city_bounds AS (
			SELECT name, boundary FROM cities WHERE id = $1
		),
		grid AS (
			SELECT 
				ST_SetSRID(ST_MakePoint(x, y), 4326) as center
			FROM city_bounds cb,
			generate_series(
				ST_XMin(cb.boundary)::numeric, 
				ST_XMax(cb.boundary)::numeric, 
				0.001
			) as x,
			generate_series(
				ST_YMin(cb.boundary)::numeric, 
				ST_YMax(cb.boundary)::numeric, 
				0.001
			) as y
			WHERE ST_Within(ST_SetSRID(ST_MakePoint(x, y), 4326), cb.boundary)
		),
		user_coverage AS (
			SELECT DISTINCT g.center
			FROM grid g, activities a
			WHERE a.user_id = $2
			AND a.city_id = $1
			AND ST_DWithin(g.center::geography, a.path::geography, 50)
		),
		stats AS (
			SELECT 
				cb.name,
				COUNT(g.center) as total_cells,
				COUNT(uc.center) as covered_cells
			FROM city_bounds cb
			CROSS JOIN grid g
			LEFT JOIN user_coverage uc ON ST_Equals(g.center, uc.center)
			GROUP BY cb.name
		)
		SELECT 
			name,
			total_cells,
			covered_cells,
			CASE 
				WHEN total_cells > 0 THEN (covered_cells::float / total_cells::float) * 100 
				ELSE 0 
			END as coverage_percentage
		FROM stats`

	var cityName string
	var totalCells, coveredCells int
	var coveragePercent float64

	err = s.DB.QueryRow(query, cityID, userID).Scan(&cityName, &totalCells, &coveredCells, &coveragePercent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate coverage"})
		return
	}

	cellSizeKm := 0.1
	result := map[string]interface{}{
		"user_id":            userID,
		"city_id":            cityID,
		"city_name":          cityName,
		"coverage_percent":   coveragePercent,
		"total_streets_km":   float64(totalCells) * cellSizeKm,
		"covered_streets_km": float64(coveredCells) * cellSizeKm,
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
