package coverage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
	"github.com/nikhilvedi/strava-coverage/internal/utils"
)

// MapService handles map-related operations and GeoJSON generation
type MapService struct {
	DB *storage.DB
}

// NewMapService creates a new map service
func NewMapService(db *storage.DB) *MapService {
	return &MapService{DB: db}
}

// RegisterMapRoutes adds map endpoints
func (s *MapService) RegisterMapRoutes(r *gin.Engine) {
	maps := r.Group("/api/maps")
	{
		// GeoJSON endpoints
		maps.GET("/cities", s.GetCitiesGeoJSONHandler)
		maps.GET("/cities/:cityId", s.GetCityGeoJSONHandler)
		maps.GET("/activities/user/:userId", s.GetUserActivitiesGeoJSONHandler)
		maps.GET("/activities/user/:userId/city/:cityId", s.GetUserCityActivitiesGeoJSONHandler)
		maps.GET("/coverage/user/:userId/city/:cityId", s.GetCoverageGeoJSONHandler)

		// Map configuration
		maps.GET("/config", s.GetMapConfigHandler)
		maps.GET("/styles", s.GetMapStylesHandler)
		maps.GET("/bounds/city/:cityId", s.GetCityBoundsHandler)
		maps.GET("/bounds/user/:userId", s.GetUserActivityBoundsHandler)
	}
}

// GeoJSON structures
type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type GeoJSONGeometry struct {
	Type        string      `json:"type"`
	Coordinates interface{} `json:"coordinates"`
}

// Map configuration structures
type MapConfig struct {
	DefaultCenter    []float64     `json:"default_center"`
	DefaultZoom      int           `json:"default_zoom"`
	MinZoom          int           `json:"min_zoom"`
	MaxZoom          int           `json:"max_zoom"`
	TileServers      []TileServer  `json:"tile_servers"`
	LayerConfigs     []LayerConfig `json:"layer_configs"`
	StylePresets     []StylePreset `json:"style_presets"`
	InteractionModes []string      `json:"interaction_modes"`
}

type TileServer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Attribution string `json:"attribution"`
	MaxZoom     int    `json:"max_zoom"`
	Default     bool   `json:"default"`
}

type LayerConfig struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"` // "cities", "activities", "coverage"
	Endpoint    string                 `json:"endpoint"`
	Visible     bool                   `json:"visible"`
	Zoomable    bool                   `json:"zoomable"`
	Clickable   bool                   `json:"clickable"`
	Style       map[string]interface{} `json:"style"`
	PopupFields []string               `json:"popup_fields"`
}

type StylePreset struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Styles map[string]interface{} `json:"styles"`
}

