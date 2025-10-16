-- Add missing activity columns for auto-processor
ALTER TABLE activities ADD COLUMN IF NOT EXISTS name VARCHAR(255);
ALTER TABLE activities ADD COLUMN IF NOT EXISTS distance_km DECIMAL(10,2);
ALTER TABLE activities ADD COLUMN IF NOT EXISTS moving_time_seconds INTEGER;
ALTER TABLE activities ADD COLUMN IF NOT EXISTS elapsed_time_seconds INTEGER;
ALTER TABLE activities ADD COLUMN IF NOT EXISTS total_elevation_gain_m DECIMAL(10,2);
ALTER TABLE activities ADD COLUMN IF NOT EXISTS start_time TIMESTAMP WITH TIME ZONE;
ALTER TABLE activities ADD COLUMN IF NOT EXISTS timezone VARCHAR(100);
ALTER TABLE activities ADD COLUMN IF NOT EXISTS polyline TEXT;
ALTER TABLE activities ADD COLUMN IF NOT EXISTS start_latitude DECIMAL(10,7);
ALTER TABLE activities ADD COLUMN IF NOT EXISTS start_longitude DECIMAL(10,7);
ALTER TABLE activities ADD COLUMN IF NOT EXISTS end_latitude DECIMAL(10,7);
ALTER TABLE activities ADD COLUMN IF NOT EXISTS end_longitude DECIMAL(10,7);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_activities_start_time ON activities(start_time);
CREATE INDEX IF NOT EXISTS idx_activities_distance ON activities(distance_km);
CREATE INDEX IF NOT EXISTS idx_activities_start_coords ON activities(start_latitude, start_longitude);