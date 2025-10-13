package coverage

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// CommentService handles Strava comment posting
type CommentService struct {
	DB     *storage.DB
	Config *config.Config
	client *resty.Client
}

// NewCommentService creates a new comment service
func NewCommentService(db *storage.DB, cfg *config.Config) *CommentService {
	return &CommentService{
		DB:     db,
		Config: cfg,
		client: resty.New(),
	}
}

// RegisterCommentRoutes adds comment posting endpoints
func (s *CommentService) RegisterCommentRoutes(r *gin.Engine) {
	comments := r.Group("/api/comments")
	{
		comments.POST("/post/:activityId", s.PostCoverageCommentHandler)
		comments.POST("/post-all/:userId", s.PostAllUncommentedHandler)
	}
}

// PostCoverageCommentHandler posts a coverage comment to a specific activity
func (s *CommentService) PostCoverageCommentHandler(c *gin.Context) {
	activityIDStr := c.Param("activityId")
	activityID, err := strconv.ParseInt(activityIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid activity ID"})
		return
	}

	// Get activity details and coverage info
	query := `
		SELECT 
			a.user_id,
			a.strava_activity_id,
			a.coverage_percentage,
			a.comment_posted,
			c.name as city_name
		FROM activities a
		LEFT JOIN cities c ON a.city_id = c.id
		WHERE a.strava_activity_id = $1`

	var userID int
	var coveragePercent *float64
	var commentPosted bool
	var cityName *string

	err = s.DB.QueryRow(query, activityID).Scan(&userID, &activityID, &coveragePercent, &commentPosted, &cityName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Activity not found"})
		return
	}

	if commentPosted {
		c.JSON(http.StatusConflict, gin.H{"error": "Comment already posted for this activity"})
		return
	}

	if coveragePercent == nil || cityName == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Activity has no coverage data calculated"})
		return
	}

	// Get user's access token
	tokenPtr, err := s.DB.GetStravaToken(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No access token found for user"})
		return
	}
	token := *tokenPtr

	// Generate comment text
	commentText := s.generateCoverageComment(*cityName, *coveragePercent, userID)

	// Post comment to Strava
	err = s.postStravaComment(activityID, commentText, token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to post comment: %v", err)})
		return
	}

	// Mark activity as commented
	updateQuery := `UPDATE activities SET comment_posted = true, updated_at = CURRENT_TIMESTAMP WHERE strava_activity_id = $1`
	_, err = s.DB.Exec(updateQuery, activityID)
	if err != nil {
		fmt.Printf("Warning: Failed to mark activity as commented: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Comment posted successfully",
		"comment":          commentText,
		"coverage_percent": *coveragePercent,
		"city":             *cityName,
	})
}

// generateCoverageComment creates the comment text based on coverage data
func (s *CommentService) generateCoverageComment(cityName string, coveragePercent float64, userID int) string {
	// Get user's total coverage for context
	totalCoverageQuery := `
		WITH user_coverage AS (
			SELECT 
				COUNT(*) as total_activities,
				AVG(coverage_percentage) as avg_coverage,
				MAX(coverage_percentage) as max_coverage
			FROM activities 
			WHERE user_id = $1 AND city_id = (
				SELECT id FROM cities WHERE name = $2 LIMIT 1
			)
		)
		SELECT 
			COALESCE(total_activities, 0),
			COALESCE(avg_coverage, 0),
			COALESCE(max_coverage, 0)
		FROM user_coverage`

	var totalActivities int
	var avgCoverage, maxCoverage float64
	err := s.DB.QueryRow(totalCoverageQuery, userID, cityName).Scan(&totalActivities, &avgCoverage, &maxCoverage)
	if err != nil {
		// Fallback if query fails
		return fmt.Sprintf("🏃‍♂️ City Coverage: %.1f%% of %s explored! 🗺️", coveragePercent, cityName)
	}

	baseComment := fmt.Sprintf("🏃‍♂️ City Coverage Update!\n📍 %s: %.1f%% explored", cityName, coveragePercent)

	if totalActivities > 1 {
		baseComment += fmt.Sprintf("\n📊 Your stats: %d activities, %.1f%% avg coverage", totalActivities, avgCoverage)
	}

	if coveragePercent == maxCoverage && coveragePercent > 0 {
		baseComment += "\n🎉 Personal best coverage!"
	}

	if coveragePercent >= 50 {
		baseComment += "\n🌟 Excellent exploration!"
	} else if coveragePercent >= 25 {
		baseComment += "\n✨ Great progress!"
	} else if coveragePercent >= 10 {
		baseComment += "\n🚀 Keep exploring!"
	}

	baseComment += "\n\n#CityCoverage #StravaRunning"

	return baseComment
}

// postStravaComment posts a comment to Strava activity
func (s *CommentService) postStravaComment(activityID int64, comment string, accessToken string) error {
	url := fmt.Sprintf("https://www.strava.com/api/v3/activities/%d/comments", activityID)

	resp, err := s.client.R().
		SetAuthToken(accessToken).
		SetFormData(map[string]string{
			"text": comment,
		}).
		Post(url)

	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}

	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return fmt.Errorf("Strava API error: %d - %s", resp.StatusCode(), string(resp.Body()))
	}

	return nil
}

// PostAllUncommentedHandler posts comments for all uncommented activities for a user
func (s *CommentService) PostAllUncommentedHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get all uncommented activities with coverage data
	query := `
		SELECT 
			a.strava_activity_id,
			a.coverage_percentage,
			c.name as city_name
		FROM activities a
		JOIN cities c ON a.city_id = c.id
		WHERE a.user_id = $1 
		AND a.comment_posted = false 
		AND a.coverage_percentage IS NOT NULL
		ORDER BY a.created_at DESC`

	rows, err := s.DB.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activities"})
		return
	}
	defer rows.Close()

	// Get user's access token
	var token storage.StravaToken
	err = s.DB.Get(&token, "SELECT * FROM strava_tokens WHERE user_id = $1", userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No access token found for user"})
		return
	}

	var results []map[string]interface{}
	var successCount, failCount int

	for rows.Next() {
		var activityID int64
		var coveragePercent float64
		var cityName string

		err := rows.Scan(&activityID, &coveragePercent, &cityName)
		if err != nil {
			failCount++
			continue
		}

		commentText := s.generateCoverageComment(cityName, coveragePercent, userID)

		err = s.postStravaComment(activityID, commentText, token.AccessToken)
		if err != nil {
			results = append(results, map[string]interface{}{
				"activity_id": activityID,
				"status":      "failed",
				"error":       err.Error(),
			})
			failCount++
		} else {
			// Mark as commented
			s.DB.Exec("UPDATE activities SET comment_posted = true WHERE strava_activity_id = $1", activityID)
			results = append(results, map[string]interface{}{
				"activity_id":      activityID,
				"status":           "success",
				"comment":          commentText,
				"coverage_percent": coveragePercent,
				"city":             cityName,
			})
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       fmt.Sprintf("Processed %d activities", len(results)),
		"success_count": successCount,
		"fail_count":    failCount,
		"results":       results,
	})
}
