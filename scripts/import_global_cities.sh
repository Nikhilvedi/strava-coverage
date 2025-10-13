#!/bin/bash

# Import Global Cities from Natural Earth Data
# This script downloads and imports comprehensive city data for production use

set -e

DB_URL=${DB_URL:-"postgres://strava_user:a_strong_password@localhost:5432/strava_coverage?sslmode=disable"}
WORK_DIR="/tmp/city_import"

echo "🌍 Importing global cities for production use..."

# Create working directory
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

# 1. Download Natural Earth populated places (10m = highest detail)
echo "📥 Downloading Natural Earth populated places..."
if [ ! -f "ne_10m_populated_places.zip" ]; then
    curl -L -o ne_10m_populated_places.zip "https://naciscdn.org/naturalearth/10m/cultural/ne_10m_populated_places.zip"
fi

if [ ! -f "ne_10m_populated_places.shp" ]; then
    unzip -q ne_10m_populated_places.zip
fi

# 2. Download Natural Earth urban areas (actual city boundaries, not just points)
echo "📥 Downloading Natural Earth urban areas..."
if [ ! -f "ne_10m_urban_areas.zip" ]; then
    curl -L -o ne_10m_urban_areas.zip "https://naciscdn.org/naturalearth/10m/cultural/ne_10m_urban_areas.zip"
fi

if [ ! -f "ne_10m_urban_areas.shp" ]; then
    unzip -q ne_10m_urban_areas.zip
fi

# 3. Create enhanced cities table with global coverage
echo "🗄️  Creating global cities table..."
psql "$DB_URL" << 'SQL'
-- Create enhanced cities table for global coverage
DROP TABLE IF EXISTS cities_global CASCADE;

