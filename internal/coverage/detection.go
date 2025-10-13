package coverage

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// CityDetectionService handles automatic city detection for activities
type CityDetectionService struct {
	DB *storage.DB
}

// NewCityDetectionService creates a new city detection service
func NewCityDetectionService(db *storage.DB) *CityDetectionService {
	return &CityDetectionService{DB: db}
}

// RegisterCityDetectionRoutes adds city detection endpoints
func (s *CityDetectionService) RegisterCityDetectionRoutes(r *gin.Engine) {
	detection := r.Group("/api/detection")
	{
		detection.POST("/find-cities/:activityId", s.FindCitiesForActivityHandler)
		detection.GET("/nearby-cities", s.GetNearbyCitiesHandler)
		detection.POST("/auto-detect/:userId", s.AutoDetectUserCitiesHandler)
	}
}

// ActivityCityResult represents cities that an activity intersects with
type ActivityCityResult struct {
	ActivityID         int64              `json:"activity_id"`
	IntersectingCities []CityIntersection `json:"intersecting_cities"`
	PrimaryCity        *CityIntersection  `json:"primary_city"`
}

// CityIntersection represents how an activity intersects with a city
type CityIntersection struct {
	CityID               int     `json:"city_id"`
	CityName             string  `json:"city_name"`
	CountryCode          string  `json:"country_code"`
	IntersectionLength   float64 `json:"intersection_length_km"`
	PercentageOfActivity float64 `json:"percentage_of_activity"`
}

// FindCitiesForActivityHandler finds all cities that an activity intersects with
func (s *CityDetectionService) FindCitiesForActivityHandler(c *gin.Context) {
	activityIDStr := c.Param("activityId")

	// Find all cities that intersect with this activity
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

	rows, err := s.DB.Query(query, activityIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find cities"})
		return
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

	activityID, _ := strconv.ParseInt(activityIDStr, 10, 64)
	result := ActivityCityResult{
		ActivityID:         activityID,
		IntersectingCities: intersections,
	}

	// Set primary city (longest intersection)
	if len(intersections) > 0 {
		result.PrimaryCity = &intersections[0]
	}

	c.JSON(http.StatusOK, result)
}

// GetNearbyCitiesHandler returns cities near a given coordinate
func (s *CityDetectionService) GetNearbyCitiesHandler(c *gin.Context) {
	lat := c.Query("lat")
	lng := c.Query("lng")
	radiusKm := c.DefaultQuery("radius", "50") // Default 50km radius

	if lat == "" || lng == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lng parameters required"})
		return
	}

	query := `
		SELECT 
			id,
			name,
			country_code,
			ST_Distance(
				ST_Transform(ST_SetSRID(ST_MakePoint($2, $1), 4326), 3857),
				ST_Transform(ST_Centroid(boundary), 3857)
			) / 1000 as distance_km
		FROM cities
		WHERE ST_DWithin(
			ST_Transform(ST_SetSRID(ST_MakePoint($2, $1), 4326), 3857),
			ST_Transform(boundary, 3857),
			$3 * 1000
		)
		ORDER BY distance_km`

	rows, err := s.DB.Query(query, lat, lng, radiusKm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find nearby cities"})
		return
	}
	defer rows.Close()

	var cities []map[string]interface{}
	for rows.Next() {
		var id int
		var name, countryCode string
		var distance float64

		err := rows.Scan(&id, &name, &countryCode, &distance)
		if err != nil {
			continue
		}

		cities = append(cities, map[string]interface{}{
			"id":           id,
			"name":         name,
			"country_code": countryCode,
			"distance_km":  distance,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"coordinate": map[string]string{"lat": lat, "lng": lng},
		"radius_km":  radiusKm,
		"cities":     cities,
	})
}

// AutoDetectUserCitiesHandler analyzes all user activities to find their cities
func (s *CityDetectionService) AutoDetectUserCitiesHandler(c *gin.Context) {
	userIDStr := c.Param("userId")

	// Get all activities for user and find cities they intersect with
	query := `
		SELECT DISTINCT
			c.id,
			c.name,
			c.country_code,
			COUNT(a.id) as activity_count,
			SUM(ST_Length(ST_Transform(a.path, 3857))) / 1000 as total_distance_km
		FROM cities c
		JOIN activities a ON ST_Intersects(a.path, c.boundary)
		WHERE a.user_id = $1
		GROUP BY c.id, c.name, c.country_code
		ORDER BY activity_count DESC, total_distance_km DESC`

	rows, err := s.DB.Query(query, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze user cities"})
		return
	}
	defer rows.Close()

	var userCities []map[string]interface{}
	for rows.Next() {
		var id, activityCount int
		var name, countryCode string
		var totalDistance float64

		err := rows.Scan(&id, &name, &countryCode, &activityCount, &totalDistance)
		if err != nil {
			continue
		}

		userCities = append(userCities, map[string]interface{}{
			"city_id":           id,
			"name":              name,
			"country_code":      countryCode,
			"activity_count":    activityCount,
			"total_distance_km": totalDistance,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userIDStr,
		"cities":  userCities,
		"message": fmt.Sprintf("Found %d cities with activities", len(userCities)),
	})
}