type Bounds struct {
	North float64 `json:"north"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
	West  float64 `json:"west"`
}

// GetCitiesGeoJSONHandler returns all cities as GeoJSON
func (s *MapService) GetCitiesGeoJSONHandler(c *gin.Context) {
	logger := utils.NewLogger("MapService")
	logger.Info("Fetching cities GeoJSON data")

	query := `
		SELECT 
			id, name, country_code,
			ST_AsGeoJSON(boundary) as boundary_geojson,
			ST_Area(ST_Transform(boundary, 3857)) / 1000000 as area_km2
		FROM cities 
		ORDER BY name`

	rows, err := s.DB.Query(query)
	if err != nil {
		logger.Error("Failed to fetch cities: %v", err)
		apiErr := utils.NewAPIError(500, "Database error", "Failed to fetch cities data")
		utils.ErrorResponse(c, apiErr)
		return
	}
	defer rows.Close()

	var features []GeoJSONFeature
	for rows.Next() {
		var id int
		var name, countryCode, boundaryJSON string
		var areaKm2 float64

		if err := rows.Scan(&id, &name, &countryCode, &boundaryJSON, &areaKm2); err != nil {
			continue
		}

		var geometry GeoJSONGeometry
		if err := json.Unmarshal([]byte(boundaryJSON), &geometry); err != nil {
			continue
		}

		feature := GeoJSONFeature{
			Type:     "Feature",
			Geometry: geometry,
			Properties: map[string]interface{}{
				"id":           id,
				"name":         name,
				"country_code": countryCode,
				"area_km2":     areaKm2,
				"type":         "city_boundary",
			},
		}
		features = append(features, feature)
	}

	collection := GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, collection)
}

// GetCityGeoJSONHandler returns a specific city as GeoJSON
func (s *MapService) GetCityGeoJSONHandler(c *gin.Context) {
	cityIDStr := c.Param("cityId")
	cityID, err := strconv.Atoi(cityIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid city ID"})
		return
	}

	query := `
		SELECT 
			id, name, country_code,
			ST_AsGeoJSON(boundary) as boundary_geojson,
			ST_Area(ST_Transform(boundary, 3857)) / 1000000 as area_km2
		FROM cities 
		WHERE id = $1`

	var id int
	var name, countryCode, boundaryJSON string
	var areaKm2 float64

	err = s.DB.QueryRow(query, cityID).Scan(&id, &name, &countryCode, &boundaryJSON, &areaKm2)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "City not found"})
		return
	}

	var geometry GeoJSONGeometry
	if err := json.Unmarshal([]byte(boundaryJSON), &geometry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse geometry"})
		return
	}

	feature := GeoJSONFeature{
		Type:     "Feature",
		Geometry: geometry,
		Properties: map[string]interface{}{
			"id":           id,
			"name":         name,
			"country_code": countryCode,
			"area_km2":     areaKm2,
			"type":         "city_boundary",
		},
	}

	collection := GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: []GeoJSONFeature{feature},
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, collection)
}

// GetUserActivitiesGeoJSONHandler returns user activities as GeoJSON
func (s *MapService) GetUserActivitiesGeoJSONHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Optional filters
	limit := c.DefaultQuery("limit", "100")
	cityID := c.Query("city_id")

	query := `
		SELECT 
			a.strava_activity_id,
			ST_AsGeoJSON(a.path) as path_geojson,
			a.coverage_percentage,
			c.name as city_name,
			c.country_code,
			ST_Length(ST_Transform(a.path, 3857)) / 1000 as distance_km
		FROM activities a
		LEFT JOIN cities c ON a.city_id = c.id
		WHERE a.user_id = $1 AND a.path IS NOT NULL`

	args := []interface{}{userID}
	argIndex := 2

	if cityID != "" {
		query += fmt.Sprintf(" AND a.city_id = $%d", argIndex)
		cityIDInt, _ := strconv.Atoi(cityID)
		args = append(args, cityIDInt)
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY a.created_at DESC LIMIT $%d", argIndex)
	limitInt, _ := strconv.Atoi(limit)
	args = append(args, limitInt)

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activities"})
		return
	}
	defer rows.Close()

	var features []GeoJSONFeature
	for rows.Next() {
		var activityID int64
		var pathJSON string
		var coverage *float64
		var cityName, countryCode *string
		var distanceKm float64

		if err := rows.Scan(&activityID, &pathJSON, &coverage, &cityName, &countryCode, &distanceKm); err != nil {
			continue
		}

		var geometry GeoJSONGeometry
		if err := json.Unmarshal([]byte(pathJSON), &geometry); err != nil {
			continue
		}

		properties := map[string]interface{}{
			"activity_id": activityID,
			"distance_km": distanceKm,
			"type":        "activity_path",
		}

		if coverage != nil {
			properties["coverage_percentage"] = *coverage
		}
		if cityName != nil {
			properties["city_name"] = *cityName
			properties["country_code"] = *countryCode
		}

		feature := GeoJSONFeature{
			Type:       "Feature",
			Geometry:   geometry,
			Properties: properties,
		}
		features = append(features, feature)
	}

	collection := GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, collection)
}

// GetUserCityActivitiesGeoJSONHandler returns user activities for a specific city
func (s *MapService) GetUserCityActivitiesGeoJSONHandler(c *gin.Context) {
	// Set city_id parameter and call general activities handler
	c.Set("city_id", c.Param("cityId"))
	s.GetUserActivitiesGeoJSONHandler(c)
}

// GetCoverageGeoJSONHandler returns coverage grid for a user and city
func (s *MapService) GetCoverageGeoJSONHandler(c *gin.Context) {
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

	// Generate coverage grid (simplified - in production you'd cache this)
	gridSize := 100.0 // 100m grid
	query := `
		WITH city_bounds AS (
			SELECT boundary FROM cities WHERE id = $1
		),
		grid AS (
			SELECT 
				ST_AsGeoJSON(
					ST_Transform(
						ST_SetSRID(
							ST_MakeBox2D(
								ST_Point(x * $2, y * $2),
								ST_Point((x + 1) * $2, (y + 1) * $2)
							), 
							3857
						), 
						4326
					)
				) as cell_geojson,
				CASE 
					WHEN EXISTS (
						SELECT 1 FROM activities a 
						WHERE a.user_id = $3 
						AND a.city_id = $1
						AND ST_Intersects(
							a.path,
							ST_Transform(
								ST_SetSRID(
									ST_MakeBox2D(
										ST_Point(x * $2, y * $2),
										ST_Point((x + 1) * $2, (y + 1) * $2)
									), 
									3857
								), 
								4326
							)
						)
					) THEN true 
					ELSE false 
				END as covered
			FROM city_bounds cb,
			generate_series(
				floor(ST_XMin(ST_Transform(cb.boundary, 3857)) / $2)::int,
				ceil(ST_XMax(ST_Transform(cb.boundary, 3857)) / $2)::int
			) AS x,
			generate_series(
				floor(ST_YMin(ST_Transform(cb.boundary, 3857)) / $2)::int,
				ceil(ST_YMax(ST_Transform(cb.boundary, 3857)) / $2)::int
			) AS y
			WHERE ST_Intersects(
				ST_Transform(cb.boundary, 3857),
				ST_SetSRID(
					ST_MakeBox2D(
						ST_Point(x * $2, y * $2),
						ST_Point((x + 1) * $2, (y + 1) * $2)
					), 
					3857
				)
			)
		)
		SELECT cell_geojson, covered FROM grid LIMIT 1000`

	rows, err := s.DB.Query(query, cityID, gridSize, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate coverage grid"})
		return
	}
	defer rows.Close()

	var features []GeoJSONFeature
	for rows.Next() {
		var cellJSON string
		var covered bool

		if err := rows.Scan(&cellJSON, &covered); err != nil {
			continue
		}

		var geometry GeoJSONGeometry
		if err := json.Unmarshal([]byte(cellJSON), &geometry); err != nil {
			continue
		}

		feature := GeoJSONFeature{
			Type:     "Feature",
			Geometry: geometry,
			Properties: map[string]interface{}{
				"covered": covered,
				"type":    "coverage_cell",
			},
		}
		features = append(features, feature)
	}

	collection := GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, collection)
}

// GetMapConfigHandler returns map configuration
func (s *MapService) GetMapConfigHandler(c *gin.Context) {
	config := MapConfig{
		DefaultCenter: []float64{53.3811, -1.4701}, // Sheffield
		DefaultZoom:   12,
		MinZoom:       8,
		MaxZoom:       18,
		TileServers: []TileServer{
			{
				ID:          "osm",
				Name:        "OpenStreetMap",
				URL:         "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
				Attribution: "&copy; OpenStreetMap contributors",
				MaxZoom:     19,
				Default:     true,
			},
			{
				ID:          "satellite",
				Name:        "Satellite",
				URL:         "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}",
				Attribution: "&copy; Esri",
				MaxZoom:     17,
				Default:     false,
			},
		},
		LayerConfigs: []LayerConfig{
			{
				ID:          "cities",
				Name:        "City Boundaries",
				Type:        "cities",
				Endpoint:    "/api/maps/cities",
				Visible:     true,
				Zoomable:    true,
				Clickable:   true,
				PopupFields: []string{"name", "country_code", "area_km2"},
				Style: map[string]interface{}{
					"color":       "#3388ff",
					"weight":      2,
					"opacity":     0.8,
					"fillOpacity": 0.1,
				},
			},
			{
				ID:          "activities",
				Name:        "Activity Paths",
				Type:        "activities",
				Endpoint:    "/api/maps/activities/user/{userId}",
				Visible:     true,
				Zoomable:    false,
				Clickable:   true,
				PopupFields: []string{"activity_id", "distance_km", "coverage_percentage"},
				Style: map[string]interface{}{
					"color":   "#ff7800",
					"weight":  3,
					"opacity": 0.8,
				},
			},
			{
				ID:          "coverage",
				Name:        "Coverage Grid",
				Type:        "coverage",
				Endpoint:    "/api/maps/coverage/user/{userId}/city/{cityId}",
				Visible:     false,
				Zoomable:    false,
				Clickable:   false,
				PopupFields: []string{"covered"},
				Style: map[string]interface{}{
					"covered": map[string]interface{}{
						"color":       "#22c55e",
						"fillColor":   "#22c55e",
						"fillOpacity": 0.6,
						"weight":      0,
					},
					"uncovered": map[string]interface{}{
						"color":       "#ef4444",
						"fillColor":   "#ef4444",
						"fillOpacity": 0.3,
						"weight":      0,
					},
				},
			},
		},
		StylePresets: []StylePreset{
			{
				ID:   "default",
				Name: "Default",
				Styles: map[string]interface{}{
					"activity_color": "#ff7800",
					"city_color":     "#3388ff",
					"covered_color":  "#22c55e",
				},
			},
			{
				ID:   "dark",
				Name: "Dark Mode",
				Styles: map[string]interface{}{
					"activity_color": "#fbbf24",
					"city_color":     "#60a5fa",
					"covered_color":  "#34d399",
				},
			},
		},
		InteractionModes: []string{"pan", "zoom", "click", "popup"},
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, config)
}

// GetMapStylesHandler returns available map styles
func (s *MapService) GetMapStylesHandler(c *gin.Context) {
	styles := map[string]interface{}{
		"activity_styles": map[string]interface{}{
			"default": map[string]interface{}{
				"color":   "#ff7800",
				"weight":  3,
				"opacity": 0.8,
			},
			"highlighted": map[string]interface{}{
				"color":   "#ff0000",
				"weight":  5,
				"opacity": 1.0,
			},
		},
		"city_styles": map[string]interface{}{
			"default": map[string]interface{}{
				"color":       "#3388ff",
				"weight":      2,
				"opacity":     0.8,
				"fillOpacity": 0.1,
			},
			"selected": map[string]interface{}{
				"color":       "#ff0000",
				"weight":      3,
				"opacity":     1.0,
				"fillOpacity": 0.2,
			},
		},
		"coverage_styles": map[string]interface{}{
			"covered": map[string]interface{}{
				"fillColor":   "#22c55e",
				"fillOpacity": 0.6,
				"color":       "#22c55e",
				"weight":      0,
			},
			"uncovered": map[string]interface{}{
				"fillColor":   "#ef4444",
				"fillOpacity": 0.3,
				"color":       "#ef4444",
				"weight":      0,
			},
		},
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, styles)
}

// GetCityBoundsHandler returns bounds for a specific city
func (s *MapService) GetCityBoundsHandler(c *gin.Context) {
	cityIDStr := c.Param("cityId")
	cityID, err := strconv.Atoi(cityIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid city ID"})
		return
	}

	query := `
		SELECT 
			ST_YMax(boundary) as north,
			ST_YMin(boundary) as south,
			ST_XMax(boundary) as east,
			ST_XMin(boundary) as west
		FROM cities 
		WHERE id = $1`

	var bounds Bounds
	err = s.DB.QueryRow(query, cityID).Scan(&bounds.North, &bounds.South, &bounds.East, &bounds.West)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "City not found"})
		return
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, bounds)
}

// GetUserActivityBoundsHandler returns bounds for all user activities
func (s *MapService) GetUserActivityBoundsHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	query := `
		SELECT 
			ST_YMax(ST_Extent(path)) as north,
			ST_YMin(ST_Extent(path)) as south,
			ST_XMax(ST_Extent(path)) as east,
			ST_XMin(ST_Extent(path)) as west
		FROM activities 
		WHERE user_id = $1 AND path IS NOT NULL`

	var bounds Bounds
	err = s.DB.QueryRow(query, userID).Scan(&bounds.North, &bounds.South, &bounds.East, &bounds.West)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No activities found"})
		return
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.JSON(http.StatusOK, bounds)
}
