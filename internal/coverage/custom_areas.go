package coverage

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// CustomAreasService handles custom drawn areas functionality
type CustomAreasService struct {
	db *storage.DB
}

// NewCustomAreasService creates a new custom areas service
func NewCustomAreasService(db *storage.DB) *CustomAreasService {
	return &CustomAreasService{db: db}
}

// CreateCustomAreaRequest represents the request to create a custom area
type CreateCustomAreaRequest struct {
	Name        string       `json:"name" binding:"required"`
	Coordinates [][2]float64 `json:"coordinates" binding:"required"`
}

// CustomAreaResponse represents the API response for custom areas
type CustomAreaResponse struct {
	ID                 int          `json:"id"`
	UserID             int          `json:"user_id"`
	Name               string       `json:"name"`
	Coordinates        [][2]float64 `json:"coordinates"`
	CoveragePercentage *float64     `json:"coverage_percentage"`
	ActivitiesCount    int          `json:"activities_count"`
	CreatedAt          string       `json:"created_at"`
	UpdatedAt          string       `json:"updated_at"`
}

// RegisterCustomAreaRoutes registers all custom area routes
func (s *CustomAreasService) RegisterCustomAreaRoutes(r *gin.Engine) {
	customAreaGroup := r.Group("/api/custom-areas")
	{
		customAreaGroup.POST("/user/:userId", s.createCustomArea)
		customAreaGroup.GET("/user/:userId", s.getUserCustomAreas)
		customAreaGroup.GET("/:id", s.getCustomArea)
		customAreaGroup.PUT("/:id", s.updateCustomArea)
		customAreaGroup.DELETE("/:id", s.deleteCustomArea)
		customAreaGroup.POST("/:id/calculate-coverage", s.calculateCustomAreaCoverage)
	}
}

// createCustomArea creates a new custom area for a user
func (s *CustomAreasService) createCustomArea(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req CreateCustomAreaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate that we have at least 3 points for a polygon
	if len(req.Coordinates) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Polygon must have at least 3 points"})
		return
	}

	// Convert coordinates to PostGIS polygon format
	polygonWKT := coordinatesToWKT(req.Coordinates)

	// Insert into database
	query := `
		INSERT INTO custom_areas (user_id, name, geometry)
		VALUES ($1, $2, ST_GeomFromText($3, 4326))
		RETURNING id, user_id, name, ST_AsText(geometry) as geometry, 
				  coverage_percentage, activities_count, created_at, updated_at`

	var area storage.CustomArea
	err = s.db.QueryRowx(query, userID, req.Name, polygonWKT).StructScan(&area)
	if err != nil {
		fmt.Printf("Error creating custom area: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create custom area"})
		return
	}

	// Convert to response format
	response := customAreaToResponse(&area)
	c.JSON(http.StatusCreated, response)
}

// getUserCustomAreas gets all custom areas for a user
func (s *CustomAreasService) getUserCustomAreas(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	query := `
		SELECT id, user_id, name, ST_AsText(geometry) as geometry,
			   coverage_percentage, activities_count, created_at, updated_at
		FROM custom_areas 
		WHERE user_id = $1
		ORDER BY created_at DESC`

	var areas []storage.CustomArea
	err = s.db.Select(&areas, query, userID)
	if err != nil {
		fmt.Printf("Error fetching custom areas: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch custom areas"})
		return
	}

	// Convert to response format
	var response []CustomAreaResponse
	for _, area := range areas {
		response = append(response, customAreaToResponse(&area))
	}

	c.JSON(http.StatusOK, response)
}

// getCustomArea gets a specific custom area by ID
func (s *CustomAreasService) getCustomArea(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid area ID"})
		return
	}

	query := `
		SELECT id, user_id, name, ST_AsText(geometry) as geometry,
			   coverage_percentage, activities_count, created_at, updated_at
		FROM custom_areas 
		WHERE id = $1`

	var area storage.CustomArea
	err = s.db.QueryRowx(query, id).StructScan(&area)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Custom area not found"})
		return
	}

	response := customAreaToResponse(&area)
	c.JSON(http.StatusOK, response)
}

