# FreeSWITCH Configuration for ESL Resilience

This directory contains the FreeSWITCH configuration files optimized for the ESL Resilience project.

## Configuration Overview

### Core Configuration Files

- **`freeswitch.xml`** - Main FreeSWITCH configuration
- **`vars.xml`** - Global variables and settings
- **`autoload_configs/`** - Module configurations
- **`dialplan/`** - Call routing logic
- **`directory/`** - User directory (if needed)

### Key Configurations

#### ESL Configuration (`autoload_configs/esl.conf.xml`)
- **Listen IP**: `0.0.0.0` (all interfaces)
- **Port**: `8021`
- **Password**: `ClueCon`
- **ACL**: `lan` (allows local network access)

#### Event Socket Configuration (`autoload_configs/event_socket.conf.xml`)
- **Events**: All channel and call events enabled
- **Event Filtering**: Configurable for specific events
- **Authentication**: Basic auth enabled

#### SIP Profiles
- **`internal.xml`** - Internal SIP profile (port 5060)
- **`external.xml`** - External SIP profile (port 5080)
- **NAT Support**: Enabled for external profile
- **Codecs**: PCMU, PCMA, G729

#### CDR Configuration (`autoload_configs/cdr_csv.conf.xml`)
- **Database**: PostgreSQL integration
- **Host**: `postgres`
- **Port**: `5432`
- **Database**: `freeswitch_cdr`
- **Table**: `cdr`

#### ACL Configuration (`autoload_configs/acl.conf.xml`)
- **Local Networks**: 192.168.x.x, 10.x.x.x, 172.16.x.x
- **Docker Networks**: Container-friendly ACLs
- **Public Access**: Configurable for external SIP

#### Logging Configuration (`autoload_configs/logfile.conf.xml`)
- **Main Log**: `/var/log/freeswitch/freeswitch.log`
- **ESL Events**: `/var/log/freeswitch/esl_events.log`
- **CDR Log**: `/var/log/freeswitch/cdr.log`
- **Rotation**: Enabled with size limits

### Dialplan Extensions

#### Default Extensions
- **`.*`** - Default welcome message and park
- **`9196`** - Echo test
- **`9197`** - Milliwatt test
- **`9198`** - Hangup test
- **`1234`** - ESL resilience test with event firing

### Integration Points

#### ESL Resilience Service
- **Host**: `esl-resilience` (Docker service name)
- **Port**: `9090` (Metrics endpoint)
- **Events**: All channel lifecycle events

#### PostgreSQL Database
- **Connection**: Automatic CDR logging
- **Schema**: Pre-configured tables
- **Backup**: Volume persistence

#### Monitoring
- **Prometheus**: Metrics collection
- **Grafana**: Visualization dashboards
- **Health Checks**: Service monitoring

## Customization

### ESL Settings
Edit `autoload_configs/esl.conf.xml`:
```xml
<param name="password" value="YourSecurePassword"/>
<param name="listen-ip" value="192.168.1.100"/>
```

### SIP Settings
Edit `autoload_configs/sip_profiles/internal.xml`:
```xml
<param name="sip-port" value="5060"/>
<param name="codec-prefs" value="PCMU,PCMA"/>
```

### Database Settings
Edit `autoload_configs/cdr_csv.conf.xml`:
```xml
<param name="db-host" value="your-db-host"/>
<param name="db-password" value="your-db-password"/>
```

## Testing

### ESL Connection Test
```bash
# Connect to FreeSWITCH ESL
docker exec freeswitch fs_cli -x "esl status"

# Test ESL events
docker exec freeswitch fs_cli -x "event plain ALL"
```

### SIP Test
```bash
# Register a SIP client to internal profile
# SIP Server: localhost:5060
# Username: 1000
# Password: any

# Make a test call to extension 1234
```

### Database Test
```bash
# Check CDR database
docker exec postgres psql -U freeswitch -d freeswitch_cdr -c "SELECT * FROM cdr;"
```

## Troubleshooting

### ESL Connection Issues
1. Check ESL configuration in `esl.conf.xml`
2. Verify ACL settings in `acl.conf.xml`
3. Check FreeSWITCH logs for connection errors

### SIP Registration Issues
1. Verify SIP profile configuration
2. Check firewall/port settings
3. Review ACL settings for external access

### Database Issues
1. Verify PostgreSQL connection settings
2. Check database schema in `sql/init.sql`
3. Review CDR configuration

### Event Issues
1. Check event socket configuration
2. Verify event filtering settings
3. Review ESL resilience service logs

## Security Considerations

### Production Deployment
1. Change default ESL password
2. Restrict ACLs to specific networks
3. Enable TLS for SIP and ESL
4. Secure database connections
5. Enable audit logging

### Network Security
1. Configure firewall rules
2. Use VPN for remote access
3. Implement rate limiting
4. Monitor for suspicious activity

## Performance Tuning

### High Volume Environments
1. Increase `max_sessions` in `vars.xml`
2. Optimize codec preferences
3. Enable RTP proxy settings
4. Configure load balancing

### Resource Optimization
1. Adjust log rotation settings
2. Optimize database connections
3. Configure memory limits
4. Monitor resource usage

## Backup and Recovery

### Configuration Backup
```bash
# Backup FreeSWITCH configuration
tar -czf freeswitch-config-backup.tar.gz freeswitch/conf/
```

### Database Backup
```bash
# Backup CDR database
docker exec postgres pg_dump -U freeswitch freeswitch_cdr > cdr-backup.sql
```

### Recovery
```bash
# Restore configuration
tar -xzf freeswitch-config-backup.tar.gz -C freeswitch/

# Restore database
docker exec -i postgres psql -U freeswitch freeswitch_cdr < cdr-backup.sql
```
