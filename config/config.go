package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	StravaClientID     string
	StravaClientSecret string
	StravaRedirectURI  string
	DBUrl              string
	FrontendURL        string
}

func Load() *Config {
	_ = godotenv.Load()
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000" // Default for local development
	}
	return &Config{
		StravaClientID:     os.Getenv("STRAVA_CLIENT_ID"),
		StravaClientSecret: os.Getenv("STRAVA_CLIENT_SECRET"),
		StravaRedirectURI:  os.Getenv("STRAVA_REDIRECT_URI"),
		DBUrl:              os.Getenv("DB_URL"),
		FrontendURL:        frontendURL,
	}
}