// updateCustomArea updates a custom area
func (s *CustomAreasService) updateCustomArea(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid area ID"})
		return
	}

	var req CreateCustomAreaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert coordinates to PostGIS polygon format
	polygonWKT := coordinatesToWKT(req.Coordinates)

	query := `
		UPDATE custom_areas 
		SET name = $2, geometry = ST_GeomFromText($3, 4326), updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, name, ST_AsText(geometry) as geometry,
				  coverage_percentage, activities_count, created_at, updated_at`

	var area storage.CustomArea
	err = s.db.QueryRowx(query, id, req.Name, polygonWKT).StructScan(&area)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update custom area"})
		return
	}

	response := customAreaToResponse(&area)
	c.JSON(http.StatusOK, response)
}

// deleteCustomArea deletes a custom area
func (s *CustomAreasService) deleteCustomArea(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid area ID"})
		return
	}

	query := `DELETE FROM custom_areas WHERE id = $1`
	result, err := s.db.Exec(query, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete custom area"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Custom area not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Custom area deleted successfully"})
}

// calculateCustomAreaCoverage starts coverage calculation in background
func (s *CustomAreasService) calculateCustomAreaCoverage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid area ID"})
		return
	}

	// Get the custom area first
	var area storage.CustomArea
	areaQuery := `
		SELECT id, user_id, name, ST_AsText(geometry) as geometry,
			   coverage_percentage, activities_count, created_at, updated_at
		FROM custom_areas WHERE id = $1`

	err = s.db.QueryRowx(areaQuery, id).StructScan(&area)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Custom area not found"})
		return
	}

	// Start coverage calculation in background
	go s.calculateCoverageAsync(id, area.UserID)

	// Return immediately with current area data
	response := customAreaToResponse(&area)
	c.JSON(http.StatusAccepted, gin.H{
		"message": "Coverage calculation started",
		"area":    response,
	})
}

