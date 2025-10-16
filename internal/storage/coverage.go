package storage

import (
	"time"
)

// City represents a city with its geographic boundary
type City struct {
	ID          int       `db:"id"`
	Name        string    `db:"name"`
	CountryCode string    `db:"country_code"`
	Boundary    []byte    `db:"boundary"` // PostGIS geometry stored as WKB
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Activity represents a Strava activity and its coverage data
type Activity struct {
	ID               int       `db:"id"`
	UserID           int       `db:"user_id"`
	StravaActivityID int64     `db:"strava_activity_id"`
	CityID           *int      `db:"city_id"` // Nullable - activity might not be in a tracked city
	Path             []byte    `db:"path"`    // PostGIS geometry stored as WKB
	CoveragePercent  *float64  `db:"coverage_percentage"`
	CommentPosted    bool      `db:"comment_posted"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// SaveActivity stores a new activity and calculates its city coverage
func (db *DB) SaveActivity(activity *Activity) error {
	query := `
        WITH activity_insert AS (
            INSERT INTO activities (user_id, strava_activity_id, path)
            VALUES ($1, $2, ST_GeomFromWKB($3, 4326))
            RETURNING id, city_id
        ),
        coverage_calc AS (
            UPDATE activities a
            SET 
                city_id = (
                    SELECT c.id 
                    FROM cities c 
                    WHERE ST_Intersects(a.path, c.boundary) 
                    ORDER BY ST_Length(ST_Intersection(a.path, c.boundary)) DESC 
                    LIMIT 1
                ),
                coverage_percentage = (
                    SELECT 
                        ROUND(
                            (ST_Length(ST_Intersection(a.path, c.boundary)) / 
                             ST_Length(ST_Union(
                                 SELECT path 
                                 FROM activities 
                                 WHERE user_id = $1 AND city_id = c.id
                             ))) * 100,
                            2
                        )
                    FROM cities c
                    WHERE c.id = a.city_id
                )
            FROM activity_insert
            WHERE a.id = activity_insert.id
            RETURNING coverage_percentage
        )
        SELECT coverage_percentage FROM coverage_calc`

	return db.QueryRow(query, activity.UserID, activity.StravaActivityID, activity.Path).
		Scan(&activity.CoveragePercent)
}

// GetUserCityCoverage gets the total coverage percentage for a user in a city
func (db *DB) GetUserCityCoverage(userID int, cityID int) (float64, error) {
	query := `
        WITH user_paths AS (
            SELECT ST_Union(path) as combined_path
            FROM activities
            WHERE user_id = $1 AND city_id = $2
        )
        SELECT 
            ROUND(
                (ST_Length(ST_Intersection(p.combined_path, c.boundary)) / 
                 ST_Length(ST_Union(
                     SELECT path 
                     FROM activities 
                     WHERE user_id = $1 AND city_id = $2
                 ))) * 100,
                2
            )
        FROM user_paths p
        JOIN cities c ON c.id = $2`

	var coverage float64
	err := db.QueryRow(query, userID, cityID).Scan(&coverage)
	return coverage, err
}

// MarkActivityCommented marks an activity as having its coverage comment posted
func (db *DB) MarkActivityCommented(activityID int64) error {
	query := `
        UPDATE activities
        SET comment_posted = true
        WHERE strava_activity_id = $1`

	_, err := db.Exec(query, activityID)
	return err
}

// GetUncommentedActivities gets activities that need coverage comments
func (db *DB) GetUncommentedActivities(userID int) ([]*Activity, error) {
	query := `
        SELECT 
            id, user_id, strava_activity_id, city_id, 
            coverage_percentage, comment_posted, created_at, updated_at
        FROM activities
        WHERE user_id = $1 
        AND comment_posted = false
        AND coverage_percentage IS NOT NULL
        ORDER BY created_at DESC`

	activities := []*Activity{}
	err := db.Select(&activities, query, userID)
	return activities, err
}

// GetCity retrieves a city by ID
func (db *DB) GetCity(cityID int) (*City, error) {
	query := `
        SELECT id, name, country_code, ST_AsBinary(boundary) as boundary, 
               created_at, updated_at
        FROM cities
        WHERE id = $1`

	city := &City{}
	err := db.QueryRow(query, cityID).Scan(
		&city.ID, &city.Name, &city.CountryCode, &city.Boundary,
		&city.CreatedAt, &city.UpdatedAt,
	)
	return city, err
}
