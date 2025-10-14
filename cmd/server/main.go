package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/auth"
	"github.com/nikhilvedi/strava-coverage/internal/coverage"
	"github.com/nikhilvedi/strava-coverage/internal/middleware"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

func main() {
	cfg := config.Load()

	// Initialize database
	db, err := storage.NewDB(cfg.DBUrl)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	gin.SetMode(gin.DebugMode)
	r := gin.New()

	// Recovery middleware
	r.Use(gin.Recovery())

	// Add custom middleware
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.LoggingMiddleware())
	r.Use(middleware.ErrorHandlingMiddleware())
	r.Use(middleware.CORSMiddleware())

	// Initialize and setup auth service
	authService := auth.NewService(cfg, db)
	authService.SetupRoutes(r)

	// Register import activity endpoint
	importService := coverage.NewImportService(db, cfg)
	importService.RegisterImportRoutes(r)

	// Register city management endpoints
	cityService := coverage.NewCityService(db)
	cityService.RegisterCityRoutes(r)

	// Register coverage calculation endpoints
	coverageService := coverage.NewCoverageService(db)
	coverageService.RegisterCoverageRoutes(r)

	// Register comment posting endpoints
	commentService := coverage.NewCommentService(db, cfg)
	commentService.RegisterCommentRoutes(r)

	// Register custom areas endpoints
	customAreasService := coverage.NewCustomAreasService(db)
	customAreasService.RegisterCustomAreaRoutes(r)

	// Register automation and webhook endpoints
	automationService := coverage.NewAutomationService(db, cfg, coverageService, commentService)
	automationService.RegisterAutomationRoutes(r)

	// Register city detection endpoints
	detectionService := coverage.NewCityDetectionService(db)
	detectionService.RegisterCityDetectionRoutes(r)

	// Register multi-city coverage endpoints
	multiCoverageService := coverage.NewMultiCityCoverageService(db)
	multiCoverageService.RegisterMultiCityCoverageRoutes(r)

	// Register initial import endpoints
	initialImportService := coverage.NewInitialImportService(db, cfg, coverageService, commentService, detectionService)
	initialImportService.RegisterInitialImportRoutes(r)

	// Register map endpoints
	mapService := coverage.NewMapService(db)
	mapService.RegisterMapRoutes(r)

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})
	}

	// Run the server directly without graceful shutdown
	addr := ":8080"
	log.Printf("Server listening on %s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}

	log.Println("Server exiting...")
}
