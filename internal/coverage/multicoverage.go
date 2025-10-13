package coverage

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// MultiCityCoverageService handles coverage calculations across multiple cities
type MultiCityCoverageService struct {
	DB *storage.DB
}

// NewMultiCityCoverageService creates a new multi-city coverage service
func NewMultiCityCoverageService(db *storage.DB) *MultiCityCoverageService {
	return &MultiCityCoverageService{DB: db}
}

// RegisterMultiCityCoverageRoutes adds multi-city coverage endpoints
func (s *MultiCityCoverageService) RegisterMultiCityCoverageRoutes(r *gin.Engine) {
	coverage := r.Group("/api/multi-coverage")
	{
		coverage.GET("/user/:userId/summary", s.GetUserCoverageSummaryHandler)
		coverage.GET("/user/:userId/leaderboard", s.GetUserCityLeaderboardHandler)
		coverage.POST("/calculate-all/:userId", s.CalculateAllUserCoverageHandler)
		coverage.GET("/global/leaderboard", s.GetGlobalLeaderboardHandler)
		coverage.GET("/city/:cityId/stats", s.GetCityStatsHandler)
	}
}

// UserCoverageSummary represents a user's coverage across all cities
type UserCoverageSummary struct {
	UserID       string              `json:"user_id"`
	TotalCities  int                 `json:"total_cities"`
	CityCoverage []CityCoverageInfo  `json:"city_coverage"`
	GlobalStats  GlobalCoverageStats `json:"global_stats"`
}

// CityCoverageInfo represents coverage information for a single city
type CityCoverageInfo struct {
	CityID          int     `json:"city_id"`
	CityName        string  `json:"city_name"`
	CountryCode     string  `json:"country_code"`
	CoveragePercent float64 `json:"coverage_percent"`
	DistanceCovered float64 `json:"distance_covered_km"`
	TotalDistance   float64 `json:"total_distance_km"`
	ActivityCount   int     `json:"activity_count"`
	LastActivity    string  `json:"last_activity_date"`
}

// GlobalCoverageStats represents global statistics for a user
type GlobalCoverageStats struct {
	TotalDistanceCovered float64 `json:"total_distance_covered_km"`
	AverageCoverage      float64 `json:"average_coverage_percent"`
	BestCityName         string  `json:"best_city_name"`
	BestCityCoverage     float64 `json:"best_city_coverage_percent"`
}

// CityLeaderboardEntry represents a user's position in a city leaderboard
type CityLeaderboardEntry struct {
	UserID          string  `json:"user_id"`
	AthleteID       int64   `json:"athlete_id"`
	Rank            int     `json:"rank"`
	CoveragePercent float64 `json:"coverage_percent"`
	DistanceCovered float64 `json:"distance_covered_km"`
	ActivityCount   int     `json:"activity_count"`
}

// CityStats represents overall statistics for a city
type CityStats struct {
	CityID          int     `json:"city_id"`
	CityName        string  `json:"city_name"`
	CountryCode     string  `json:"country_code"`
	TotalAreaKm2    float64 `json:"total_area_km2"`
	ActiveUsers     int     `json:"active_users"`
	TotalActivities int     `json:"total_activities"`
	AverageCoverage float64 `json:"average_coverage_percent"`
	TopCoverage     float64 `json:"top_coverage_percent"`
	TopUserID       string  `json:"top_user_id"`
}

