package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	resty "github.com/go-resty/resty/v2"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// AutoProcessor handles automatic processing of user data on login
type AutoProcessor struct {
	DB     *storage.DB
	Config *config.Config
	client *resty.Client
}

// NewAutoProcessor creates a new auto processor
func NewAutoProcessor(db *storage.DB, cfg *config.Config) *AutoProcessor {
	return &AutoProcessor{
		DB:     db,
		Config: cfg,
		client: resty.New(),
	}
}

// ProcessUserOnLogin automatically processes user's activities and maps them to cities
func (ap *AutoProcessor) ProcessUserOnLogin(userID int, accessToken string) error {
	log.Printf("Starting automatic processing for user %d", userID)

	// Step 1: Check if user already has activities imported
	hasActivities, err := ap.hasExistingActivities(userID)
	if err != nil {
		return fmt.Errorf("failed to check existing activities: %w", err)
	}

	if hasActivities {
		log.Printf("User %d already has activities, skipping import and proceeding to mapping/coverage", userID)
		// Skip import, but still try to map activities to cities and calculate coverage
		if err := ap.mapActivitiesToCities(userID); err != nil {
			log.Printf("Warning: Failed to map activities to cities for user %d: %v", userID, err)
		}
		if err := ap.calculateCoverageForUserCities(userID); err != nil {
			log.Printf("Warning: Failed to calculate coverage for user %d: %v", userID, err)
		}
		return nil
	}

	// Step 2: Import all activities from Strava
	log.Printf("Importing activities for user %d", userID)
	if err := ap.importAllActivities(userID, accessToken); err != nil {
		return fmt.Errorf("failed to import activities: %w", err)
	}

	// Step 3: Map activities to cities
	log.Printf("Mapping activities to cities for user %d", userID)
	if err := ap.mapActivitiesToCities(userID); err != nil {
		return fmt.Errorf("failed to map activities to cities: %w", err)
	}

	// Step 4: Calculate coverage for detected cities
	log.Printf("Calculating coverage for user %d", userID)
	if err := ap.calculateCoverageForUserCities(userID); err != nil {
		return fmt.Errorf("failed to calculate coverage: %w", err)
	}

	log.Printf("Completed automatic processing for user %d", userID)
	return nil
}

