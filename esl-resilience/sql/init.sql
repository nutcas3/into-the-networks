-- Initialize FreeSWITCH CDR database
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create CDR table
CREATE TABLE IF NOT EXISTS cdr (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    caller_id_number VARCHAR(64),
    destination_number VARCHAR(64),
    start_timestamp TIMESTAMP,
    answer_timestamp TIMESTAMP,
    end_timestamp TIMESTAMP,
    duration_seconds INTEGER,
    billsec_seconds INTEGER,
    hangup_cause VARCHAR(64),
    channel_uuid VARCHAR(64),
    destination_channel_uuid VARCHAR(64),
    context VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index for performance
CREATE INDEX IF NOT EXISTS idx_cdr_start_timestamp ON cdr(start_timestamp);
CREATE INDEX IF NOT EXISTS idx_cdr_caller_id_number ON cdr(caller_id_number);
CREATE INDEX IF NOT EXISTS idx_cdr_destination_number ON cdr(destination_number);
CREATE INDEX IF NOT EXISTS idx_cdr_channel_uuid ON cdr(channel_uuid);

-- Create table for call metadata
CREATE TABLE IF NOT EXISTS call_metadata (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cdr_id UUID REFERENCES cdr(id) ON DELETE CASCADE,
    key VARCHAR(128),
    value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_call_metadata_cdr_id ON call_metadata(cdr_id);
CREATE INDEX IF NOT EXISTS idx_call_metadata_key ON call_metadata(key);