CREATE TABLE cities_global (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    name_en VARCHAR(255), -- English name
    country_code CHAR(2) NOT NULL,
    country_name VARCHAR(100),
    region VARCHAR(100),
    population BIGINT DEFAULT 0,
    boundary GEOMETRY(POLYGON, 4326),
    center_point GEOMETRY(POINT, 4326),
    area_km2 DECIMAL(10,2),
    timezone VARCHAR(50),
    elevation_m INTEGER,
    wikidata_id VARCHAR(20),
    source VARCHAR(20) DEFAULT 'natural_earth',
    min_zoom_level INTEGER DEFAULT 10, -- For map display optimization
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create spatial indexes for performance
CREATE INDEX idx_cities_global_boundary ON cities_global USING GIST (boundary);
CREATE INDEX idx_cities_global_center ON cities_global USING GIST (center_point);
CREATE INDEX idx_cities_global_country ON cities_global (country_code);
CREATE INDEX idx_cities_global_population ON cities_global (population DESC);
CREATE INDEX idx_cities_global_name ON cities_global (name);

-- Add constraints
ALTER TABLE cities_global ADD CONSTRAINT chk_population CHECK (population >= 0);
ALTER TABLE cities_global ADD CONSTRAINT chk_area CHECK (area_km2 >= 0);
SQL

# 4. Import populated places (city centers and metadata)
echo "📍 Importing populated places..."
psql "$DB_URL" << 'SQL'
-- Create temporary table for populated places
CREATE TEMP TABLE temp_places (
    name VARCHAR(255),
    name_en VARCHAR(255),
    country_code CHAR(2),
    admin1_name VARCHAR(100),
    population BIGINT,
    timezone VARCHAR(50),
    elevation INTEGER,
    wikidata VARCHAR(20),
    geom GEOMETRY(POINT, 4326)
);
SQL

# Import shapefile data
shp2pgsql -a -s 4326 -W UTF-8 ne_10m_populated_places.shp temp_places | psql "$DB_URL" > /dev/null

# 5. Import urban area boundaries
echo "🏙️  Importing urban boundaries..."
psql "$DB_URL" << 'SQL'
-- Create temporary table for urban areas
CREATE TEMP TABLE temp_urban (
    name VARCHAR(255),
    geom GEOMETRY(MULTIPOLYGON, 4326)
);
SQL

shp2pgsql -a -s 4326 -W UTF-8 ne_10m_urban_areas.shp temp_urban | psql "$DB_URL" > /dev/null

# 6. Combine and process the data
echo "🔄 Processing and combining city data..."
psql "$DB_URL" << 'SQL'
-- Insert cities with boundaries (from urban areas)
INSERT INTO cities_global (
    name, name_en, country_code, population, boundary, center_point, 
    area_km2, timezone, elevation_m, wikidata_id, source, min_zoom_level
)
SELECT DISTINCT
    COALESCE(u.name, p.name) as name,
    p.name_en,
    p.country_code,
    COALESCE(p.population, 0) as population,
    ST_GeometryN(u.geom, 1)::GEOMETRY(POLYGON, 4326) as boundary,
    COALESCE(p.geom, ST_Centroid(u.geom)) as center_point,
    ST_Area(ST_Transform(u.geom, 3857)) / 1000000 as area_km2, -- Convert to km²
    p.timezone,
    p.elevation as elevation_m,
    p.wikidata as wikidata_id,
    'natural_earth_urban' as source,
    CASE 
        WHEN COALESCE(p.population, 0) > 5000000 THEN 6  -- Major cities visible from zoom 6
        WHEN COALESCE(p.population, 0) > 1000000 THEN 8  -- Large cities from zoom 8
        WHEN COALESCE(p.population, 0) > 100000 THEN 10  -- Medium cities from zoom 10
        ELSE 12  -- Small cities from zoom 12
    END as min_zoom_level
FROM temp_urban u
LEFT JOIN temp_places p ON ST_DWithin(p.geom, ST_Centroid(u.geom), 0.1) -- 0.1 degrees ≈ 11km
WHERE u.geom IS NOT NULL 
AND ST_GeometryType(u.geom) IN ('ST_Polygon', 'ST_MultiPolygon');

-- Insert remaining cities without boundaries (as circular areas around point)
INSERT INTO cities_global (
    name, name_en, country_code, population, boundary, center_point,
    area_km2, timezone, elevation_m, wikidata_id, source, min_zoom_level
)
SELECT 
    p.name,
    p.name_en,
    p.country_code,
    p.population,
    ST_Buffer(
        ST_Transform(p.geom, 3857), 
        CASE 
            WHEN p.population > 5000000 THEN 25000  -- 25km radius for mega cities
            WHEN p.population > 1000000 THEN 15000  -- 15km radius for large cities
            WHEN p.population > 100000 THEN 8000    -- 8km radius for medium cities
            WHEN p.population > 50000 THEN 5000     -- 5km radius for small cities
            ELSE 3000  -- 3km radius for towns
        END
    )::GEOMETRY(POLYGON, 4326) as boundary,
    p.geom as center_point,
    PI() * POWER(
        CASE 
            WHEN p.population > 5000000 THEN 25
            WHEN p.population > 1000000 THEN 15
            WHEN p.population > 100000 THEN 8
            WHEN p.population > 50000 THEN 5
            ELSE 3
        END, 2
    ) as area_km2,
    p.timezone,
    p.elevation,
    p.wikidata,
    'natural_earth_point' as source,
    CASE 
        WHEN p.population > 5000000 THEN 6
        WHEN p.population > 1000000 THEN 8
        WHEN p.population > 100000 THEN 10
        ELSE 12
    END as min_zoom_level
FROM temp_places p
WHERE p.geom IS NOT NULL
AND p.population > 50000  -- Only include cities with significant population
AND NOT EXISTS (
    SELECT 1 FROM cities_global cg 
    WHERE ST_DWithin(cg.center_point, p.geom, 0.05)  -- Avoid duplicates
);

-- Update statistics
ANALYZE cities_global;

-- Show import results
SELECT 
    source,
    COUNT(*) as city_count,
    AVG(population) as avg_population,
    SUM(CASE WHEN population > 1000000 THEN 1 ELSE 0 END) as major_cities
FROM cities_global 
GROUP BY source
ORDER BY city_count DESC;

SELECT 
    'Total cities imported' as metric,
    COUNT(*) as value
FROM cities_global;
SQL

# 7. Update existing activities to use new city data
echo "🔄 Updating existing activities with global city data..."
psql "$DB_URL" << 'SQL'
-- Backup existing cities table
CREATE TABLE cities_backup AS SELECT * FROM cities;

-- Replace cities table with global data
DROP TABLE cities CASCADE;
ALTER TABLE cities_global RENAME TO cities;

-- Update activity city assignments
UPDATE activities 
SET city_id = (
    SELECT c.id 
    FROM cities c 
    WHERE ST_Intersects(activities.path, c.boundary)
    ORDER BY ST_Length(ST_Intersection(activities.path, c.boundary)) DESC
    LIMIT 1
),
updated_at = CURRENT_TIMESTAMP
WHERE city_id IS NULL OR city_id NOT IN (SELECT id FROM cities);
SQL

echo "✅ Global cities import completed!"
echo "📊 Summary:"
psql "$DB_URL" -c "
SELECT 
    COUNT(*) as total_cities,
    COUNT(CASE WHEN population > 1000000 THEN 1 END) as major_cities,
    COUNT(CASE WHEN source LIKE '%urban%' THEN 1 END) as with_boundaries,
    COUNT(DISTINCT country_code) as countries_covered
FROM cities;"

echo ""
echo "🗺️  Top cities by population:"
psql "$DB_URL" -c "
SELECT name, country_code, population, ROUND(area_km2, 1) as area_km2
FROM cities 
WHERE population > 0
ORDER BY population DESC 
LIMIT 10;"

# Cleanup
cd /
rm -rf "$WORK_DIR"

echo ""
echo "🎉 Ready for global Strava coverage tracking!"
echo "💡 You now have comprehensive city coverage for production use"