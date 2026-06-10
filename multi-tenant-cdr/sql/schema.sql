-- Multi-Tenant CDR System Database Schema
-- This schema builds upon the ESL Resilience foundation

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_partman";

-- Enhanced tenant table with additional fields
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    domain VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    config JSONB NOT NULL DEFAULT '{}',
    billing_config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for tenant table
CREATE INDEX IF NOT EXISTS idx_tenants_name ON tenants(name);
CREATE INDEX IF NOT EXISTS idx_tenants_domain ON tenants(domain);
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
CREATE INDEX IF NOT EXISTS idx_tenants_last_active ON tenants(last_active);

-- Trigger for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tenants_updated_at 
    BEFORE UPDATE ON tenants 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Partitioned CDR table (time-based partitioning)
CREATE TABLE IF NOT EXISTS cdr (
    id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    account_code VARCHAR(255),
    caller_id_name VARCHAR(255),
    caller_id_number VARCHAR(255),
    destination_number VARCHAR(255),
    start_timestamp TIMESTAMP NOT NULL,
    answer_timestamp TIMESTAMP,
    end_timestamp TIMESTAMP,
    duration_seconds INTEGER,
    billsec_seconds INTEGER,
    hangup_cause VARCHAR(64),
    channel_uuid UUID NOT NULL,
    destination_channel_uuid UUID,
    context VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    enriched_data JSONB DEFAULT '{}',
    cost NUMERIC(10,4) DEFAULT 0,
    quality_score NUMERIC(3,2),
    PRIMARY KEY (id, start_timestamp)
) PARTITION BY RANGE (start_timestamp);

-- Create monthly partitions
CREATE TABLE cdr_2024_01 PARTITION OF cdr
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

CREATE TABLE cdr_2024_02 PARTITION OF cdr
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');

CREATE TABLE cdr_2024_03 PARTITION OF cdr
    FOR VALUES FROM ('2024-03-01') TO ('2024-04-01');

CREATE TABLE cdr_2024_04 PARTITION OF cdr
    FOR VALUES FROM ('2024-04-01') TO ('2024-05-01');

CREATE TABLE cdr_2024_05 PARTITION OF cdr
    FOR VALUES FROM ('2024-05-01') TO ('2024-06-01');

CREATE TABLE cdr_2024_06 PARTITION OF cdr
    FOR VALUES FROM ('2024-06-01') TO ('2024-07-01');

-- Default partition for future data
CREATE TABLE cdr_default PARTITION OF cdr
    DEFAULT;

-- Create indexes for CDR table
CREATE INDEX IF NOT EXISTS idx_cdr_tenant_id ON cdr(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cdr_caller_id_number ON cdr(caller_id_number);
CREATE INDEX IF NOT EXISTS idx_cdr_destination_number ON cdr(destination_number);
CREATE INDEX IF NOT EXISTS idx_cdr_channel_uuid ON cdr(channel_uuid);
CREATE INDEX IF NOT EXISTS idx_cdr_hangup_cause ON cdr(hangup_cause);
CREATE INDEX IF NOT EXISTS idx_cdr_tenant_start ON cdr(tenant_id, start_timestamp);

-- CDR metadata table
CREATE TABLE IF NOT EXISTS cdr_metadata (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cdr_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    key VARCHAR(128),
    value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cdr_id, key)
);

CREATE INDEX IF NOT EXISTS idx_cdr_metadata_cdr_id ON cdr_metadata(cdr_id);
CREATE INDEX IF NOT EXISTS idx_cdr_metadata_tenant_id ON cdr_metadata(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cdr_metadata_key ON cdr_metadata(key);

-- Daily analytics aggregation table
CREATE TABLE IF NOT EXISTS cdr_analytics_daily (
    tenant_id UUID NOT NULL,
    date DATE NOT NULL,
    total_calls INTEGER DEFAULT 0,
    successful_calls INTEGER DEFAULT 0,
    failed_calls INTEGER DEFAULT 0,
    avg_duration_seconds NUMERIC,
    total_duration_seconds NUMERIC DEFAULT 0,
    total_cost NUMERIC DEFAULT 0,
    peak_hour INTEGER,
    peak_hour_calls INTEGER DEFAULT 0,
    unique_callers INTEGER DEFAULT 0,
    unique_destinations INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, date)
);

CREATE INDEX IF NOT EXISTS idx_analytics_daily_date ON cdr_analytics_daily(date);
CREATE INDEX IF NOT EXISTS idx_analytics_daily_tenant ON cdr_analytics_daily(tenant_id);

CREATE TRIGGER update_analytics_daily_updated_at 
    BEFORE UPDATE ON cdr_analytics_daily 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Hourly analytics aggregation table
CREATE TABLE IF NOT EXISTS cdr_analytics_hourly (
    tenant_id UUID NOT NULL,
    date DATE NOT NULL,
    hour INTEGER NOT NULL,
    total_calls INTEGER DEFAULT 0,
    successful_calls INTEGER DEFAULT 0,
    failed_calls INTEGER DEFAULT 0,
    avg_duration_seconds NUMERIC,
    total_cost NUMERIC DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, date, hour)
);

CREATE INDEX IF NOT EXISTS idx_analytics_hourly_datetime ON cdr_analytics_hourly(date, hour);

