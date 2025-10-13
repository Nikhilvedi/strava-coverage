#!/bin/bash

# Simple city import with proper SRID handling

set -e

source .env

echo "🏙️  Adding major cities for coverage testing..."

psql "$DB_URL" << 'SQL'

-- Clear existing cities and add major world cities
TRUNCATE cities RESTART IDENTITY CASCADE;

-- Add major cities with proper SRID handling  
INSERT INTO cities (name, country_code, boundary) VALUES
-- UK Cities
('Sheffield', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.4701, 53.3811), 4326), 3857), 8000), 4326)),
('London', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-0.1278, 51.5074), 4326), 3857), 25000), 4326)),
('Manchester', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-2.2426, 53.4808), 4326), 3857), 15000), 4326)),
('Birmingham', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.8904, 52.4862), 4326), 3857), 15000), 4326)),
('Leeds', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.5491, 53.8008), 4326), 3857), 12000), 4326)),
('Bristol', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-2.5879, 51.4545), 4326), 3857), 10000), 4326)),
('Liverpool', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-2.9916, 53.4084), 4326), 3857), 10000), 4326)),
('Newcastle', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.6131, 54.9783), 4326), 3857), 8000), 4326)),
('Nottingham', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-1.1581, 52.9548), 4326), 3857), 8000), 4326)),
('Glasgow', 'GB', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-4.2518, 55.8642), 4326), 3857), 12000), 4326)),

-- Major International Cities
('Paris', 'FR', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(2.3522, 48.8566), 4326), 3857), 25000), 4326)),
('Berlin', 'DE', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(13.4050, 52.5200), 4326), 3857), 20000), 4326)),
('New York', 'US', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-74.0060, 40.7128), 4326), 3857), 30000), 4326)),
('Los Angeles', 'US', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-118.2437, 34.0522), 4326), 3857), 25000), 4326)),
('Tokyo', 'JP', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(139.6503, 35.6762), 4326), 3857), 30000), 4326)),
('Sydney', 'AU', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(151.2093, -33.8688), 4326), 3857), 20000), 4326)),
('Amsterdam', 'NL', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(4.9041, 52.3676), 4326), 3857), 15000), 4326)),
('Barcelona', 'ES', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(2.1734, 41.3851), 4326), 3857), 15000), 4326)),
('Rome', 'IT', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(12.4964, 41.9028), 4326), 3857), 20000), 4326)),
('Toronto', 'CA', ST_Transform(ST_Buffer(ST_Transform(ST_SetSRID(ST_MakePoint(-79.3832, 43.6532), 4326), 3857), 20000), 4326));

-- Show results
SELECT 
    COUNT(*) as total_cities,
    COUNT(CASE WHEN country_code = 'GB' THEN 1 END) as uk_cities
FROM cities;

SELECT 'Cities added:' as status, name, country_code 
FROM cities 
ORDER BY country_code, name;

SQL

echo "✅ Added ${total_cities} cities successfully!"
echo "🎯 Now ready to test coverage calculation with imported activities"