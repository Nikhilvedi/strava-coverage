#!/bin/bash

echo "🗄️ Setting up Strava Coverage Database Schema"
echo "=============================================="

# Get the Cloud SQL instance connection
INSTANCE_CONNECTION_NAME="${GCP_PROJECT_ID}:us-central1:strava-coverage-db"

echo "📋 Step 1: Connect to Cloud SQL instance..."
echo "Instance: $INSTANCE_CONNECTION_NAME"

# You can either use cloud_sql_proxy or connect directly
echo ""
echo "🔧 Option A: Using Cloud SQL Proxy (Recommended)"
echo "================================================"
echo "1. Download and install cloud_sql_proxy:"
echo "   curl -o cloud_sql_proxy https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64"
echo "   chmod +x cloud_sql_proxy"
echo ""
echo "2. Start the proxy:"
echo "   ./cloud_sql_proxy -instances=$INSTANCE_CONNECTION_NAME=tcp:5432"
echo ""
echo "3. In another terminal, connect:"
echo "   psql 'host=127.0.0.1 port=5432 user=stravauser dbname=strava_coverage'"

echo ""
echo "🔧 Option B: Direct Connection from GCP Console"
echo "=============================================="
echo "1. Go to: https://console.cloud.google.com/sql/instances"
echo "2. Click on 'strava-coverage-db'"
echo "3. Click 'Connect using Cloud Shell'"
echo "4. Run: \\c strava_coverage"

echo ""
echo "📊 Step 2: Run these SQL commands to set up schema:"
echo "================================================="

cat << 'EOF'

-- Enable PostGIS extension
CREATE EXTENSION IF NOT EXISTS postgis;

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    strava_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(255),
    firstname VARCHAR(255),
    lastname VARCHAR(255),
    profile_url VARCHAR(255),
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create cities table
CREATE TABLE IF NOT EXISTS cities (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    country VARCHAR(255),
    boundary GEOMETRY(POLYGON, 4326),
    center_lat DECIMAL(10, 8),
    center_lng DECIMAL(11, 8),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create activities table
CREATE TABLE IF NOT EXISTS activities (
    id SERIAL PRIMARY KEY,
    strava_id BIGINT UNIQUE NOT NULL,
    user_id INTEGER REFERENCES users(id),
    name VARCHAR(255),
    activity_type VARCHAR(50),
    start_date TIMESTAMP,
    polyline TEXT,
    geometry GEOMETRY(LINESTRING, 4326),
    distance DECIMAL(10, 2),
    moving_time INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create coverage table
CREATE TABLE IF NOT EXISTS coverage (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    city_id INTEGER REFERENCES cities(id),
    coverage_percentage DECIMAL(5, 2),
    total_segments INTEGER DEFAULT 0,
    covered_segments INTEGER DEFAULT 0,
    last_calculated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, city_id)
);

-- Create custom_areas table
CREATE TABLE IF NOT EXISTS custom_areas (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    boundary GEOMETRY(POLYGON, 4326),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create recalculation_jobs table
CREATE TABLE IF NOT EXISTS recalculation_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER REFERENCES users(id),
    status VARCHAR(50) DEFAULT 'pending',
    progress INTEGER DEFAULT 0,
    total_activities INTEGER DEFAULT 0,
    processed_activities INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_activities_user_id ON activities(user_id);
CREATE INDEX IF NOT EXISTS idx_activities_strava_id ON activities(strava_id);
CREATE INDEX IF NOT EXISTS idx_coverage_user_city ON coverage(user_id, city_id);
CREATE INDEX IF NOT EXISTS idx_cities_boundary ON cities USING GIST (boundary);
CREATE INDEX IF NOT EXISTS idx_activities_geometry ON activities USING GIST (geometry);

-- Insert some default cities (optional)
INSERT INTO cities (name, country, center_lat, center_lng) VALUES 
('Sheffield', 'UK', 53.3811, -1.4701),
('London', 'UK', 51.5074, -0.1278),
('Manchester', 'UK', 53.4808, -2.2426)
ON CONFLICT DO NOTHING;

EOF

echo ""
echo "🎉 Database setup complete!"
echo ""
echo "🔍 Step 3: Verify deployment"
echo "==========================="
echo "1. Check Cloud Run service:"
echo "   gcloud run services list"
echo ""
echo "2. Test your API:"
echo "   curl https://your-service-url/api/health"
echo ""
echo "3. View logs:"
echo "   gcloud logs tail strava-coverage-backend"