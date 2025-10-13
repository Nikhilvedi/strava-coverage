package coverage

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// CityService handles city-related operations
type CityService struct {
	DB *storage.DB
}

// NewCityService creates a new city service
func NewCityService(db *storage.DB) *CityService {
	return &CityService{DB: db}
}

// RegisterCityRoutes adds city management endpoints
func (s *CityService) RegisterCityRoutes(r *gin.Engine) {
	cities := r.Group("/api/cities")
	{
		cities.GET("/", s.GetCitiesHandler)
		cities.GET("/:id", s.GetCityHandler)
		cities.GET("/:id/boundary", s.GetCityBoundaryHandler)
		cities.POST("/", s.CreateCityHandler)
	}
}

// City represents a city with its boundary
type City struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	CountryCode string  `json:"country_code"`
	AreaKm2     float64 `json:"area_km2,omitempty"`
	// Note: We don't include the actual boundary geometry in JSON responses
	// as it would be too large. Use separate endpoint for geometry if needed.
}

// GetCitiesHandler returns all cities
func (s *CityService) GetCitiesHandler(c *gin.Context) {
	query := `
		SELECT 
			id, 
			name, 
			country_code,
			ST_Area(ST_Transform(boundary, 3857)) / 1000000 AS area_km2
		FROM cities 
		ORDER BY name`

	rows, err := s.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cities"})
		return
	}
	defer rows.Close()

	var cities []City
	for rows.Next() {
		var city City
		err := rows.Scan(&city.ID, &city.Name, &city.CountryCode, &city.AreaKm2)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan city"})
			return
		}
		cities = append(cities, city)
	}

	c.JSON(http.StatusOK, cities)
}

// GetCityHandler returns a specific city by ID
func (s *CityService) GetCityHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid city ID"})
		return
	}

	query := `
		SELECT 
			id, 
			name, 
			country_code,
			ST_Area(ST_Transform(boundary, 3857)) / 1000000 AS area_km2
		FROM cities 
		WHERE id = $1`

	var city City
	err = s.DB.QueryRow(query, id).Scan(&city.ID, &city.Name, &city.CountryCode, &city.AreaKm2)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "City not found"})
		return
	}

	c.JSON(http.StatusOK, city)
}

// CreateCityRequest represents the request to create a new city
type CreateCityRequest struct {
	Name        string      `json:"name" binding:"required"`
	CountryCode string      `json:"country_code" binding:"required,len=2"`
	Boundary    interface{} `json:"boundary" binding:"required"`
}

// CreateCityHandler creates a new city with boundary
func (s *CityService) CreateCityHandler(c *gin.Context) {
	var req CreateCityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert boundary to JSON string for PostGIS
	boundaryJSON, err := json.Marshal(req.Boundary)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid boundary format"})
		return
	}

	query := `
		INSERT INTO cities (name, country_code, boundary)
		VALUES ($1, $2, ST_GeomFromGeoJSON($3))
		RETURNING id`

	var cityID int
	err = s.DB.QueryRow(query, req.Name, req.CountryCode, string(boundaryJSON)).Scan(&cityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create city"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": cityID, "message": "City created successfully"})
}

// GetCityBoundaryHandler returns the GeoJSON boundary for a city
func (s *CityService) GetCityBoundaryHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid city ID"})
		return
	}

	query := `
		SELECT ST_AsGeoJSON(boundary) as boundary_geojson
		FROM cities 
		WHERE id = $1`

	var boundaryGeoJSON string
	err = s.DB.QueryRow(query, id).Scan(&boundaryGeoJSON)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "City not found"})
		return
	}

	// Parse the GeoJSON string to return as proper JSON
	var boundary interface{}
	if err := json.Unmarshal([]byte(boundaryGeoJSON), &boundary); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse boundary"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"boundary": boundary})
}
