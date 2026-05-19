-- Create CDR table
CREATE TABLE IF NOT EXISTS cdr (
    id SERIAL PRIMARY KEY,
    call_uuid VARCHAR(255) UNIQUE NOT NULL,
    caller VARCHAR(255) NOT NULL,
    destination VARCHAR(255) NOT NULL,
    call_start_time TIMESTAMP,
    call_end_time TIMESTAMP,
    duration_seconds INTEGER,
    disposition VARCHAR(50) NOT NULL,
    hangup_cause VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index for faster queries
CREATE INDEX idx_cdr_call_uuid ON cdr(call_uuid);
CREATE INDEX idx_cdr_caller ON cdr(caller);
CREATE INDEX idx_cdr_destination ON cdr(destination);
CREATE INDEX idx_cdr_created_at ON cdr(created_at);
