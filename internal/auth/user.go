package auth

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetUserHandler returns user information by user ID
func (s *Service) GetUserHandler(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get user from database by internal ID
	var user struct {
		ID       int    `db:"id"`
		StravaID int64  `db:"strava_id"`
		Name     string `db:"name"`
	}

	query := "SELECT id, strava_id, name FROM users WHERE id = $1"
	err = s.db.QueryRowx(query, userID).StructScan(&user)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	response := gin.H{
		"id":        user.ID,
		"strava_id": user.StravaID,
		"name":      user.Name,
		"email":     "", // We don't store email
	}

	c.JSON(http.StatusOK, response)
}
