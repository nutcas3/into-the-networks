-- Kamailio database schema for dispatcher and routing

-- Create extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Dispatcher table for load balancing
CREATE TABLE IF NOT EXISTS dispatcher (
    id SERIAL PRIMARY KEY,
    setid INTEGER NOT NULL,
    destination VARCHAR(192) NOT NULL,
    flags INTEGER NOT NULL DEFAULT 0,
    priority INTEGER NOT NULL DEFAULT 0,
    attrs VARCHAR(128),
    description VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_dispatcher_setid ON dispatcher(setid);
CREATE INDEX idx_dispatcher_flags ON dispatcher(flags);

-- Carrier routing tables
CREATE TABLE IF NOT EXISTS dr_gateways (
    gwid SERIAL PRIMARY KEY,
    gwid_uuid VARCHAR(64) UNIQUE NOT NULL DEFAULT uuid_generate_v4(),
    type INTEGER NOT NULL DEFAULT 0,
    address VARCHAR(128) NOT NULL,
    strip INTEGER NOT NULL DEFAULT 0,
    pri_prefix VARCHAR(64),
    attrs VARCHAR(256),
    description VARCHAR(128),
    state INTEGER NOT NULL DEFAULT 0,
    socket VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS dr_rules (
    ruleid SERIAL PRIMARY KEY,
    groupid INTEGER NOT NULL,
    prefix VARCHAR(64) NOT NULL,
    timerec VARCHAR(255),
    priority INTEGER NOT NULL DEFAULT 0,
    routeid INTEGER NOT NULL,
    gwlist VARCHAR(255) NOT NULL,
    description VARCHAR(128),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_dr_rules_groupid ON dr_rules(groupid);
CREATE INDEX idx_dr_rules_prefix ON dr_rules(prefix);

CREATE TABLE IF NOT EXISTS dr_carriers (
    id SERIAL PRIMARY KEY,
    carrierid INTEGER NOT NULL,
    carrier_name VARCHAR(64) NOT NULL,
    domain VARCHAR(128),
    flags INTEGER NOT NULL DEFAULT 0,
    gwlist VARCHAR(255) NOT NULL,
    description VARCHAR(128),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_dr_carriers_carrierid ON dr_carriers(carrierid);

-- Carrier routing table
CREATE TABLE IF NOT EXISTS dr_gw_lists (
    id SERIAL PRIMARY KEY,
    gwid INTEGER NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    weight INTEGER NOT NULL DEFAULT 1,
    attrs VARCHAR(256),
    description VARCHAR(128),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Health check table
CREATE TABLE IF NOT EXISTS health_checks (
    id SERIAL PRIMARY KEY,
    destination VARCHAR(192) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'unknown',
    last_check TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    response_time INTEGER,
    failure_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_health_checks_destination ON health_checks(destination);
CREATE INDEX idx_health_checks_status ON health_checks(status);

-- Insert default dispatcher entries (will be managed dynamically)
-- These are placeholders for FreeSWITCH instances
INSERT INTO dispatcher (setid, destination, flags, priority, description) VALUES
(1, 'sip:freeswitch:5060;transport=udp', 0, 0, 'Multi-Tenant CDR FreeSWITCH')
ON CONFLICT DO NOTHING;

-- Insert default carrier
INSERT INTO dr_carriers (carrierid, carrier_name, gwlist, description) VALUES
(1, 'default', '1', 'Default carrier for all calls')
ON CONFLICT DO NOTHING;

-- Insert default gateway
INSERT INTO dr_gateways (gwid_uuid, type, address, description) VALUES
(uuid_generate_v4(), 0, 'sip:freeswitch1:5060;transport=udp', 'FreeSWITCH Gateway 1')
ON CONFLICT DO NOTHING;
