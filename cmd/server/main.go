package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhilvedi/strava-coverage/config"
	"github.com/nikhilvedi/strava-coverage/internal/auth"
	"github.com/nikhilvedi/strava-coverage/internal/comments"
	"github.com/nikhilvedi/strava-coverage/internal/coverage"
	"github.com/nikhilvedi/strava-coverage/internal/middleware"
	"github.com/nikhilvedi/strava-coverage/internal/storage"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func run() error {
	// Load configuration
	cfg := config.Load()
	if cfg.DBUrl == "" {
		return fmt.Errorf("DB_URL environment variable is required")
	}

	// Initialize database
	db, err := storage.NewDB(cfg.DBUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
	}()

	// Configure Gin mode
	if os.Getenv("GIN_MODE") != "" {
		gin.SetMode(os.Getenv("GIN_MODE"))
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Initialize router
	r := setupRouter(cfg, db)

	// Setup HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Server shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		return err
	}

	log.Println("Server exited")
	return nil
}

func setupRouter(cfg *config.Config, db *storage.DB) *gin.Engine {
	r := gin.New()

	// Add middleware
	r.Use(gin.Recovery())
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.LoggingMiddleware())
	r.Use(middleware.ErrorHandlingMiddleware())
	r.Use(middleware.CORSMiddleware())

	// Initialize services
	authService := auth.NewService(cfg, db)
	coverageService := coverage.NewCoverageService(db)
	commentService := coverage.NewCommentService(db, cfg)

	// Register routes
	setupRoutes(r, cfg, db, authService, coverageService, commentService)

	return r
}

func setupRoutes(r *gin.Engine, cfg *config.Config, db *storage.DB, authService *auth.Service, coverageService *coverage.CoverageService, commentService *coverage.CommentService) {
	// Auth routes
	authService.SetupRoutes(r)

	// Core service routes
	importService := coverage.NewImportService(db, cfg)
	importService.RegisterImportRoutes(r)

	cityService := coverage.NewCityService(db)
	cityService.RegisterCityRoutes(r)

	coverageService.RegisterCoverageRoutes(r)

	// Comment system routes
	autoCommentHandler := comments.NewHandler(db, cfg)
	autoCommentHandler.RegisterRoutes(r)
	commentService.RegisterCommentRoutes(r)

	// Advanced features
	customAreasService := coverage.NewCustomAreasService(db)
	customAreasService.RegisterCustomAreaRoutes(r)

	automationService := coverage.NewAutomationService(db, cfg, coverageService, commentService)
	automationService.RegisterAutomationRoutes(r)

	detectionService := coverage.NewCityDetectionService(db)
	detectionService.RegisterCityDetectionRoutes(r)

	multiCoverageService := coverage.NewMultiCityCoverageService(db)
	multiCoverageService.RegisterMultiCityCoverageRoutes(r)

	initialImportService := coverage.NewInitialImportService(db, cfg, coverageService, commentService, detectionService)
	initialImportService.RegisterInitialImportRoutes(r)

	mapService := coverage.NewMapService(db)
	mapService.RegisterMapRoutes(r)

	// Health check
	api := r.Group("/api")
	{
		api.GET("/health", healthCheckHandler)
	}
}

func healthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}