// hasExistingActivities checks if user already has activities in the database
func (ap *AutoProcessor) hasExistingActivities(userID int) (bool, error) {
	var count int
	err := ap.DB.QueryRow("SELECT COUNT(*) FROM activities WHERE user_id = $1", userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// importAllActivities imports all activities from Strava API with rate limit handling
func (ap *AutoProcessor) importAllActivities(userID int, accessToken string) error {
	page := 1
	perPage := 25 // Conservative to avoid rate limits (Strava allows ~100 requests per 15 min)
	totalImported := 0
	maxRetries := 3

	for {
		log.Printf("Importing page %d for user %d (imported %d so far)", page, userID, totalImported)

		var activities []StravaActivity
		var err error

		// Retry with exponential backoff for rate limits
		for attempt := 0; attempt < maxRetries; attempt++ {
			// Fetch activities from Strava API
			resp, httpErr := ap.client.R().
				SetHeader("Authorization", "Bearer "+accessToken).
				SetQueryParams(map[string]string{
					"page":     fmt.Sprintf("%d", page),
					"per_page": fmt.Sprintf("%d", perPage),
				}).
				Get("https://www.strava.com/api/v3/athlete/activities")

			if httpErr != nil {
				err = fmt.Errorf("HTTP request failed: %w", httpErr)
				break
			}

			// Check for rate limit (429) or other API errors
			if resp.StatusCode() == 429 {
				backoffDuration := time.Duration(1<<uint(attempt)) * time.Minute // 1min, 2min, 4min
				log.Printf("Rate limited on page %d, attempt %d/%d. Backing off for %v", page, attempt+1, maxRetries, backoffDuration)

				if attempt == maxRetries-1 {
					return fmt.Errorf("rate limit exceeded after %d attempts on page %d", maxRetries, page)
				}

				time.Sleep(backoffDuration)
				continue
			}

			if resp.StatusCode() != 200 {
				return fmt.Errorf("Strava API error: %d - %s", resp.StatusCode(), string(resp.Body()))
			}

			// Parse successful response
			if parseErr := json.Unmarshal(resp.Body(), &activities); parseErr != nil {
				err = fmt.Errorf("failed to parse activities page %d: %w", page, parseErr)
				break
			}

			// Success - break out of retry loop
			err = nil
			break
		}

		if err != nil {
			return err
		}

		// If no activities returned, we've reached the end
		if len(activities) == 0 {
			break
		}

		// Process each activity (using summary data only, no detailed fetches)
		for _, activity := range activities {
			if err := ap.importActivitySummary(userID, activity); err != nil {
				log.Printf("Failed to import activity %d: %v", activity.ID, err)
				continue
			}
			totalImported++
		}

		// If we got fewer activities than requested, we've reached the end
		if len(activities) < perPage {
			break
		}

		page++

		// Longer delay between pages to respect rate limits (aim for ~10-15 requests per minute)
		time.Sleep(5 * time.Second)
	}

	log.Printf("Imported %d activities for user %d", totalImported, userID)
	return nil
}

// StravaActivity represents a Strava activity from the API
type StravaActivity struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	SportType      string    `json:"sport_type"`
	Distance       float64   `json:"distance"`
	MovingTime     int       `json:"moving_time"`
	ElapsedTime    int       `json:"elapsed_time"`
	TotalElevation float64   `json:"total_elevation_gain"`
	StartDate      string    `json:"start_date"`
	StartDateLocal string    `json:"start_date_local"`
	TimeZone       string    `json:"timezone"`
	StartLatLng    []float64 `json:"start_latlng"`
	EndLatLng      []float64 `json:"end_latlng"`
	Map            struct {
		ID              string `json:"id"`
		Polyline        string `json:"polyline"`
		SummaryPolyline string `json:"summary_polyline"`
	} `json:"map"`
}

// importSingleActivity imports a single activity with detailed data
// TODO: This function will be used for selective activity import in future versions
func (ap *AutoProcessor) importSingleActivity(userID int, accessToken string, activity StravaActivity) error {
	// Check if activity already exists
	var existingID int64
	err := ap.DB.QueryRow("SELECT strava_activity_id FROM activities WHERE strava_activity_id = $1", activity.ID).Scan(&existingID)
	if err == nil {
		// Activity already exists, skip
		return nil
	}

	// Get detailed activity data including polyline
	detailedResp, err := ap.client.R().
		SetHeader("Authorization", "Bearer "+accessToken).
		Get(fmt.Sprintf("https://www.strava.com/api/v3/activities/%d", activity.ID))

	if err != nil {
		return fmt.Errorf("failed to fetch detailed activity: %w", err)
	}

	var detailedActivity StravaActivity
	if err := json.Unmarshal(detailedResp.Body(), &detailedActivity); err != nil {
		return fmt.Errorf("failed to parse detailed activity: %w", err)
	}

	// Parse start date
	startTime, err := time.Parse(time.RFC3339, detailedActivity.StartDate)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %w", err)
	}

	// Insert activity into database
	query := `
		INSERT INTO activities (
			user_id, strava_activity_id, name, activity_type, sport_type,
			distance_km, moving_time_seconds, elapsed_time_seconds,
			total_elevation_gain_m, start_time, timezone, polyline,
			start_latitude, start_longitude, end_latitude, end_longitude
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (strava_activity_id) DO NOTHING`

	var startLat, startLng, endLat, endLng *float64
	if len(detailedActivity.StartLatLng) == 2 {
		startLat = &detailedActivity.StartLatLng[0]
		startLng = &detailedActivity.StartLatLng[1]
	}
	if len(detailedActivity.EndLatLng) == 2 {
		endLat = &detailedActivity.EndLatLng[0]
		endLng = &detailedActivity.EndLatLng[1]
	}

	// Convert sport_type to activity_type if sport_type is empty
	activityType := detailedActivity.Type
	sportType := detailedActivity.SportType
	if sportType == "" {
		sportType = activityType
	}

	_, err = ap.DB.Exec(query,
		userID,
		detailedActivity.ID,
		detailedActivity.Name,
		activityType,
		sportType,
		detailedActivity.Distance/1000.0, // Convert to km
		detailedActivity.MovingTime,
		detailedActivity.ElapsedTime,
		detailedActivity.TotalElevation,
		startTime,
		detailedActivity.TimeZone,
		detailedActivity.Map.Polyline,
		startLat,
		startLng,
		endLat,
		endLng,
	)

	if err != nil {
		return fmt.Errorf("failed to insert activity: %w", err)
	}

	return nil
}

