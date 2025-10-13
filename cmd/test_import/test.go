package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

// Test script for the initial import system
func main() {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	cfg := config.Load()

	// Initialize database
	db, err := storage.NewDB(cfg.DBUrl)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test.go [command]")
		fmt.Println("Commands:")
		fmt.Println("  test-status <user_id>   - Check import status for user")
		fmt.Println("  list-users              - List users with Strava tokens")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "test-status":
		if len(os.Args) < 3 {
			log.Fatal("Please provide user ID")
		}
		testImportStatus(db, os.Args[2])
	case "list-users":
		listUsers(db)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func testImportStatus(db *storage.DB, userIDStr string) {
	fmt.Printf("Testing import status for user %s...\n", userIDStr)

	// Check if user exists in strava_tokens
	query := "SELECT user_id FROM strava_tokens WHERE user_id = $1"
	var userID int
	err := db.QueryRow(query, userIDStr).Scan(&userID)
	if err != nil {
		fmt.Printf("❌ User %s not found in strava_tokens table\n", userIDStr)
		fmt.Println("💡 Make sure the user has completed OAuth flow first")
		return
	}

	fmt.Printf("✅ User %s found in strava_tokens\n", userIDStr)

	// Check existing activities
	activityQuery := "SELECT COUNT(*) FROM activities WHERE user_id = $1"
	var activityCount int
	err = db.QueryRow(activityQuery, userIDStr).Scan(&activityCount)
	if err == nil {
		fmt.Printf("📊 User has %d activities in database\n", activityCount)
	}

	// Check import status
	statusQuery := `
		SELECT in_progress, imported_count, processed_count, failed_count 
		FROM import_status 
		WHERE user_id = $1`

	var inProgress bool
	var imported, processed, failed int
	err = db.QueryRow(statusQuery, userIDStr).Scan(&inProgress, &imported, &processed, &failed)

	if err != nil {
		fmt.Printf("⏳ No import status found - user ready for initial import\n")
	} else {
		fmt.Printf("📈 Import Status:\n")
		fmt.Printf("   - In Progress: %t\n", inProgress)
		fmt.Printf("   - Imported: %d\n", imported)
		fmt.Printf("   - Processed: %d\n", processed)
		fmt.Printf("   - Failed: %d\n", failed)
	}

	fmt.Printf("\n🚀 To start initial import, make a POST request to:\n")
	fmt.Printf("   curl -X POST http://localhost:8080/api/import/initial/%s\n", userIDStr)
	fmt.Printf("\n📊 To check progress:\n")
	fmt.Printf("   curl http://localhost:8080/api/import/status/%s\n", userIDStr)
}

func listUsers(db *storage.DB) {
	fmt.Println("Listing users with Strava tokens...")

	query := `
		SELECT user_id, access_token IS NOT NULL as has_token, 
		       expires_at, refresh_token IS NOT NULL as has_refresh
		FROM strava_tokens 
		ORDER BY user_id`

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var userID int
		var hasToken, hasRefresh bool
		var expiresAt string

		err := rows.Scan(&userID, &hasToken, &expiresAt, &hasRefresh)
		if err != nil {
			continue
		}

		status := "🔴"
		if hasToken && hasRefresh {
			status = "🟢"
		} else if hasToken {
			status = "🟡"
		}

		fmt.Printf("%s User %d (expires: %s)\n", status, userID, expiresAt)
		count++
	}

	if count == 0 {
		fmt.Println("❌ No users found with Strava tokens")
		fmt.Println("💡 Complete OAuth flow first at: http://localhost:8080/oauth/authorize")
	} else {
		fmt.Printf("\n✅ Found %d users ready for import\n", count)
	}
}
