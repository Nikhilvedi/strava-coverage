-- Migration to add comment settings table for auto-commenting functionality

CREATE TABLE IF NOT EXISTS comment_settings (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    running_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    cycling_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    walking_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    hiking_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ebiking_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    skiing_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    comment_template TEXT NOT NULL DEFAULT 'Your coverage of {city} is {coverage}%!',
    min_coverage_increase DECIMAL(5,2) NOT NULL DEFAULT 0.1,
    custom_areas_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Add index for faster queries
CREATE INDEX IF NOT EXISTS idx_comment_settings_user_id ON comment_settings(user_id);

-- Add commented_at column to activities table to track which activities have been commented on
ALTER TABLE activities ADD COLUMN IF NOT EXISTS commented_at TIMESTAMP WITH TIME ZONE;

-- Add index for faster queries on uncommented activities
CREATE INDEX IF NOT EXISTS idx_activities_commented_at ON activities(user_id, commented_at) WHERE commented_at IS NULL;