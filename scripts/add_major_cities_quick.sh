#!/bin/bash

# Quick fix: Add more cities manually for testing
# This creates a reasonable set of major world cities for coverage testing

set -e

DB_URL=${DB_URL:-"postgres://strava_user:a_strong_password@localhost:5432/strava_coverage?sslmode=disable"}

echo "🏙️  Adding major world cities for testing..."

source .env

# Add major cities with approximate boundaries (circular areas)
psql "$DB_URL" << 'SQL'

-- First, let's see what we have
SELECT COUNT(*) as current_cities FROM cities;

-- Add major world cities with circular boundaries
INSERT INTO cities (name, country_code, boundary) VALUES
-- UK Cities (more comprehensive)
('London', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-0.1278, 51.5074), 4326), 3857), 25000), 4326)),
('Manchester', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-2.2426, 53.4808), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Birmingham', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.8904, 52.4862), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Leeds', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.5491, 53.8008), 4326), 3857), 12000)::geometry(Polygon, 4326)),
('Glasgow', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-4.2518, 55.8642), 4326), 3857), 12000)::geometry(Polygon, 4326)),
('Liverpool', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-2.9916, 53.4084), 4326), 3857), 10000)::geometry(Polygon, 4326)),
('Bristol', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-2.5879, 51.4545), 4326), 3857), 10000)::geometry(Polygon, 4326)),
('Edinburgh', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-3.1883, 55.9533), 4326), 3857), 10000)::geometry(Polygon, 4326)),
('Leicester', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.1398, 52.6369), 4326), 3857), 8000)::geometry(Polygon, 4326)),
('Coventry', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.5128, 52.4068), 4326), 3857), 8000)::geometry(Polygon, 4326)),
('Hull', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-0.3274, 53.7676), 4326), 3857), 7000)::geometry(Polygon, 4326)),
('Bradford', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.7594, 53.7960), 4326), 3857), 7000)::geometry(Polygon, 4326)),
('Cardiff', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-3.1791, 51.4816), 4326), 3857), 8000)::geometry(Polygon, 4326)),
('Belfast', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-5.9301, 54.5973), 4326), 3857), 8000)::geometry(Polygon, 4326)),
('Nottingham', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.1581, 52.9548), 4326), 3857), 7000)::geometry(Polygon, 4326)),
('Newcastle', 'GB', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.6131, 54.9783), 4326), 3857), 7000)::geometry(Polygon, 4326)),

-- Major European Cities  
('Paris', 'FR', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(2.3522, 48.8566), 4326), 3857), 25000)::geometry(Polygon, 4326)),
('Berlin', 'DE', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(13.4050, 52.5200), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('Madrid', 'ES', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-3.7038, 40.4168), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('Rome', 'IT', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(12.4964, 41.9028), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('Amsterdam', 'NL', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(4.9041, 52.3676), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Barcelona', 'ES', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(2.1734, 41.3851), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Munich', 'DE', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(11.5820, 48.1351), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Vienna', 'AT', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(16.3738, 48.2082), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Hamburg', 'DE', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(9.9937, 53.5511), 4326), 3857), 12000)::geometry(Polygon, 4326)),
('Prague', 'CZ', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(14.4378, 50.0755), 4326), 3857), 12000)::geometry(Polygon, 4326)),

-- North American Cities
('New York', 'US', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-74.0060, 40.7128), 4326), 3857), 30000)::geometry(Polygon, 4326)),
('Los Angeles', 'US', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-118.2437, 34.0522), 4326), 3857), 25000)::geometry(Polygon, 4326)),
('Chicago', 'US', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-87.6298, 41.8781), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('Toronto', 'CA', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-79.3832, 43.6532), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('San Francisco', 'US', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-122.4194, 37.7749), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Boston', 'US', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-71.0589, 42.3601), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Vancouver', 'CA', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-123.1207, 49.2827), 4326), 3857), 12000)::geometry(Polygon, 4326)),
('Montreal', 'CA', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-73.5673, 45.5017), 4326), 3857), 12000)::geometry(Polygon, 4326)),

-- Asia-Pacific Cities
('Tokyo', 'JP', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(139.6503, 35.6762), 4326), 3857), 30000)::geometry(Polygon, 4326)),
('Sydney', 'AU', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(151.2093, -33.8688), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('Melbourne', 'AU', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(144.9631, -37.8136), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('Singapore', 'SG', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(103.8198, 1.3521), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Hong Kong', 'HK', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(114.1694, 22.3193), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Seoul', 'KR', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(126.9780, 37.5665), 4326), 3857), 25000)::geometry(Polygon, 4326)),

-- Other Major Cities
('Dubai', 'AE', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(55.2708, 25.2048), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('Cape Town', 'ZA', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(18.4241, -33.9249), 4326), 3857), 15000)::geometry(Polygon, 4326)),
('São Paulo', 'BR', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-46.6333, -23.5505), 4326), 3857), 25000)::geometry(Polygon, 4326)),
('Rio de Janeiro', 'BR', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-43.1729, -22.9068), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('Buenos Aires', 'AR', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-58.3816, -34.6037), 4326), 3857), 20000)::geometry(Polygon, 4326)),
('Mexico City', 'MX', ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-99.1332, 19.4326), 4326), 3857), 25000)::geometry(Polygon, 4326))

ON CONFLICT (name, country_code) DO NOTHING;

-- Update statistics
ANALYZE cities;

-- Show results
SELECT 
    COUNT(*) as total_cities,
    COUNT(CASE WHEN country_code = 'GB' THEN 1 END) as uk_cities,
    COUNT(CASE WHEN country_code IN ('US', 'CA') THEN 1 END) as north_america,
    COUNT(CASE WHEN country_code IN ('FR', 'DE', 'ES', 'IT', 'NL', 'AT', 'CZ') THEN 1 END) as europe,
    COUNT(CASE WHEN country_code IN ('JP', 'AU', 'SG', 'HK', 'KR') THEN 1 END) as asia_pacific
FROM cities;

SELECT 'Cities by country:' as info;
SELECT country_code, COUNT(*) as city_count, array_agg(name) as cities
FROM cities 
GROUP BY country_code 
ORDER BY city_count DESC;

SQL

echo "✅ Added major cities for comprehensive coverage testing!"
echo ""
echo "🎯 Now your imported activities should find matching cities"
echo "📍 Coverage includes major UK cities where your activities likely occurred"