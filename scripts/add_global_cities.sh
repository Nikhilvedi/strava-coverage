#!/bin/bash

# Exit on error
set -e

echo "Adding major cities worldwide for Strava Coverage..."

# Connect to the database and insert major city boundaries
psql "postgres://nikhilvediair@localhost:5432/strava_coverage" << 'EOF'

-- Clear existing cities (optional - remove this line if you want to keep Sheffield)
-- DELETE FROM cities;

-- Major UK Cities
INSERT INTO cities (name, country_code, boundary) VALUES 
('London', 'GB', ST_GeomFromText('POLYGON((
    -0.4800 51.2800,
    -0.4800 51.7000,
    0.2000 51.7000,
    0.2000 51.2800,
    -0.4800 51.2800
))', 4326)),

('Manchester', 'GB', ST_GeomFromText('POLYGON((
    -2.3500 53.4000,
    -2.3500 53.5500,
    -2.1500 53.5500,
    -2.1500 53.4000,
    -2.3500 53.4000
))', 4326)),

('Birmingham', 'GB', ST_GeomFromText('POLYGON((
    -2.0000 52.4000,
    -2.0000 52.6000,
    -1.7500 52.6000,
    -1.7500 52.4000,
    -2.0000 52.4000
))', 4326)),

('Edinburgh', 'GB', ST_GeomFromText('POLYGON((
    -3.3500 55.9000,
    -3.3500 56.0000,
    -3.1000 56.0000,
    -3.1000 55.9000,
    -3.3500 55.9000
))', 4326)),

('Glasgow', 'GB', ST_GeomFromText('POLYGON((
    -4.3500 55.8000,
    -4.3500 55.9500,
    -4.1500 55.9500,
    -4.1500 55.8000,
    -4.3500 55.8000
))', 4326));

-- Major US Cities
INSERT INTO cities (name, country_code, boundary) VALUES 
('New York', 'US', ST_GeomFromText('POLYGON((
    -74.2000 40.4500,
    -74.2000 40.9500,
    -73.7000 40.9500,
    -73.7000 40.4500,
    -74.2000 40.4500
))', 4326)),

('San Francisco', 'US', ST_GeomFromText('POLYGON((
    -122.5500 37.7000,
    -122.5500 37.8500,
    -122.3500 37.8500,
    -122.3500 37.7000,
    -122.5500 37.7000
))', 4326)),

('Los Angeles', 'US', ST_GeomFromText('POLYGON((
    -118.7000 33.9000,
    -118.7000 34.3500,
    -118.1500 34.3500,
    -118.1500 33.9000,
    -118.7000 33.9000
))', 4326)),

('Chicago', 'US', ST_GeomFromText('POLYGON((
    -87.9500 41.6000,
    -87.9500 42.1000,
    -87.5000 42.1000,
    -87.5000 41.6000,
    -87.9500 41.6000
))', 4326)),

('Boston', 'US', ST_GeomFromText('POLYGON((
    -71.2000 42.2500,
    -71.2000 42.4500,
    -70.9500 42.4500,
    -70.9500 42.2500,
    -71.2000 42.2500
))', 4326));

-- Major European Cities
INSERT INTO cities (name, country_code, boundary) VALUES 
('Paris', 'FR', ST_GeomFromText('POLYGON((
    2.2000 48.8000,
    2.2000 48.9500,
    2.4500 48.9500,
    2.4500 48.8000,
    2.2000 48.8000
))', 4326)),

('Berlin', 'DE', ST_GeomFromText('POLYGON((
    13.2000 52.4000,
    13.2000 52.6500,
    13.5500 52.6500,
    13.5500 52.4000,
    13.2000 52.4000
))', 4326)),

('Amsterdam', 'NL', ST_GeomFromText('POLYGON((
    4.8000 52.3000,
    4.8000 52.4000,
    5.0000 52.4000,
    5.0000 52.3000,
    4.8000 52.3000
))', 4326)),

('Rome', 'IT', ST_GeomFromText('POLYGON((
    12.4000 41.8000,
    12.4000 41.9500,
    12.6000 41.9500,
    12.6000 41.8000,
    12.4000 41.8000
))', 4326)),

('Barcelona', 'ES', ST_GeomFromText('POLYGON((
    2.0500 41.3000,
    2.0500 41.4500,
    2.2500 41.4500,
    2.2500 41.3000,
    2.0500 41.3000
))', 4326));

-- Major Australian Cities
INSERT INTO cities (name, country_code, boundary) VALUES 
('Sydney', 'AU', ST_GeomFromText('POLYGON((
    150.9000 -34.0000,
    150.9000 -33.7000,
    151.3000 -33.7000,
    151.3000 -34.0000,
    150.9000 -34.0000
))', 4326)),

('Melbourne', 'AU', ST_GeomFromText('POLYGON((
    144.8000 -37.9500,
    144.8000 -37.6500,
    145.1000 -37.6500,
    145.1000 -37.9500,
    144.8000 -37.9500
))', 4326));

-- Major Canadian Cities
INSERT INTO cities (name, country_code, boundary) VALUES 
('Toronto', 'CA', ST_GeomFromText('POLYGON((
    -79.7000 43.5500,
    -79.7000 43.8500,
    -79.2000 43.8500,
    -79.2000 43.5500,
    -79.7000 43.5500
))', 4326)),

('Vancouver', 'CA', ST_GeomFromText('POLYGON((
    -123.3000 49.2000,
    -123.3000 49.3500,
    -123.0000 49.3500,
    -123.0000 49.2000,
    -123.3000 49.2000
))', 4326));

-- Major Asian Cities
INSERT INTO cities (name, country_code, boundary) VALUES 
('Tokyo', 'JP', ST_GeomFromText('POLYGON((
    139.6000 35.6000,
    139.6000 35.8000,
    139.8500 35.8000,
    139.8500 35.6000,
    139.6000 35.6000
))', 4326)),

('Singapore', 'SG', ST_GeomFromText('POLYGON((
    103.6000 1.2500,
    103.6000 1.4500,
    103.9500 1.4500,
    103.9500 1.2500,
    103.6000 1.2500
))', 4326));

-- Display results
SELECT 
    id, 
    name, 
    country_code,
    ST_Area(ST_Transform(boundary, 3857)) / 1000000 AS area_km2 
FROM cities 
ORDER BY country_code, name;

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_cities_country ON cities(country_code);
CREATE INDEX IF NOT EXISTS idx_cities_name_country ON cities(name, country_code);

EOF

echo "Cities added successfully! The system now supports major cities worldwide."