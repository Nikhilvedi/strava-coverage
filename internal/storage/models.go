package storage

import (
	"fmt"
	"time"
)

// GetTokenByUserID fetches the token for a user (by string userID)
func GetTokenByUserID(db *DB, token *StravaToken, userID string) error {
	// Convert string userID to int64 for database lookup
	var stravaID int64
	if _, err := fmt.Sscanf(userID, "%d", &stravaID); err != nil {
		return fmt.Errorf("invalid strava_id format: %s", userID)
	}

	fmt.Printf("Looking up user with Strava ID: %d\n", stravaID)
	var id int
	if err := db.Get(&id, "SELECT id FROM users WHERE strava_id = $1", stravaID); err != nil {
		fmt.Printf("Error finding user: %v\n", err)
		return fmt.Errorf("failed to find user with strava_id %d: %v", stravaID, err)
	}
	fmt.Printf("Found user with internal ID: %d\n", id)

	err := db.Get(token, "SELECT * FROM strava_tokens WHERE user_id = $1", id)
	if err != nil {
		fmt.Printf("Error finding token: %v\n", err)
		return fmt.Errorf("failed to find token for user_id %d: %v", id, err)
	}
	fmt.Printf("Successfully found token for user_id %d\n", id)
	return nil
}

// User represents a user in the database
type User struct {
	ID        int       `db:"id"`
	StravaID  int64     `db:"strava_id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// StravaToken represents OAuth tokens for Strava
type StravaToken struct {
	ID           int       `db:"id"`
	UserID       int       `db:"user_id"`
	AccessToken  string    `db:"access_token"`
	RefreshToken string    `db:"refresh_token"`
	ExpiresAt    time.Time `db:"expires_at"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// CreateUser creates a new user in the database
func (db *DB) CreateUser(stravaID int64) (*User, error) {
	query := `
        INSERT INTO users (strava_id)
        VALUES ($1)
        RETURNING id, strava_id, created_at, updated_at`

	user := &User{}
	err := db.QueryRowx(query, stravaID).StructScan(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByStravaID retrieves a user by their Strava ID
func (db *DB) GetUserByStravaID(stravaID int64) (*User, error) {
	query := `
        SELECT id, strava_id, created_at, updated_at
        FROM users
        WHERE strava_id = $1`

	user := &User{}
	err := db.QueryRowx(query, stravaID).StructScan(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpsertStravaToken creates or updates Strava tokens for a user
func (db *DB) UpsertStravaToken(userID int, token *StravaToken) error {
	query := `
        INSERT INTO strava_tokens (user_id, access_token, refresh_token, expires_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (user_id)
        DO UPDATE SET
            access_token = EXCLUDED.access_token,
            refresh_token = EXCLUDED.refresh_token,
            expires_at = EXCLUDED.expires_at,
            updated_at = CURRENT_TIMESTAMP`

	_, err := db.Exec(query, userID, token.AccessToken, token.RefreshToken, token.ExpiresAt)
	return err
}

// GetStravaToken retrieves Strava tokens for a user
func (db *DB) GetStravaToken(userID int) (*StravaToken, error) {
	query := `
        SELECT id, user_id, access_token, refresh_token, expires_at, created_at, updated_at
        FROM strava_tokens
        WHERE user_id = $1`

	token := &StravaToken{}
	err := db.QueryRowx(query, userID).StructScan(token)
	if err != nil {
		return nil, err
	}
	return token, nil
}
