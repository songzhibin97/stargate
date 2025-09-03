-- Migration: Remove initial data
-- Version: 000002
-- Description: Remove default admin user and sample data

-- Remove sample credential
DELETE FROM credentials WHERE id = 'cred-001';

-- Remove sample application
DELETE FROM applications WHERE id = 'app-001';

-- Remove sample users
DELETE FROM users WHERE id IN ('admin-001', 'dev-001', 'viewer-001');