// calculateCoverageAsync performs the actual coverage calculation in background
func (s *CustomAreasService) calculateCoverageAsync(areaID, userID int) {
	fmt.Printf("Starting coverage calculation for area %d, user %d\n", areaID, userID)

	// Most complex and detailed grid-based coverage calculation
	// High-resolution grid with multiple coverage layers and detailed analysis
	coverageQuery := `
		WITH area_geometry AS (
			SELECT geometry, ST_Area(ST_Transform(geometry, 3857)) as area_sqm FROM custom_areas WHERE id = $2
		),
		intersecting_activities AS (
			SELECT DISTINCT a.id, a.path, a.distance_km, a.activity_type
			FROM activities a, area_geometry ag
			WHERE a.user_id = $1
			AND ST_Intersects(a.path, ag.geometry)
		),
		area_bounds AS (
			SELECT 
				ST_XMin(geometry) as min_x, ST_YMin(geometry) as min_y,
				ST_XMax(geometry) as max_x, ST_YMax(geometry) as max_y,
				geometry, area_sqm
			FROM area_geometry
		),
		-- High resolution grid: 0.0005 degrees (~50m spacing)
		fine_grid AS (
			SELECT 
				ST_SetSRID(ST_MakePoint(
					ab.min_x + (x::float * 0.0005), 
					ab.min_y + (y::float * 0.0005)
				), 4326) as point,
				x, y
			FROM area_bounds ab,
			generate_series(0, ((ab.max_x - ab.min_x) / 0.0005)::integer) as x,
			generate_series(0, ((ab.max_y - ab.min_y) / 0.0005)::integer) as y
			WHERE ST_Contains(ab.geometry, ST_SetSRID(ST_MakePoint(
				ab.min_x + (x::float * 0.0005), 
				ab.min_y + (y::float * 0.0005)
			), 4326))
		),
		-- Multiple coverage layers with different buffer distances
		coverage_layers AS (
			SELECT 
				fg.point,
				fg.x, fg.y,
				-- Direct coverage (25m buffer)
				CASE WHEN EXISTS (
					SELECT 1 FROM intersecting_activities ia
					WHERE ST_DWithin(ST_Transform(ia.path, 3857), ST_Transform(fg.point, 3857), 25)
				) THEN 3 ELSE 0 END as direct_coverage,
				
				-- Close coverage (50m buffer) 
				CASE WHEN EXISTS (
					SELECT 1 FROM intersecting_activities ia
					WHERE ST_DWithin(ST_Transform(ia.path, 3857), ST_Transform(fg.point, 3857), 50)
				) THEN 2 ELSE 0 END as close_coverage,
				
				-- Moderate coverage (100m buffer)
				CASE WHEN EXISTS (
					SELECT 1 FROM intersecting_activities ia
					WHERE ST_DWithin(ST_Transform(ia.path, 3857), ST_Transform(fg.point, 3857), 100)
				) THEN 1 ELSE 0 END as moderate_coverage,
				
				-- Activity type diversity
				(SELECT COUNT(DISTINCT ia.activity_type) 
				 FROM intersecting_activities ia
				 WHERE ST_DWithin(ST_Transform(ia.path, 3857), ST_Transform(fg.point, 3857), 100)
				) as activity_types_count
			FROM fine_grid fg
		),
		-- Weighted coverage calculation
		coverage_stats AS (
			SELECT 
				COUNT(*) as total_points,
				SUM(CASE WHEN direct_coverage > 0 THEN 1 ELSE 0 END) as direct_covered_points,
				SUM(CASE WHEN close_coverage > 0 THEN 1 ELSE 0 END) as close_covered_points,  
				SUM(CASE WHEN moderate_coverage > 0 THEN 1 ELSE 0 END) as moderate_covered_points,
				-- Weighted coverage score (direct=3, close=2, moderate=1)
				SUM(GREATEST(direct_coverage, close_coverage, moderate_coverage)) as weighted_coverage_score,
				AVG(activity_types_count) as avg_activity_diversity,
				MAX(activity_types_count) as max_activity_diversity
			FROM coverage_layers
		),
		-- Spatial clustering analysis
		cluster_analysis AS (
			SELECT 
				COUNT(CASE WHEN cl.direct_coverage > 0 THEN 1 END) as covered_clusters,
				-- Calculate coverage density (covered points per sq km)
				COUNT(CASE WHEN cl.direct_coverage > 0 THEN 1 END)::float / 
				 (ab.area_sqm / 1000000.0) as coverage_density_per_sqkm
			FROM coverage_layers cl
			CROSS JOIN area_bounds ab
		)
		SELECT 
			-- Primary coverage percentage (direct coverage)
			CASE WHEN cs.total_points > 0 
				THEN LEAST((cs.direct_covered_points::float / cs.total_points::float) * 100, 100)
				ELSE 0 
			END as coverage_percentage,
			
			-- Additional metrics
			(SELECT COUNT(*) FROM intersecting_activities) as activities_count,
			cs.total_points as grid_points_total,
			cs.direct_covered_points as direct_covered_points,
			cs.close_covered_points as close_covered_points,
			cs.moderate_covered_points as moderate_covered_points,
			
			-- Weighted coverage score (0-300 scale, normalize to 0-100)
			CASE WHEN cs.total_points > 0
				THEN LEAST((cs.weighted_coverage_score::float / (cs.total_points::float * 3)) * 100, 100)
				ELSE 0
			END as weighted_coverage_percentage,
			
			cs.avg_activity_diversity,
			cs.max_activity_diversity,
			ca.coverage_density_per_sqkm
		FROM coverage_stats cs, cluster_analysis ca`

	var coverage struct {
		CoveragePercentage         float64 `db:"coverage_percentage"`
		ActivitiesCount            int     `db:"activities_count"`
		GridPointsTotal            int     `db:"grid_points_total"`
		DirectCoveredPoints        int     `db:"direct_covered_points"`
		CloseCoveredPoints         int     `db:"close_covered_points"`
		ModerateCoveredPoints      int     `db:"moderate_covered_points"`
		WeightedCoveragePercentage float64 `db:"weighted_coverage_percentage"`
		AvgActivityDiversity       float64 `db:"avg_activity_diversity"`
		MaxActivityDiversity       int     `db:"max_activity_diversity"`
		CoverageDensityPerSqkm     float64 `db:"coverage_density_per_sqkm"`
	}

	err := s.db.QueryRowx(coverageQuery, userID, areaID).StructScan(&coverage)
	if err != nil {
		fmt.Printf("Error calculating coverage for area %d: %v\n", areaID, err)
		return
	}

	// Update the area with the calculated coverage
	updateQuery := `
		UPDATE custom_areas 
		SET coverage_percentage = $2, activities_count = $3, updated_at = NOW()
		WHERE id = $1`

	_, err = s.db.Exec(updateQuery, areaID, coverage.CoveragePercentage, coverage.ActivitiesCount)
	if err != nil {
		fmt.Printf("Error updating coverage for area %d: %v\n", areaID, err)
		return
	}

	fmt.Printf("Coverage calculation completed for area %d: %.2f%% (%d activities)\n",
		areaID, coverage.CoveragePercentage, coverage.ActivitiesCount)
	fmt.Printf("  Grid analysis: %d total points, %d direct covered, %d close covered, %d moderate covered\n",
		coverage.GridPointsTotal, coverage.DirectCoveredPoints, coverage.CloseCoveredPoints, coverage.ModerateCoveredPoints)
	fmt.Printf("  Weighted coverage: %.2f%%, Activity diversity: %.2f avg / %d max, Density: %.2f per sq km\n",
		coverage.WeightedCoveragePercentage, coverage.AvgActivityDiversity, coverage.MaxActivityDiversity, coverage.CoverageDensityPerSqkm)
}