// importActivitySummary imports activity using only summary data (no additional API calls)
func (ap *AutoProcessor) importActivitySummary(userID int, activity StravaActivity) error {
	// Check if activity already exists
	var existingID int64
	err := ap.DB.QueryRow("SELECT strava_activity_id FROM activities WHERE strava_activity_id = $1", activity.ID).Scan(&existingID)
	if err == nil {
		// Activity already exists, skip
		return nil
	}

	// Parse start date
	startTime, err := time.Parse(time.RFC3339, activity.StartDate)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %w", err)
	}

	// Insert activity using summary data only
	query := `
		INSERT INTO activities (
			user_id, strava_activity_id, name, activity_type, sport_type,
			distance_km, moving_time_seconds, elapsed_time_seconds,
			total_elevation_gain_m, start_time, timezone,
			start_latitude, start_longitude, end_latitude, end_longitude
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (strava_activity_id) DO NOTHING`

	var startLat, startLng, endLat, endLng *float64
	if len(activity.StartLatLng) == 2 {
		startLat = &activity.StartLatLng[0]
		startLng = &activity.StartLatLng[1]
	}
	if len(activity.EndLatLng) == 2 {
		endLat = &activity.EndLatLng[0]
		endLng = &activity.EndLatLng[1]
	}

	// Convert sport_type to activity_type if sport_type is empty
	activityType := activity.Type
	sportType := activity.SportType
	if sportType == "" {
		sportType = activityType
	}

	_, err = ap.DB.Exec(query,
		userID,
		activity.ID,
		activity.Name,
		activityType,
		sportType,
		activity.Distance/1000.0, // Convert to km
		activity.MovingTime,
		activity.ElapsedTime,
		activity.TotalElevation,
		startTime,
		activity.TimeZone,
		startLat,
		startLng,
		endLat,
		endLng,
	)

	if err != nil {
		return fmt.Errorf("failed to insert activity: %w", err)
	}

	return nil
}

// mapActivitiesToCities maps user activities to cities based on spatial intersection and discovers new cities
func (ap *AutoProcessor) mapActivitiesToCities(userID int) error {
	// First, create geometries for activities that don't have them
	if err := ap.createActivityGeometries(userID); err != nil {
		return fmt.Errorf("failed to create activity geometries: %w", err)
	}

	// Step 1: Discover and create new cities BEFORE mapping to avoid over-assignment to large existing cities
	if err := ap.discoverAndCreateCitiesFromAllActivities(userID); err != nil {
		log.Printf("Warning: Failed to discover new cities for user %d: %v", userID, err)
		// Continue with mapping to existing cities
	}

	// Step 2: Map activities to cities (both existing and newly created)
	query := `
		UPDATE activities 
		SET city_id = (
			SELECT c.id 
			FROM cities c 
			WHERE ST_Intersects(activities.path, c.boundary)
			ORDER BY ST_Length(ST_Transform(ST_Intersection(activities.path, c.boundary), 3857)) DESC
			LIMIT 1
		)
		WHERE user_id = $1 AND path IS NOT NULL AND city_id IS NULL`

	result, err := ap.DB.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to map activities to cities: %w", err)
	}

	mapped, _ := result.RowsAffected()
	log.Printf("Mapped %d activities to cities for user %d", mapped, userID)

	return nil
}

// createActivityGeometries creates PostGIS geometries from polylines or start/end coordinates
func (ap *AutoProcessor) createActivityGeometries(userID int) error {
	// First try to create geometries from polylines if available
	polylineQuery := `
		UPDATE activities 
		SET path = ST_LineFromEncodedPolyline(polyline)
		WHERE user_id = $1 
		AND path IS NULL 
		AND polyline IS NOT NULL 
		AND polyline != ''`

	result, err := ap.DB.Exec(polylineQuery, userID)
	if err != nil {
		log.Printf("Polyline geometry creation failed (expected if PostGIS < 3.1): %v", err)
		// Fall back to coordinate-based approach
	} else {
		rowsAffected, _ := result.RowsAffected()
		log.Printf("Created geometries from polylines for %d activities for user %d", rowsAffected, userID)
	}

	// Fall back to creating simple linestrings from start/end coordinates for activities without paths
	coordQuery := `
		UPDATE activities 
		SET path = ST_GeomFromText('LINESTRING(' || 
			start_longitude || ' ' || start_latitude ||
			CASE 
				WHEN end_latitude IS NOT NULL AND end_longitude IS NOT NULL 
					AND (end_latitude != start_latitude OR end_longitude != start_longitude)
				THEN ', ' || end_longitude || ' ' || end_latitude
				ELSE ''
			END || ')', 4326)
		WHERE user_id = $1 
		AND path IS NULL 
		AND start_latitude IS NOT NULL 
		AND start_longitude IS NOT NULL`

	result2, err := ap.DB.Exec(coordQuery, userID)
	if err != nil {
		return fmt.Errorf("failed to create coordinate-based geometries: %w", err)
	}

	rowsAffected2, _ := result2.RowsAffected()
	log.Printf("Created geometries from coordinates for %d activities for user %d", rowsAffected2, userID)

	return nil
}

