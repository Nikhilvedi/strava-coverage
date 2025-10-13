# Strava Coverage

Track and visualize your city coverage through Strava activities.

## Features

- Import Strava activities and store them in PostGIS
- Calculate coverage percentage of city streets
- Automatically post coverage statistics as Strava comments
- Support for multiple cities and activities

## Prerequisites

- Go 1.25.2 or higher
- PostgreSQL with PostGIS extension
- Strava API credentials

## Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/strava-coverage.git
   cd strava-coverage
   ```

2. Copy `.env.example` to `.env` and fill in your values:
   ```bash
   cp .env.example .env
   ```

3. Set up the database:
   ```bash
   cd scripts
   ./setup_db.sh
   ```

4. Run the server:
   ```bash
   go run ./cmd/server
   ```

## Development

The project follows standard Go project layout conventions:

- `cmd/server/`: Main application entry point
- `internal/`: Internal packages
  - `auth/`: Strava OAuth implementation
  - `coverage/`: Coverage calculation logic
  - `storage/`: Database operations
  - `strava/`: Strava API client

## Environment Variables

- `STRAVA_CLIENT_ID`: Your Strava API client ID
- `STRAVA_CLIENT_SECRET`: Your Strava API client secret
- `STRAVA_REDIRECT_URI`: OAuth callback URL
- `DB_URL`: PostgreSQL connection string

## License

MIT License