// Helper function to convert coordinates to WKT polygon format
func coordinatesToWKT(coords [][2]float64) string {
	if len(coords) == 0 {
		return ""
	}

	var points []string
	for _, coord := range coords {
		points = append(points, fmt.Sprintf("%f %f", coord[1], coord[0])) // Note: WKT uses lon lat order
	}

	// Ensure polygon is closed (first point = last point)
	if coords[0][0] != coords[len(coords)-1][0] || coords[0][1] != coords[len(coords)-1][1] {
		points = append(points, fmt.Sprintf("%f %f", coords[0][1], coords[0][0]))
	}

	return fmt.Sprintf("POLYGON((%s))", strings.Join(points, ","))
}

// Helper function to convert WKT to coordinates
func wktToCoordinates(wkt string) ([][2]float64, error) {
	// Simple parser for "POLYGON((lon lat,lon lat,...))" format
	if !strings.HasPrefix(wkt, "POLYGON((") || !strings.HasSuffix(wkt, "))") {
		return nil, fmt.Errorf("invalid polygon WKT format")
	}

	// Extract coordinates string
	coordsStr := wkt[9 : len(wkt)-2] // Remove "POLYGON((" and "))"
	pointStrs := strings.Split(coordsStr, ",")

	var coordinates [][2]float64
	for _, pointStr := range pointStrs {
		pointStr = strings.TrimSpace(pointStr)
		if pointStr == "" {
			continue
		}

		parts := strings.Fields(pointStr)
		if len(parts) != 2 {
			continue
		}

		lon, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			continue
		}
		lat, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}

		coordinates = append(coordinates, [2]float64{lat, lon}) // Note: Convert back to lat lng order for frontend
	}

	return coordinates, nil
}

// Helper function to convert CustomArea to response format
func customAreaToResponse(area *storage.CustomArea) CustomAreaResponse {
	coords, _ := wktToCoordinates(area.Geometry)

	return CustomAreaResponse{
		ID:                 area.ID,
		UserID:             area.UserID,
		Name:               area.Name,
		Coordinates:        coords,
		CoveragePercentage: area.CoveragePercentage,
		ActivitiesCount:    area.ActivitiesCount,
		CreatedAt:          area.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          area.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
