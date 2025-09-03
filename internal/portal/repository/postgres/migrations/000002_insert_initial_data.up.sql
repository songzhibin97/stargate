-- Migration: Insert initial data
-- Version: 000002
-- Description: Insert default admin user and sample data for development

-- Insert default admin user (for development/testing)
INSERT INTO users (id, email, name, role, status, created_at, updated_at) 
VALUES (
    'admin-001', 
    'admin@stargate.local', 
    'System Administrator', 
    'admin', 
    'active',
    NOW(),
    NOW()
) ON CONFLICT (id) DO NOTHING;

-- Insert sample developer user for testing
INSERT INTO users (id, email, name, role, status, created_at, updated_at) 
VALUES (
    'dev-001', 
    'developer@stargate.local', 
    'Sample Developer', 
    'developer', 
    'active',
    NOW(),
    NOW()
) ON CONFLICT (id) DO NOTHING;

-- Insert sample viewer user for testing
INSERT INTO users (id, email, name, role, status, created_at, updated_at) 
VALUES (
    'viewer-001', 
    'viewer@stargate.local', 
    'Sample Viewer', 
    'viewer', 
    'active',
    NOW(),
    NOW()
) ON CONFLICT (id) DO NOTHING;

-- Insert sample application for the developer user
INSERT INTO applications (id, name, description, user_id, api_key, api_secret, status, rate_limit, created_at, updated_at)
VALUES (
    'app-001',
    'Sample Application',
    'A sample application for testing purposes',
    'dev-001',
    'ak_sample_key_12345678901234567890',
    'as_sample_secret_12345678901234567890abcdef',
    'active',
    1000,
    NOW(),
    NOW()
) ON CONFLICT (id) DO NOTHING;

-- Insert corresponding credential record for the sample application
INSERT INTO credentials (id, application_id, credential_type, credential_value, is_active, created_at, updated_at)
VALUES (
    'cred-001',
    'app-001',
    'api_key',
    'ak_sample_key_12345678901234567890',
    true,
    NOW(),
    NOW()
) ON CONFLICT (id) DO NOTHING;
