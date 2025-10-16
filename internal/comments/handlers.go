package comments

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// Handler provides HTTP handlers for auto-comment functionality
type Handler struct {
	service *AutoCommentService
}

// NewHandler creates a new comment handler
func NewHandler(db *storage.DB, cfg *config.Config) *Handler {
	return &Handler{
		service: NewAutoCommentService(db, cfg),
	}
}

// RegisterRoutes registers comment-related routes
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	comments := router.Group("/api/comments")
	{
		comments.GET("/settings/user/:userId", h.GetCommentSettingsHandler)
		comments.PUT("/settings/user/:userId", h.UpdateCommentSettingsHandler)
		comments.POST("/process/user/:userId", h.ProcessCommentsHandler)
		comments.GET("/increases/user/:userId", h.GetCoverageIncreasesHandler)
	}
}

// GetCommentSettingsHandler gets user's auto-comment settings
func (h *Handler) GetCommentSettingsHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	settings, err := h.service.GetUserCommentSettings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comment settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// UpdateCommentSettingsHandler updates user's auto-comment settings
func (h *Handler) UpdateCommentSettingsHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var settings CommentSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.service.UpdateUserCommentSettings(userID, &settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update comment settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Comment settings updated successfully",
		"settings": settings,
	})
}

// ProcessCommentsHandler manually triggers comment processing for a user
func (h *Handler) ProcessCommentsHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get access token from request (you might want to get this from user session/db)
	accessToken := c.GetHeader("Authorization")
	if accessToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Access token required"})
		return
	}

	// Remove "Bearer " prefix if present
	if len(accessToken) > 7 && accessToken[:7] == "Bearer " {
		accessToken = accessToken[7:]
	}

	// Process comments in background
	go func() {
		if err := h.service.ProcessAutoCommentsForUser(userID, accessToken); err != nil {
			// Log error but don't return it to client since this is async
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Comment processing started for user " + userIDStr,
		"user_id": userID,
	})
}

// GetCoverageIncreasesHandler gets pending coverage increases for a user
func (h *Handler) GetCoverageIncreasesHandler(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	increases, err := h.service.DetectCoverageIncreases(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to detect coverage increases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"increases": increases,
		"count":     len(increases),
		"user_id":   userID,
	})
}
