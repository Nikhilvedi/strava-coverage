#!/bin/bash

# Exit on error
set -e

echo "Creating database if it doesn't exist..."
createdb strava_coverage 2>/dev/null || true

echo "Enabling PostGIS extension..."
psql strava_coverage -c "CREATE EXTENSION IF NOT EXISTS postgis;"

echo "Running migrations..."
for migration in ../internal/storage/migrations/*.sql; do
    echo "Applying migration: $migration"
    psql strava_coverage -f "$migration"
done

echo "Database setup complete!"