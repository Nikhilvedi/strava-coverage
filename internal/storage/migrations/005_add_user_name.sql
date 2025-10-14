-- Add name field to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(255);

-- Create index on name for faster searches
CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);