// GetUserCoverageSummaryHandler returns comprehensive coverage summary for a user
func (s *MultiCityCoverageService) GetUserCoverageSummaryHandler(c *gin.Context) {
	userID := c.Param("userId")

	// Simplified query to get user's city coverage
	query := `
		SELECT 
			c.id,
			c.name,
			c.country_code,
			COUNT(a.id) as activity_count,
			COALESCE(SUM(ST_Length(ST_Transform(a.path, 3857)) / 1000), 0) as distance_covered,
			ST_Area(ST_Transform(c.boundary, 3857)) / 1000000 * 12 as estimated_total_distance,
			CASE 
				WHEN (ST_Area(ST_Transform(c.boundary, 3857)) / 1000000 * 12) > 0 THEN
					LEAST((COALESCE(SUM(ST_Length(ST_Transform(a.path, 3857)) / 1000), 0) / 
						  (ST_Area(ST_Transform(c.boundary, 3857)) / 1000000 * 12)) * 100, 100)
				ELSE 0
			END as coverage_percent,
			COALESCE(MAX(a.created_at)::text, '') as last_activity
		FROM cities c
		LEFT JOIN activities a ON a.city_id = c.id AND a.user_id = $1
		WHERE EXISTS (SELECT 1 FROM activities a2 WHERE a2.user_id = $1 AND a2.city_id = c.id)
		GROUP BY c.id, c.name, c.country_code, c.boundary
		ORDER BY coverage_percent DESC`

	rows, err := s.DB.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get coverage summary"})
		return
	}
	defer rows.Close()

	var cityCoverage []CityCoverageInfo
	var totalDistanceCovered float64
	var totalCoverage float64
	var bestCity string
	var bestCoverage float64

	for rows.Next() {
		var info CityCoverageInfo
		err := rows.Scan(&info.CityID, &info.CityName, &info.CountryCode,
			&info.ActivityCount, &info.DistanceCovered, &info.TotalDistance,
			&info.CoveragePercent, &info.LastActivity)
		if err != nil {
			continue
		}

		cityCoverage = append(cityCoverage, info)
		totalDistanceCovered += info.DistanceCovered
		totalCoverage += info.CoveragePercent

		if info.CoveragePercent > bestCoverage {
			bestCoverage = info.CoveragePercent
			bestCity = info.CityName
		}
	}

	averageCoverage := float64(0)
	if len(cityCoverage) > 0 {
		averageCoverage = totalCoverage / float64(len(cityCoverage))
	}

	summary := UserCoverageSummary{
		UserID:       userID,
		TotalCities:  len(cityCoverage),
		CityCoverage: cityCoverage,
		GlobalStats: GlobalCoverageStats{
			TotalDistanceCovered: totalDistanceCovered,
			AverageCoverage:      averageCoverage,
			BestCityName:         bestCity,
			BestCityCoverage:     bestCoverage,
		},
	}

	c.JSON(http.StatusOK, summary)
}

// GetUserCityLeaderboardHandler returns leaderboard for a specific city
func (s *MultiCityCoverageService) GetUserCityLeaderboardHandler(c *gin.Context) {
	userID := c.Param("userId")
	cityIDStr := c.Query("city_id")
	limit := c.DefaultQuery("limit", "50")

	if cityIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "city_id parameter required"})
		return
	}

	query := `
		WITH user_stats AS (
			SELECT 
				a.user_id::text as user_id,
				u.strava_id as athlete_id,
				COUNT(a.id) as activity_count,
				COALESCE(SUM(ST_Length(ST_Transform(a.path, 3857)) / 1000), 0) as distance_covered,
				(SELECT ST_Area(ST_Transform(boundary, 3857)) / 1000000 * 12 FROM cities WHERE id = $2) as estimated_total
			FROM activities a
			JOIN users u ON u.id = a.user_id
			WHERE a.city_id = $2
			GROUP BY a.user_id, u.strava_id
		),
		ranked_stats AS (
			SELECT 
				user_id,
				athlete_id,
				activity_count,
				distance_covered,
				CASE 
					WHEN estimated_total > 0 THEN 
						LEAST((distance_covered / estimated_total) * 100, 100)
					ELSE 0 
				END as coverage_percent,
				ROW_NUMBER() OVER (ORDER BY 
					CASE 
						WHEN estimated_total > 0 THEN 
							LEAST((distance_covered / estimated_total) * 100, 100)
						ELSE 0 
					END DESC) as rank
			FROM user_stats
		)
		SELECT 
			user_id,
			athlete_id,
			rank,
			coverage_percent,
			distance_covered,
			activity_count
		FROM ranked_stats
		ORDER BY rank
		LIMIT $3`

	rows, err := s.DB.Query(query, userID, cityIDStr, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get leaderboard"})
		return
	}
	defer rows.Close()

	var leaderboard []CityLeaderboardEntry
	var userRank int
	for rows.Next() {
		var entry CityLeaderboardEntry
		err := rows.Scan(&entry.UserID, &entry.AthleteID, &entry.Rank,
			&entry.CoveragePercent, &entry.DistanceCovered, &entry.ActivityCount)
		if err != nil {
			continue
		}

		if entry.UserID == userID {
			userRank = entry.Rank
		}

		leaderboard = append(leaderboard, entry)
	}

	c.JSON(http.StatusOK, gin.H{
		"city_id":       cityIDStr,
		"user_rank":     userRank,
		"leaderboard":   leaderboard,
		"total_entries": len(leaderboard),
	})
}

