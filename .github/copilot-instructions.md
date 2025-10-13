# Strava Coverage Project - AI Agent Instructions

This document provides essential context for AI agents working with the Strava Coverage codebase.

## Project Overview
Strava Coverage is a Go service that integrates with Strava's API to analyze activity coverage. The project follows a modular architecture with clear separation of concerns.

## Architecture & Components

### Core Components
- `cmd/server/`: Entry point for the HTTP server using Gin framework
- `internal/`: Contains core business logic divided into:
  - `auth/`: Strava OAuth authentication
  - `coverage/`: Activity coverage analysis
  - `storage/`: Data persistence layer
  - `strava/`: Strava API integration
- `config/`: Configuration management using environment variables

### Key Patterns
1. **Configuration Management**:
   - Uses `.env` files for local development
   - Environment variables loaded via `config.Load()` in `config/config.go`
   - Required variables: `STRAVA_CLIENT_ID`, `STRAVA_CLIENT_SECRET`, `STRAVA_REDIRECT_URI`, `DB_URL`

2. **HTTP Server**:
   - Built with Gin framework (`github.com/gin-gonic/gin`)
   - Main server initialization in `cmd/server/main.go`
   - Default port: 8080

3. **Database**:
   - PostgreSQL with `sqlx` for enhanced database operations
   - Connection string format: `postgres://user:pass@localhost:5432/strava_coverage?sslmode=disable`

## Development Workflow

### Setup
1. Copy `.env.example` to `.env` and fill in required values:
   ```env
   STRAVA_CLIENT_ID=your_client_id
   STRAVA_CLIENT_SECRET=your_client_secret
   STRAVA_REDIRECT_URI=http://localhost:8080/oauth/callback
   DB_URL=postgres://user:pass@localhost:5432/strava_coverage?sslmode=disable
   ```

### Dependencies
- Go 1.25.2 or higher
- PostgreSQL database
- Strava API credentials

### Core Libraries
- `gin-gonic/gin`: HTTP web framework
- `jmoiron/sqlx`: Enhanced database operations
- `go-resty/resty`: HTTP client for Strava API
- `joho/godotenv`: Environment variable management

## Best Practices
1. Use structured logging with context
2. Implement graceful shutdown for HTTP server
3. Follow standard Go project layout conventions
4. Use dependency injection for better testability
5. Keep handler logic thin, moving business logic to internal packages

## Integration Points
1. **Strava API**:
   - OAuth flow handled in `internal/auth`
   - API client in `internal/strava`
   - Required scopes: TBD

2. **Database**:
   - Connection managed via `internal/storage`
   - Schema migrations: TBD
   - Queries should use prepared statements via `sqlx`