-- Migration for initial import status tracking
-- Run date: 2025-01-13

-- Create import_status table to track bulk imports
CREATE TABLE IF NOT EXISTS import_status (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    total_activities INTEGER DEFAULT 0,
    imported_count INTEGER DEFAULT 0,
    processed_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    last_import_time TIMESTAMP WITH TIME ZONE,
    in_progress BOOLEAN DEFAULT false,
    current_page INTEGER DEFAULT 1,
    estimated_remaining INTEGER DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    UNIQUE(user_id),  -- One import status per user
    CHECK (imported_count >= 0),
    CHECK (processed_count >= 0),
    CHECK (failed_count >= 0),
    CHECK (current_page >= 1)
);

-- Create index for efficient queries
CREATE INDEX IF NOT EXISTS idx_import_status_user_id ON import_status(user_id);
CREATE INDEX IF NOT EXISTS idx_import_status_in_progress ON import_status(in_progress) WHERE in_progress = true;

-- Add comment for documentation
COMMENT ON TABLE import_status IS 'Tracks the status of bulk activity imports from Strava for each user';
COMMENT ON COLUMN import_status.user_id IS 'References the user performing the import';
COMMENT ON COLUMN import_status.total_activities IS 'Total number of activities discovered on Strava';
COMMENT ON COLUMN import_status.imported_count IS 'Number of activities imported to database';
COMMENT ON COLUMN import_status.processed_count IS 'Number of activities that have had coverage calculated';
COMMENT ON COLUMN import_status.failed_count IS 'Number of activities that failed to import';
COMMENT ON COLUMN import_status.in_progress IS 'Whether import is currently running';
COMMENT ON COLUMN import_status.current_page IS 'Current page being processed from Strava API';
COMMENT ON COLUMN import_status.estimated_remaining IS 'Estimated activities remaining to process';