package comments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// AutoCommentService handles automatic commenting on Strava activities
type AutoCommentService struct {
	DB     *storage.DB
	Config *config.Config
	client *http.Client
}

// NewAutoCommentService creates a new auto comment service
func NewAutoCommentService(db *storage.DB, cfg *config.Config) *AutoCommentService {
	return &AutoCommentService{
		DB:     db,
		Config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// CommentSettings represents user preferences for auto-commenting and feature switches
type CommentSettings struct {
	UserID              int       `json:"user_id" db:"user_id"`
	Enabled             bool      `json:"enabled" db:"enabled"`
	RunningEnabled      bool      `json:"running_enabled" db:"running_enabled"`
	CyclingEnabled      bool      `json:"cycling_enabled" db:"cycling_enabled"`
	WalkingEnabled      bool      `json:"walking_enabled" db:"walking_enabled"`
	HikingEnabled       bool      `json:"hiking_enabled" db:"hiking_enabled"`
	EBikingEnabled      bool      `json:"ebiking_enabled" db:"ebiking_enabled"`
	SkiingEnabled       bool      `json:"skiing_enabled" db:"skiing_enabled"`
	CommentTemplate     string    `json:"comment_template" db:"comment_template"`
	MinCoverageIncrease float64   `json:"min_coverage_increase" db:"min_coverage_increase"`
	CustomAreasEnabled  bool      `json:"custom_areas_enabled" db:"custom_areas_enabled"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// CoverageIncrease represents a detected coverage increase
type CoverageIncrease struct {
	UserID           int       `json:"user_id"`
	ActivityID       int64     `json:"activity_id"`
	CityID           int       `json:"city_id"`
	CityName         string    `json:"city_name"`
	PreviousCoverage float64   `json:"previous_coverage"`
	NewCoverage      float64   `json:"new_coverage"`
	Increase         float64   `json:"increase"`
	ActivityType     string    `json:"activity_type"`
	ActivityDate     time.Time `json:"activity_date"`
}

// GetUserCommentSettings retrieves user's auto-comment preferences
func (acs *AutoCommentService) GetUserCommentSettings(userID int) (*CommentSettings, error) {
	query := `
		SELECT user_id, enabled, running_enabled, cycling_enabled, walking_enabled, 
		       hiking_enabled, ebiking_enabled, skiing_enabled, comment_template, 
		       min_coverage_increase, custom_areas_enabled, created_at, updated_at
		FROM comment_settings 
		WHERE user_id = $1`

	var settings CommentSettings
	err := acs.DB.Get(&settings, query, userID)
	if err != nil {
		// Return default settings if none exist
		return &CommentSettings{
			UserID:              userID,
			Enabled:             false,
			RunningEnabled:      true,
			CyclingEnabled:      true,
			WalkingEnabled:      true,
			HikingEnabled:       true,
			EBikingEnabled:      true,
			SkiingEnabled:       true,
			CommentTemplate:     "Your coverage of {city} is {coverage}%!",
			MinCoverageIncrease: 0.1,
			CustomAreasEnabled:  false,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}, nil
	}
	return &settings, nil
}

// UpdateUserCommentSettings updates user's auto-comment preferences
func (acs *AutoCommentService) UpdateUserCommentSettings(userID int, settings *CommentSettings) error {
	settings.UserID = userID
	settings.UpdatedAt = time.Now()

	query := `
		INSERT INTO comment_settings (
			user_id, enabled, running_enabled, cycling_enabled, walking_enabled,
			hiking_enabled, ebiking_enabled, skiing_enabled, comment_template,
			min_coverage_increase, custom_areas_enabled, created_at, updated_at
		) VALUES (
			:user_id, :enabled, :running_enabled, :cycling_enabled, :walking_enabled,
			:hiking_enabled, :ebiking_enabled, :skiing_enabled, :comment_template,
			:min_coverage_increase, :custom_areas_enabled, :created_at, :updated_at
		) ON CONFLICT (user_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			running_enabled = EXCLUDED.running_enabled,
			cycling_enabled = EXCLUDED.cycling_enabled,
			walking_enabled = EXCLUDED.walking_enabled,
			hiking_enabled = EXCLUDED.hiking_enabled,
			ebiking_enabled = EXCLUDED.ebiking_enabled,
			skiing_enabled = EXCLUDED.skiing_enabled,
			comment_template = EXCLUDED.comment_template,
			min_coverage_increase = EXCLUDED.min_coverage_increase,
			custom_areas_enabled = EXCLUDED.custom_areas_enabled,
			updated_at = EXCLUDED.updated_at`

	_, err := acs.DB.NamedExec(query, settings)
	return err
}

// DetectCoverageIncreases finds activities that increased coverage and need comments
func (acs *AutoCommentService) DetectCoverageIncreases(userID int) ([]CoverageIncrease, error) {
	query := `
		WITH previous_coverage AS (
			SELECT 
				a1.city_id,
				COALESCE(MAX(a2.coverage_percentage), 0) as prev_coverage
			FROM activities a1
			LEFT JOIN activities a2 ON a1.city_id = a2.city_id 
				AND a2.user_id = a1.user_id 
				AND a2.start_date < a1.start_date
			WHERE a1.user_id = $1
				AND a1.coverage_percentage IS NOT NULL
				AND a1.commented_at IS NULL
			GROUP BY a1.city_id, a1.id
		)
		SELECT 
			a.user_id,
			a.strava_id as activity_id,
			a.city_id,
			c.name as city_name,
			COALESCE(pc.prev_coverage, 0) as previous_coverage,
			a.coverage_percentage as new_coverage,
			a.coverage_percentage - COALESCE(pc.prev_coverage, 0) as increase,
			a.activity_type,
			a.start_date as activity_date
		FROM activities a
		JOIN cities c ON a.city_id = c.id
		LEFT JOIN previous_coverage pc ON a.city_id = pc.city_id
		WHERE a.user_id = $1
			AND a.coverage_percentage IS NOT NULL
			AND a.commented_at IS NULL
			AND (a.coverage_percentage - COALESCE(pc.prev_coverage, 0)) > 0
		ORDER BY a.start_date ASC`

	var increases []CoverageIncrease
	err := acs.DB.Select(&increases, query, userID)
	return increases, err
}

// ShouldCommentOnActivity checks if we should comment on this activity type
func (acs *AutoCommentService) ShouldCommentOnActivity(settings *CommentSettings, activityType string, increase float64) bool {
	if !settings.Enabled || increase < settings.MinCoverageIncrease {
		return false
	}

	switch activityType {
	case "Run", "VirtualRun":
		return settings.RunningEnabled
	case "Ride", "VirtualRide":
		return settings.CyclingEnabled
	case "Walk":
		return settings.WalkingEnabled
	case "Hike":
		return settings.HikingEnabled
	case "EBikeRide":
		return settings.EBikingEnabled
	case "AlpineSki", "BackcountrySki", "NordicSki":
		return settings.SkiingEnabled
	default:
		return true // Default to enabled for other activity types
	}
}

// PostCommentToStrava posts a comment to a Strava activity
func (acs *AutoCommentService) PostCommentToStrava(accessToken string, activityID int64, comment string) error {
	url := fmt.Sprintf("https://www.strava.com/api/v3/activities/%d/comments", activityID)

	payload := map[string]string{
		"text": comment,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal comment payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := acs.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to post comment, status: %d", resp.StatusCode)
	}

	log.Printf("Successfully posted comment to activity %d", activityID)
	return nil
}

// FormatComment formats the comment template with actual values
func (acs *AutoCommentService) FormatComment(template, cityName string, coverage float64) string {
	comment := template
	comment = strings.ReplaceAll(comment, "{city}", cityName)
	comment = strings.ReplaceAll(comment, "{coverage}", fmt.Sprintf("%.1f", coverage))
	return comment
}

// MarkActivityAsCommented marks an activity as having been commented on
func (acs *AutoCommentService) MarkActivityAsCommented(activityID int64) error {
	query := `UPDATE activities SET commented_at = NOW() WHERE strava_id = $1`
	_, err := acs.DB.Exec(query, activityID)
	return err
}

// ProcessAutoCommentsForUser processes all pending auto-comments for a user
func (acs *AutoCommentService) ProcessAutoCommentsForUser(userID int, accessToken string) error {
	// Get user's comment settings
	settings, err := acs.GetUserCommentSettings(userID)
	if err != nil {
		return fmt.Errorf("failed to get comment settings: %w", err)
	}

	if !settings.Enabled {
		return nil // Auto-comments disabled
	}

	// Detect coverage increases
	increases, err := acs.DetectCoverageIncreases(userID)
	if err != nil {
		return fmt.Errorf("failed to detect coverage increases: %w", err)
	}

	commentsPosted := 0
	for _, increase := range increases {
		if acs.ShouldCommentOnActivity(settings, increase.ActivityType, increase.Increase) {
			comment := acs.FormatComment(settings.CommentTemplate, increase.CityName, increase.NewCoverage)

			if err := acs.PostCommentToStrava(accessToken, increase.ActivityID, comment); err != nil {
				log.Printf("Failed to post comment to activity %d: %v", increase.ActivityID, err)
				continue
			}

			if err := acs.MarkActivityAsCommented(increase.ActivityID); err != nil {
				log.Printf("Failed to mark activity %d as commented: %v", increase.ActivityID, err)
			}

			commentsPosted++

			// Rate limiting: wait between comments
			time.Sleep(2 * time.Second)
		}
	}

	log.Printf("Posted %d auto-comments for user %d", commentsPosted, userID)
	return nil
}
