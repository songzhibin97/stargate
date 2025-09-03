-- PostgreSQL schema for Stargate Portal
-- This file defines the database schema for users, applications, and related entities

-- Enable UUID extension for generating UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('admin', 'developer', 'viewer')),
    status VARCHAR(50) NOT NULL CHECK (status IN ('active', 'inactive', 'suspended')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for users table
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_created_at ON users(created_at);

-- Applications table
CREATE TABLE applications (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key VARCHAR(255) NOT NULL UNIQUE,
    api_secret VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('active', 'inactive', 'suspended')),
    rate_limit BIGINT NOT NULL DEFAULT 1000,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for applications table
CREATE INDEX idx_applications_user_id ON applications(user_id);
CREATE INDEX idx_applications_api_key ON applications(api_key);
CREATE INDEX idx_applications_status ON applications(status);
CREATE INDEX idx_applications_created_at ON applications(created_at);
CREATE INDEX idx_applications_name ON applications(name);

-- Credentials table (for future extensibility)
CREATE TABLE credentials (
    id VARCHAR(255) PRIMARY KEY DEFAULT uuid_generate_v4()::text,
    application_id VARCHAR(255) NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    credential_type VARCHAR(50) NOT NULL CHECK (credential_type IN ('api_key', 'oauth2', 'jwt')),
    credential_value TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for credentials table
CREATE INDEX idx_credentials_application_id ON credentials(application_id);
CREATE INDEX idx_credentials_type ON credentials(credential_type);
CREATE INDEX idx_credentials_active ON credentials(is_active);
CREATE INDEX idx_credentials_expires_at ON credentials(expires_at);

-- API usage logs table (for analytics and monitoring)
CREATE TABLE api_usage_logs (
    id BIGSERIAL PRIMARY KEY,
    application_id VARCHAR(255) NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id VARCHAR(255),
    method VARCHAR(10) NOT NULL,
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    response_time_ms BIGINT NOT NULL,
    request_size BIGINT,
    response_size BIGINT,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for api_usage_logs table
CREATE INDEX idx_api_usage_logs_application_id ON api_usage_logs(application_id);
CREATE INDEX idx_api_usage_logs_user_id ON api_usage_logs(user_id);
CREATE INDEX idx_api_usage_logs_created_at ON api_usage_logs(created_at);
CREATE INDEX idx_api_usage_logs_status_code ON api_usage_logs(status_code);
CREATE INDEX idx_api_usage_logs_method ON api_usage_logs(method);

-- Partitioning for api_usage_logs (monthly partitions)
-- This helps with performance for large datasets
CREATE TABLE api_usage_logs_template (LIKE api_usage_logs INCLUDING ALL);

-- Function to create monthly partitions
CREATE OR REPLACE FUNCTION create_monthly_partition(table_name text, start_date date)
RETURNS void AS $$
DECLARE
    partition_name text;
    end_date date;
BEGIN
    partition_name := table_name || '_' || to_char(start_date, 'YYYY_MM');
    end_date := start_date + interval '1 month';
    
    EXECUTE format('CREATE TABLE IF NOT EXISTS %I PARTITION OF %I
                    FOR VALUES FROM (%L) TO (%L)',
                   partition_name, table_name, start_date, end_date);
END;
$$ LANGUAGE plpgsql;

-- Create trigger function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for updated_at columns
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_applications_updated_at
    BEFORE UPDATE ON applications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_credentials_updated_at
    BEFORE UPDATE ON credentials
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Create views for common queries
CREATE VIEW active_users AS
SELECT * FROM users WHERE status = 'active';

CREATE VIEW active_applications AS
SELECT * FROM applications WHERE status = 'active';

CREATE VIEW user_application_summary AS
SELECT 
    u.id as user_id,
    u.name as user_name,
    u.email as user_email,
    u.role as user_role,
    COUNT(a.id) as application_count,
    COUNT(CASE WHEN a.status = 'active' THEN 1 END) as active_application_count
FROM users u
LEFT JOIN applications a ON u.id = a.user_id
GROUP BY u.id, u.name, u.email, u.role;

-- Insert default admin user (for development/testing)
INSERT INTO users (id, email, name, role, status) 
VALUES ('admin-001', 'admin@stargate.local', 'System Administrator', 'admin', 'active')
ON CONFLICT (id) DO NOTHING;

-- Comments for documentation
COMMENT ON TABLE users IS 'Portal users with role-based access control';
COMMENT ON TABLE applications IS 'Developer applications with API credentials';
COMMENT ON TABLE credentials IS 'Application credentials for various authentication methods';
COMMENT ON TABLE api_usage_logs IS 'API usage tracking for analytics and monitoring';

COMMENT ON COLUMN users.role IS 'User role: admin, developer, or viewer';
COMMENT ON COLUMN users.status IS 'User status: active, inactive, or suspended';
COMMENT ON COLUMN applications.rate_limit IS 'API rate limit per hour for this application';
COMMENT ON COLUMN credentials.credential_type IS 'Type of credential: api_key, oauth2, or jwt';