// calculateCoverageForUserCities calculates coverage for all cities that the user has activities in
func (ap *AutoProcessor) calculateCoverageForUserCities(userID int) error {
	// Get all cities where user has activities
	citiesQuery := `
		SELECT DISTINCT c.id, c.name
		FROM cities c
		JOIN activities a ON a.city_id = c.id
		WHERE a.user_id = $1`

	rows, err := ap.DB.Query(citiesQuery, userID)
	if err != nil {
		return fmt.Errorf("failed to get user cities: %w", err)
	}
	defer rows.Close()

	var cityIDs []int
	cityNames := make(map[int]string)

	for rows.Next() {
		var cityID int
		var cityName string
		if err := rows.Scan(&cityID, &cityName); err != nil {
			continue
		}
		cityIDs = append(cityIDs, cityID)
		cityNames[cityID] = cityName
	}

	log.Printf("Calculating coverage for %d cities for user %d", len(cityIDs), userID)

	// Calculate coverage for each city
	for _, cityID := range cityIDs {
		if err := ap.calculateCityCoverage(userID, cityID); err != nil {
			log.Printf("Failed to calculate coverage for city %d (%s): %v", cityID, cityNames[cityID], err)
			continue
		}
		log.Printf("Calculated coverage for city %d (%s)", cityID, cityNames[cityID])
	}

	return nil
}

// calculateCityCoverage calculates coverage percentage for a specific city
func (ap *AutoProcessor) calculateCityCoverage(userID, cityID int) error {
	// Simple coverage calculation based on activity intersections
	query := `
		WITH city_area AS (
			SELECT boundary, ST_Area(ST_Transform(boundary, 3857)) as area_sqm 
			FROM cities WHERE id = $2
		),
		user_activities AS (
			SELECT path FROM activities 
			WHERE user_id = $1 AND city_id = $2 AND path IS NOT NULL
		),
		covered_area AS (
			SELECT ST_Union(ST_Buffer(ST_Transform(ua.path, 3857), 50)) as covered_geom
			FROM user_activities ua
		)
		UPDATE activities 
		SET coverage_percentage = (
			SELECT 
				CASE 
					WHEN ca.area_sqm > 0 
					THEN (ST_Area(ST_Intersection(ST_Transform(ca.boundary, 3857), cov.covered_geom)) / ca.area_sqm) * 100
					ELSE 0 
				END
			FROM city_area ca, covered_area cov
		)
		WHERE user_id = $1 AND city_id = $2`

	_, err := ap.DB.Exec(query, userID, cityID)
	return err
}

// discoverAndCreateCitiesForUnmappedActivities finds unmapped activities and creates cities for them
// TODO: This function will be used for automatic city discovery in future versions
func (ap *AutoProcessor) discoverAndCreateCitiesForUnmappedActivities(userID int) error {
	// Find unique coordinate clusters for unmapped activities
	query := `
		SELECT 
			ROUND(start_latitude::numeric, 2) as rounded_lat,
			ROUND(start_longitude::numeric, 2) as rounded_lng,
			COUNT(*) as activity_count,
			AVG(start_latitude) as avg_lat,
			AVG(start_longitude) as avg_lng
		FROM activities 
		WHERE user_id = $1 
		AND city_id IS NULL 
		AND start_latitude IS NOT NULL 
		AND start_longitude IS NOT NULL
		GROUP BY ROUND(start_latitude::numeric, 2), ROUND(start_longitude::numeric, 2)
		HAVING COUNT(*) >= 3
		ORDER BY activity_count DESC
		LIMIT 5`

	rows, err := ap.DB.Query(query, userID)
	if err != nil {
		return fmt.Errorf("failed to find unmapped activity clusters: %w", err)
	}
	defer rows.Close()

	citiesCreated := 0
	for rows.Next() {
		var roundedLat, roundedLng, avgLat, avgLng float64
		var activityCount int

		if err := rows.Scan(&roundedLat, &roundedLng, &activityCount, &avgLat, &avgLng); err != nil {
			continue
		}

		log.Printf("Found cluster of %d activities around lat=%f, lng=%f", activityCount, avgLat, avgLng)

		// Try to get city name from reverse geocoding or use coordinates as name
		cityName, countryCode := ap.reverseGeocodeCity(avgLat, avgLng)
		if cityName == "" {
			cityName = fmt.Sprintf("Area_%d_%d", int(avgLat*100), int(avgLng*100))
			countryCode = "XX" // Unknown country
		}

		// Check if city with similar name already exists
		var existingCount int
		checkQuery := `SELECT COUNT(*) FROM cities WHERE name ILIKE $1 AND country_code = $2`
		if err := ap.DB.QueryRow(checkQuery, cityName, countryCode).Scan(&existingCount); err != nil {
			continue
		}

		if existingCount > 0 {
			log.Printf("City %s already exists, skipping creation", cityName)
			continue
		}

		// Create new city with circular boundary (10km radius)
		if err := ap.createCityFromCoordinates(cityName, countryCode, avgLat, avgLng); err != nil {
			log.Printf("Failed to create city %s: %v", cityName, err)
			continue
		}

		citiesCreated++
		log.Printf("Created new city: %s, %s", cityName, countryCode)
	}

	log.Printf("Created %d new cities for user %d", citiesCreated, userID)
	return nil
}

