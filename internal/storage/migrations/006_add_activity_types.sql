-- Add activity type fields to activities table
ALTER TABLE activities ADD COLUMN IF NOT EXISTS activity_type VARCHAR(50);
ALTER TABLE activities ADD COLUMN IF NOT EXISTS sport_type VARCHAR(50);

-- Create index for activity type filtering
CREATE INDEX IF NOT EXISTS idx_activities_type ON activities(activity_type);
CREATE INDEX IF NOT EXISTS idx_activities_sport_type ON activities(sport_type);