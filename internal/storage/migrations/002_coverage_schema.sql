-- Create cities table with geographic boundaries
CREATE TABLE IF NOT EXISTS cities (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    country_code CHAR(2) NOT NULL,
    -- PostGIS geometry column for city boundaries
    boundary GEOMETRY(MULTIPOLYGON, 4326) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create activities table to store Strava activities
CREATE TABLE IF NOT EXISTS activities (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    strava_activity_id BIGINT UNIQUE NOT NULL,
    city_id INTEGER REFERENCES cities(id),
    -- Activity path as LineString
    path GEOMETRY(LINESTRING, 4326) NOT NULL,
    -- Coverage calculation results
    coverage_percentage DECIMAL(5,2),
    comment_posted BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create spatial indexes
CREATE INDEX IF NOT EXISTS idx_cities_boundary ON cities USING GIST(boundary);
CREATE INDEX IF NOT EXISTS idx_activities_path ON activities USING GIST(path);

-- Create index for faster activity lookups
CREATE INDEX IF NOT EXISTS idx_activities_strava_id ON activities(strava_activity_id);
CREATE INDEX IF NOT EXISTS idx_activities_user_city ON activities(user_id, city_id);