// discoverAndCreateCitiesFromAllActivities finds activity clusters from ALL user activities (regardless of current mapping) and creates cities for them
func (ap *AutoProcessor) discoverAndCreateCitiesFromAllActivities(userID int) error {
	log.Printf("Starting city discovery from all activities for user %d", userID)

	// Find unique coordinate clusters from GPS path data (since start coordinates are sparse)
	query := `
		SELECT 
			ROUND(ST_Y(ST_StartPoint(path))::numeric, 1) as rounded_lat,
			ROUND(ST_X(ST_StartPoint(path))::numeric, 1) as rounded_lng,
			COUNT(*) as activity_count,
			AVG(ST_Y(ST_StartPoint(path))) as avg_lat,
			AVG(ST_X(ST_StartPoint(path))) as avg_lng
		FROM activities 
		WHERE user_id = $1 
		AND path IS NOT NULL 
		GROUP BY ROUND(ST_Y(ST_StartPoint(path))::numeric, 1), ROUND(ST_X(ST_StartPoint(path))::numeric, 1)
		HAVING COUNT(*) >= 5
		ORDER BY activity_count DESC
		LIMIT 20`

	rows, err := ap.DB.Query(query, userID)
	if err != nil {
		return fmt.Errorf("failed to find activity clusters: %w", err)
	}
	defer rows.Close()

	citiesCreated := 0
	clustersFound := 0
	for rows.Next() {
		var roundedLat, roundedLng, avgLat, avgLng float64
		var activityCount int

		if err := rows.Scan(&roundedLat, &roundedLng, &activityCount, &avgLat, &avgLng); err != nil {
			continue
		}

		clustersFound++
		log.Printf("Found cluster #%d of %d activities around lat=%f, lng=%f", clustersFound, activityCount, avgLat, avgLng)

		// Try to get city name from reverse geocoding or use coordinates as name
		cityName, countryCode := ap.reverseGeocodeCity(avgLat, avgLng)
		if cityName == "" {
			cityName = fmt.Sprintf("Area_%d_%d", int(avgLat*100), int(avgLng*100))
			countryCode = "XX" // Unknown country
		}

		// Check if city with similar name and location already exists (within 20km)
		var existingCount int
		checkQuery := `
			SELECT COUNT(*) FROM cities 
			WHERE name ILIKE $1 AND country_code = $2 
			OR ST_DWithin(ST_Transform(ST_Point($3, $4, 4326), 3857), ST_Transform(ST_Centroid(boundary), 3857), 20000)`
		if err := ap.DB.QueryRow(checkQuery, cityName, countryCode, avgLng, avgLat).Scan(&existingCount); err != nil {
			continue
		}

		if existingCount > 0 {
			log.Printf("City %s or nearby city already exists, skipping creation", cityName)
			continue
		}

		// Create new city with circular boundary (10km radius)
		if err := ap.createCityFromCoordinates(cityName, countryCode, avgLat, avgLng); err != nil {
			log.Printf("Failed to create city %s: %v", cityName, err)
			continue
		}

		citiesCreated++
		log.Printf("Created new city: %s, %s", cityName, countryCode)
	}

	log.Printf("Found %d activity clusters for user %d, created %d new cities", clustersFound, userID, citiesCreated)
	if clustersFound > 0 && citiesCreated == 0 {
		log.Printf("Warning: Found activity clusters but no new cities were created - they may already exist or be too close to existing cities")
	}
	return nil
}