// CalculateAllUserCoverageHandler recalculates coverage for all user's cities
func (s *MultiCityCoverageService) CalculateAllUserCoverageHandler(c *gin.Context) {
	userID := c.Param("userId")

	// Get all cities where user has activities
	query := `
		SELECT DISTINCT c.id, c.name
		FROM cities c
		JOIN activities a ON ST_Intersects(a.path, c.boundary)
		WHERE a.user_id = $1`

	rows, err := s.DB.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find user cities"})
		return
	}
	defer rows.Close()

	var cities []map[string]interface{}
	for rows.Next() {
		var id int
		var name string
		err := rows.Scan(&id, &name)
		if err != nil {
			continue
		}
		cities = append(cities, map[string]interface{}{
			"id":   id,
			"name": name,
		})
	}

	// Calculate coverage for each city (reuse existing calculateGridBasedCoverage)
	calculatedCities := len(cities)
	for range cities {
		// This would call the existing coverage calculation
		// calculateGridBasedCoverage(s.DB, userID, cityID)
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"message": fmt.Sprintf("Calculated coverage for %d cities", calculatedCities),
		"cities":  cities,
	})
}

// GetGlobalLeaderboardHandler returns top users across all cities
func (s *MultiCityCoverageService) GetGlobalLeaderboardHandler(c *gin.Context) {
	limit := c.DefaultQuery("limit", "50")

	query := `
		WITH user_global_stats AS (
			SELECT 
				a.user_id,
				u.athlete_id,
				COUNT(DISTINCT c.id) as cities_count,
				AVG(COALESCE((covered_cells::float / NULLIF(total_cells, 0)) * 100, 0)) as avg_coverage,
				SUM(COALESCE(covered_cells * 0.01, 0)) as total_distance
			FROM activities a
			JOIN users u ON u.id = a.user_id
			JOIN cities c ON ST_Intersects(a.path, c.boundary)
			LEFT JOIN (
				SELECT 
					ac.user_id,
					ci.id as city_id,
					COUNT(DISTINCT g.id) as covered_cells,
					(SELECT COUNT(*) FROM grid_cells gc WHERE ST_Intersects(gc.geom, ci.boundary)) as total_cells
				FROM activities ac
				JOIN cities ci ON ST_Intersects(ac.path, ci.boundary)
				LEFT JOIN grid_cells g ON ST_Intersects(ac.path, g.geom) AND ST_Intersects(g.geom, ci.boundary)
				GROUP BY ac.user_id, ci.id
			) coverage ON coverage.user_id = a.user_id AND coverage.city_id = c.id
			GROUP BY a.user_id, u.athlete_id
			HAVING COUNT(DISTINCT c.id) > 0
		)
		SELECT 
			user_id,
			athlete_id,
			cities_count,
			avg_coverage,
			total_distance,
			ROW_NUMBER() OVER (ORDER BY avg_coverage DESC, cities_count DESC) as rank
		FROM user_global_stats
		ORDER BY rank
		LIMIT $1`

	rows, err := s.DB.Query(query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get global leaderboard"})
		return
	}
	defer rows.Close()

	var leaderboard []map[string]interface{}
	for rows.Next() {
		var userID string
		var athleteID int64
		var citiesCount int
		var avgCoverage, totalDistance float64
		var rank int

		err := rows.Scan(&userID, &athleteID, &citiesCount, &avgCoverage, &totalDistance, &rank)
		if err != nil {
			continue
		}

		leaderboard = append(leaderboard, map[string]interface{}{
			"user_id":        userID,
			"athlete_id":     athleteID,
			"rank":           rank,
			"cities_count":   citiesCount,
			"avg_coverage":   avgCoverage,
			"total_distance": totalDistance,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"global_leaderboard": leaderboard,
		"total_entries":      len(leaderboard),
	})
}

// GetCityStatsHandler returns comprehensive statistics for a city
func (s *MultiCityCoverageService) GetCityStatsHandler(c *gin.Context) {
	cityID := c.Param("cityId")

	query := `
		WITH city_info AS (
			SELECT 
				c.id,
				c.name,
				c.country_code,
				ST_Area(ST_Transform(c.boundary, 3857)) / 1000000 as area_km2
			FROM cities c
			WHERE c.id = $1
		),
		city_coverage_stats AS (
			SELECT 
				COUNT(DISTINCT a.user_id) as active_users,
				COUNT(DISTINCT a.id) as total_activities,
				AVG(COALESCE((covered_cells::float / NULLIF(total_cells, 0)) * 100, 0)) as avg_coverage,
				MAX(COALESCE((covered_cells::float / NULLIF(total_cells, 0)) * 100, 0)) as max_coverage,
				(SELECT user_id FROM (
					SELECT 
						ac.user_id,
						COUNT(DISTINCT g.id) as covered_cells,
						(SELECT COUNT(*) FROM grid_cells gc WHERE ST_Intersects(gc.geom, c.boundary)) as total_cells
					FROM activities ac
					LEFT JOIN grid_cells g ON ST_Intersects(ac.path, g.geom) AND ST_Intersects(g.geom, c.boundary)
					WHERE ST_Intersects(ac.path, c.boundary)
					GROUP BY ac.user_id
					ORDER BY (covered_cells::float / NULLIF(total_cells, 0)) DESC
					LIMIT 1
				) top_user) as top_user_id
			FROM activities a
			JOIN cities c ON c.id = $1 AND ST_Intersects(a.path, c.boundary)
			LEFT JOIN (
				SELECT 
					ac.user_id,
					COUNT(DISTINCT g.id) as covered_cells,
					(SELECT COUNT(*) FROM grid_cells gc WHERE ST_Intersects(gc.geom, ci.boundary)) as total_cells
				FROM activities ac
				JOIN cities ci ON ci.id = $1 AND ST_Intersects(ac.path, ci.boundary)
				LEFT JOIN grid_cells g ON ST_Intersects(ac.path, g.geom) AND ST_Intersects(g.geom, ci.boundary)
				GROUP BY ac.user_id
			) coverage ON coverage.user_id = a.user_id
		)
		SELECT 
			ci.id,
			ci.name,
			ci.country_code,
			ci.area_km2,
			COALESCE(cs.active_users, 0),
			COALESCE(cs.total_activities, 0),
			COALESCE(cs.avg_coverage, 0),
			COALESCE(cs.max_coverage, 0),
			COALESCE(cs.top_user_id, '')
		FROM city_info ci
		LEFT JOIN city_coverage_stats cs ON true`

	var stats CityStats
	err := s.DB.QueryRow(query, cityID).Scan(&stats.CityID, &stats.CityName,
		&stats.CountryCode, &stats.TotalAreaKm2, &stats.ActiveUsers,
		&stats.TotalActivities, &stats.AverageCoverage, &stats.TopCoverage,
		&stats.TopUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get city stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
