package coverage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// CityService handles city-related operations
type CityService struct {
	DB     *storage.DB
	client *resty.Client
}

// ExternalCityData represents city data from external API
type ExternalCityData struct {
	Name        string  `json:"name"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Population  int     `json:"population"`
}

// NewCityService creates a new city service
func NewCityService(db *storage.DB) *CityService {
	return &CityService{
		DB:     db,
		client: resty.New(),
	}
}

// RegisterCityRoutes adds city management endpoints
func (s *CityService) RegisterCityRoutes(r *gin.Engine) {
	cities := r.Group("/api/cities")
	{
		cities.GET("/", s.GetCitiesHandler)
		cities.GET("/search", s.SearchCitiesHandler)
		cities.POST("/external", s.CreateCityFromExternalHandler)
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
	Latitude    float64 `json:"latitude,omitempty"`  // For external cities
	Longitude   float64 `json:"longitude,omitempty"` // For external cities
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

// SearchCitiesHandler searches for cities by name, using both local database and external API
func (s *CityService) SearchCitiesHandler(c *gin.Context) {
	searchQuery := c.Query("q")
	if searchQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query parameter 'q' is required"})
		return
	}

	searchQuery = strings.TrimSpace(searchQuery)
	if len(searchQuery) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query must be at least 2 characters"})
		return
	}

	fmt.Printf("Searching for cities with query: %s\n", searchQuery)

	// First search local database
	localCities := s.searchLocalCities(searchQuery)

	// If we have good local results, return them
	if len(localCities) >= 3 {
		fmt.Printf("Found %d cities in local database\n", len(localCities))
		c.JSON(http.StatusOK, localCities)
		return
	}

	// Otherwise, search external API and combine results
	externalCities := s.searchExternalCities(searchQuery)

	// Combine and deduplicate results
	allCities := s.combineAndDeduplicateCities(localCities, externalCities)

	fmt.Printf("Found %d total cities (local: %d, external: %d)\n", len(allCities), len(localCities), len(externalCities))
	c.JSON(http.StatusOK, allCities)
}

// searchLocalCities searches the local database
func (s *CityService) searchLocalCities(query string) []City {
	searchPattern := "%" + query + "%"
	sqlQuery := `
		SELECT 
			id, 
			name, 
			country_code,
			COALESCE(ST_Area(ST_Transform(boundary, 3857)) / 1000000, 0) AS area_km2
		FROM cities 
		WHERE LOWER(name) LIKE LOWER($1) OR LOWER(country_code) LIKE LOWER($1)
		ORDER BY 
			CASE 
				WHEN LOWER(name) = LOWER($2) THEN 1
				WHEN LOWER(name) LIKE LOWER($3) THEN 2  
				WHEN LOWER(country_code) = LOWER($2) THEN 3
				ELSE 4 
			END,
			name
		LIMIT 25`

	exactMatch := query
	prefixMatch := query + "%"

	rows, err := s.DB.Query(sqlQuery, searchPattern, exactMatch, prefixMatch)
	if err != nil {
		fmt.Printf("Error searching local cities: %v\n", err)
		return []City{}
	}
	defer rows.Close()

	var cities []City
	for rows.Next() {
		var city City
		if err := rows.Scan(&city.ID, &city.Name, &city.CountryCode, &city.AreaKm2); err != nil {
			fmt.Printf("Error scanning city row: %v\n", err)
			continue
		}
		cities = append(cities, city)
	}

	return cities
}

// searchExternalCities searches using external geocoding API
func (s *CityService) searchExternalCities(query string) []City {
	// Using OpenStreetMap Nominatim API (free, no API key required)
	url := "https://nominatim.openstreetmap.org/search"

	resp, err := s.client.R().
		SetQueryParams(map[string]string{
			"q":      query,
			"format": "json",
			"class":  "place",
			"type":   "city,town,village",
			"limit":  "25",
		}).
		SetHeader("User-Agent", "StravaCoverage/1.0").
		Get(url)

	if err != nil {
		fmt.Printf("Error calling external geocoding API: %v\n", err)
		return []City{}
	}

	var nominatimResults []struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		Class       string `json:"class"`
		Type        string `json:"type"`
	}

	if err := json.Unmarshal(resp.Body(), &nominatimResults); err != nil {
		fmt.Printf("Error parsing geocoding response: %v\n", err)
		return []City{}
	}

	var cities []City
	for _, result := range nominatimResults {
		// Extract country code from display_name (last part usually)
		parts := strings.Split(result.DisplayName, ", ")
		countryCode := "XX" // Default
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			if len(lastPart) <= 3 {
				countryCode = strings.ToUpper(lastPart)
			}
		}

		// Parse latitude and longitude
		lat, latErr := strconv.ParseFloat(result.Lat, 64)
		lon, lonErr := strconv.ParseFloat(result.Lon, 64)

		if latErr != nil || lonErr != nil {
			fmt.Printf("Error parsing coordinates for %s: lat=%s, lon=%s\n", result.Name, result.Lat, result.Lon)
			continue // Skip cities with invalid coordinates
		}

		// Create a city with estimated area (we don't have exact boundaries from Nominatim)
		city := City{
			ID:          0, // External cities have ID 0 to distinguish them
			Name:        result.Name,
			CountryCode: countryCode,
			AreaKm2:     50.0, // Estimated area for cities
			Latitude:    lat,
			Longitude:   lon,
		}
		cities = append(cities, city)
	}

	return cities
}

// combineAndDeduplicateCities merges local and external results, removing duplicates
func (s *CityService) combineAndDeduplicateCities(local, external []City) []City {
	seen := make(map[string]bool)
	var combined []City

	// Add local cities first (they take priority)
	for _, city := range local {
		key := strings.ToLower(city.Name + city.CountryCode)
		if !seen[key] {
			seen[key] = true
			combined = append(combined, city)
		}
	}

	// Add external cities if not already seen
	for _, city := range external {
		key := strings.ToLower(city.Name + city.CountryCode)
		if !seen[key] {
			seen[key] = true
			combined = append(combined, city)
		}
	}

	// Limit total results
	if len(combined) > 50 {
		combined = combined[:50]
	}

	return combined
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

// CreateCityFromExternalHandler creates a city from external geocoding data
func (s *CityService) CreateCityFromExternalHandler(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		CountryCode string  `json:"country_code" binding:"required"`
		Latitude    float64 `json:"latitude" binding:"required"`
		Longitude   float64 `json:"longitude" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("Creating city from external data: %s, %s (%f, %f)\n", req.Name, req.CountryCode, req.Latitude, req.Longitude)

	// Create a circular boundary around the city center (approximately 10km radius)
	query := `
		INSERT INTO cities (name, country_code, boundary) 
		VALUES ($1, $2, ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint($3, $4), 4326), 3857), 10000), 4326))
		RETURNING id, name, country_code, ST_Area(ST_Transform(boundary, 3857)) / 1000000 AS area_km2`

	var city City
	err := s.DB.QueryRow(query, req.Name, req.CountryCode, req.Longitude, req.Latitude).
		Scan(&city.ID, &city.Name, &city.CountryCode, &city.AreaKm2)

	if err != nil {
		fmt.Printf("Error creating city: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create city: %v", err)})
		return
	}

	fmt.Printf("Successfully created city with ID %d\n", city.ID)
	c.JSON(http.StatusCreated, city)
}
