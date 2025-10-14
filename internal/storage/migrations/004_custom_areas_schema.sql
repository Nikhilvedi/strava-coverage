-- Create custom_areas table
CREATE TABLE custom_areas (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    geometry GEOMETRY(POLYGON, 4326) NOT NULL,
    coverage_percentage DECIMAL(5,2),
    activities_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT fk_custom_areas_user 
        FOREIGN KEY (user_id) 
        REFERENCES users(id) 
        ON DELETE CASCADE
);

-- Create index for spatial queries
CREATE INDEX idx_custom_areas_geometry ON custom_areas USING GIST (geometry);

-- Create index for user queries
CREATE INDEX idx_custom_areas_user_id ON custom_areas (user_id);

-- Create index for performance on coverage calculations
CREATE INDEX idx_custom_areas_coverage ON custom_areas (user_id, coverage_percentage DESC);