// NominatimResponse represents the response from Nominatim reverse geocoding API
type NominatimResponse struct {
	Address struct {
		City        string `json:"city"`
		Town        string `json:"town"`
		Village     string `json:"village"`
		County      string `json:"county"`
		State       string `json:"state"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
	} `json:"address"`
	DisplayName string `json:"display_name"`
}

// reverseGeocodeCity attempts to get city name from coordinates using Nominatim API
func (ap *AutoProcessor) reverseGeocodeCity(lat, lng float64) (string, string) {
	// Use Nominatim (OpenStreetMap) reverse geocoding API
	url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f&zoom=12&addressdetails=1", lat, lng)

	resp, err := ap.client.R().
		SetHeader("User-Agent", "Strava-Coverage/1.0 (contact@example.com)").
		Get(url)

	if err != nil || resp.StatusCode() != 200 {
		log.Printf("Reverse geocoding API failed for lat=%f, lng=%f: %v", lat, lng, err)
		return ap.fallbackCityName(lat, lng)
	}

	var result NominatimResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		log.Printf("Failed to parse reverse geocoding response for lat=%f, lng=%f: %v", lat, lng, err)
		return ap.fallbackCityName(lat, lng)
	}

	// Extract city name (try different fields)
	cityName := result.Address.City
	if cityName == "" {
		cityName = result.Address.Town
	}
	if cityName == "" {
		cityName = result.Address.Village
	}
	if cityName == "" {
		cityName = result.Address.County
	}
	if cityName == "" {
		cityName = result.Address.State
	}

	countryCode := result.Address.CountryCode
	if countryCode == "" {
		countryCode = "XX"
	} else {
		countryCode = strings.ToUpper(countryCode)
	}

	if cityName == "" {
		log.Printf("No city name found in API response for lat=%f, lng=%f", lat, lng)
		return ap.fallbackCityName(lat, lng)
	}

	log.Printf("Reverse geocoded lat=%f, lng=%f to %s, %s", lat, lng, cityName, countryCode)
	return cityName, countryCode
}

// fallbackCityName generates a fallback city name when API fails
func (ap *AutoProcessor) fallbackCityName(lat, lng float64) (string, string) {
	// Basic UK coordinate detection with more cities
	if lat >= 49.0 && lat <= 61.0 && lng >= -8.0 && lng <= 2.0 {
		if lat >= 52.7 && lat <= 52.8 && lng >= -1.3 && lng <= -1.1 {
			return "Loughborough", "GB"
		}
		if lat >= 52.4 && lat <= 52.6 && lng >= -1.3 && lng <= -1.0 {
			return "Leicester", "GB"
		}
		if lat >= 53.35 && lat <= 53.45 && lng >= -1.6 && lng <= -1.3 {
			return "Sheffield", "GB"
		}
		if lat >= 51.45 && lat <= 51.55 && lng >= -0.2 && lng <= 0.1 {
			return "London", "GB"
		}
		return fmt.Sprintf("UK_City_%.2f_%.2f", lat, lng), "GB"
	}
	if lat >= 45.0 && lat <= 51.0 && lng >= -5.0 && lng <= 8.0 {
		return fmt.Sprintf("France_%.2f_%.2f", lat, lng), "FR"
	}
	if lat >= 25.0 && lat <= 49.0 && lng >= -125.0 && lng <= -66.0 {
		return fmt.Sprintf("US_City_%.2f_%.2f", lat, lng), "US"
	}
	return fmt.Sprintf("Area_%.2f_%.2f", lat, lng), "XX"
} // createCityFromCoordinates creates a new city in the database
func (ap *AutoProcessor) createCityFromCoordinates(name, countryCode string, lat, lng float64) error {
	// Create a circular boundary around the city center (10km radius)
	query := `
		INSERT INTO cities (name, country_code, boundary) 
		VALUES ($1, $2, ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint($3, $4), 4326), 3857), 10000), 4326))
		RETURNING id`

	var cityID int
	err := ap.DB.QueryRow(query, name, countryCode, lng, lat).Scan(&cityID)
	if err != nil {
		return fmt.Errorf("failed to insert city: %w", err)
	}

	log.Printf("Created city '%s' with ID %d at coordinates (%.6f, %.6f)", name, cityID, lat, lng)
	return nil
}
