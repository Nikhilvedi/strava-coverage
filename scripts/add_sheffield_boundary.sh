#!/bin/bash

# Exit on error
set -e

echo "Adding Sheffield city boundary data..."

# Connect to the database and insert Sheffield boundary
psql "postgres://nikhilvediair@localhost:5432/strava_coverage" << 'EOF'

-- Insert Sheffield city boundary (simplified polygon)
INSERT INTO cities (name, country_code, boundary) VALUES (
    'Sheffield',
    'GB',
    ST_GeomFromText('POLYGON((
        -1.6200 53.3200,
        -1.6200 53.4300,
        -1.4000 53.4300,
        -1.4000 53.3200,
        -1.6200 53.3200
    ))', 4326)
);

-- Verify the city was inserted
SELECT id, name, country_code, ST_Area(ST_Transform(boundary, 3857)) / 1000000 AS area_km2 
FROM cities;

EOF

echo "Sheffield city boundary added successfully!"