CREATE TRIGGER update_analytics_hourly_updated_at 
    BEFORE UPDATE ON cdr_analytics_hourly 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Report templates table
CREATE TABLE IF NOT EXISTS report_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    template_type VARCHAR(50) NOT NULL, -- cdr_summary, billing, quality, custom
    template_config JSONB NOT NULL,
    schedule_config JSONB,
    output_format VARCHAR(20) DEFAULT 'pdf', -- pdf, excel, csv
    created_by VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_report_templates_tenant ON report_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_report_templates_type ON report_templates(template_type);

CREATE TRIGGER update_report_templates_updated_at 
    BEFORE UPDATE ON report_templates 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Generated reports table
CREATE TABLE IF NOT EXISTS generated_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    template_id UUID REFERENCES report_templates(id),
    report_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending', -- pending, generating, completed, failed
    parameters JSONB,
    file_path VARCHAR(512),
    file_size BIGINT,
    generated_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_generated_reports_tenant ON generated_reports(tenant_id);
CREATE INDEX IF NOT EXISTS idx_generated_reports_status ON generated_reports(status);
CREATE INDEX IF NOT EXISTS idx_generated_reports_template ON generated_reports(template_id);

-- User roles and permissions table
CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    tenant_id UUID REFERENCES tenants(id),
    role VARCHAR(50) NOT NULL, -- super_admin, tenant_admin, analyst, billing, support
    permissions JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_tenant ON user_roles(tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role ON user_roles(role);

CREATE TRIGGER update_user_roles_updated_at 
    BEFORE UPDATE ON user_roles 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- API audit log table
CREATE TABLE IF NOT EXISTS api_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID,
    user_id UUID,
    request_id VARCHAR(255),
    method VARCHAR(10),
    path VARCHAR(512),
    query_params TEXT,
    request_body TEXT,
    response_status INTEGER,
    response_time_ms INTEGER,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_log_tenant ON api_audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_user ON api_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON api_audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_request ON api_audit_log(request_id);

-- Data enrichment cache table
CREATE TABLE IF NOT EXISTS enrichment_cache (
    phone_number VARCHAR(20) PRIMARY KEY,
    carrier VARCHAR(255),
    country VARCHAR(100),
    region VARCHAR(100),
    city VARCHAR(100),
    line_type VARCHAR(50), -- mobile, landline, voip, toll_free
    is_valid BOOLEAN DEFAULT true,
    enriched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_enrichment_cache_expires ON enrichment_cache(expires_at);

-- Function to create monthly partitions automatically
CREATE OR REPLACE FUNCTION create_monthly_partition()
RETURNS TRIGGER AS $$
DECLARE
    partition_name TEXT;
    start_date TEXT;
    end_date TEXT;
BEGIN
    -- Calculate partition names based on the date
    partition_name := 'cdr_' || TO_CHAR(NEW.start_timestamp, 'YYYY_MM');
    start_date := date_trunc('month', NEW.start_timestamp);
    end_date := start_date + INTERVAL '1 month';
    
    -- Check if partition exists, create if not
    IF NOT EXISTS (
        SELECT 1 FROM pg_class 
        WHERE relname = partition_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF cdr FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
        
        -- Create indexes for the new partition
        EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_tenant_id ON %I(tenant_id)', partition_name, partition_name);
        EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_start_timestamp ON %I(start_timestamp)', partition_name, partition_name);
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-create partitions
CREATE TRIGGER trigger_auto_create_partition
    BEFORE INSERT ON cdr
    FOR EACH ROW
    EXECUTE FUNCTION create_monthly_partition();

-- Function to update daily analytics
CREATE OR REPLACE FUNCTION update_daily_analytics()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO cdr_analytics_daily (
        tenant_id, date, total_calls, successful_calls, failed_calls,
        total_duration_seconds, total_cost, unique_callers, unique_destinations
    ) VALUES (
        NEW.tenant_id,
        DATE(NEW.start_timestamp),
        1,
        CASE WHEN NEW.hangup_cause = 'NORMAL_CLEARING' THEN 1 ELSE 0 END,
        CASE WHEN NEW.hangup_cause != 'NORMAL_CLEARING' THEN 1 ELSE 0 END,
        COALESCE(NEW.duration_seconds, 0),
        COALESCE(NEW.cost, 0),
        1, -- unique_callers (simplified)
        1  -- unique_destinations (simplified)
    )
    ON CONFLICT (tenant_id, date) DO UPDATE SET
        total_calls = cdr_analytics_daily.total_calls + 1,
        successful_calls = cdr_analytics_daily.successful_calls + 
            CASE WHEN NEW.hangup_cause = 'NORMAL_CLEARING' THEN 1 ELSE 0 END,
        failed_calls = cdr_analytics_daily.failed_calls + 
            CASE WHEN NEW.hangup_cause != 'NORMAL_CLEARING' THEN 1 ELSE 0 END,
        total_duration_seconds = cdr_analytics_daily.total_duration_seconds + COALESCE(NEW.duration_seconds, 0),
        total_cost = cdr_analytics_daily.total_cost + COALESCE(NEW.cost, 0),
        updated_at = CURRENT_TIMESTAMP;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update analytics on CDR insert
CREATE TRIGGER trigger_update_analytics
    AFTER INSERT ON cdr
    FOR EACH ROW
    EXECUTE FUNCTION update_daily_analytics();
