-- Allow NULL values for activity paths to handle activities without GPS data
-- (e.g., treadmill runs, strength training, indoor activities)

-- Remove NOT NULL constraint from path column
ALTER TABLE activities ALTER COLUMN path DROP NOT NULL;

-- Update the comment to reflect that path can now be NULL
COMMENT ON COLUMN activities.path IS 'Activity path as LineString geometry. NULL for activities without GPS data (indoor activities, treadmill runs, etc.)';