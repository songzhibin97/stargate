-- Migration: Drop portal tables
-- Version: 000001
-- Description: Drop all portal-related tables and functions

-- Drop views first (they depend on tables)
DROP VIEW IF EXISTS user_application_summary;
DROP VIEW IF EXISTS active_applications;
DROP VIEW IF EXISTS active_users;

-- Drop triggers
DROP TRIGGER IF EXISTS update_credentials_updated_at ON credentials;
DROP TRIGGER IF EXISTS update_applications_updated_at ON applications;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS api_usage_logs;
DROP TABLE IF EXISTS credentials;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS users;

-- Drop extension (only if no other tables use it)
-- Note: We don't drop the uuid-ossp extension as other parts of the system might use it
-- DROP EXTENSION IF EXISTS "uuid-ossp";
