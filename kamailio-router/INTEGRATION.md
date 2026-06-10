# Kamailio Router Integration Guide

This guide explains how to integrate the Kamailio SIP proxy with the multi-tenant CDR system's FreeSWITCH instances.

## Architecture Overview

```
SIP Clients
    |
    v
Kamailio Router (Load Balancer)
    |
    +---> FreeSWITCH (multi-tenant-cdr)
    |       |
    |       +---> CDR Capture Service
    |       +---> Multi-Tenant Database
    |       +---> Analytics Engine
    |
    +---> Additional FreeSWITCH Instances (optional)
```

## Network Configuration

Both systems use a shared Docker network called `telephony` for communication.

### Network Setup

The `telephony` network is automatically created by Docker Compose when you start either system. All services are connected to this network:

**multi-tenant-cdr services:**
- cdr-postgres
- cdr-redis
- cdr-freeswitch
- cdr-api

**kamailio-router services:**
- kamailio-postgres
- kamailio
- kamailio-rpc

## Integration Steps

### 1. Start Multi-Tenant CDR System

```bash
cd /Users/nutcase/Documents/mines/networks/multi-tenant-cdr
docker-compose up -d
```

This starts:
- PostgreSQL database
- Redis cache
- FreeSWITCH with ESL configured on port 8021
- API server on port 8080

### 2. Start Kamailio Router

```bash
cd /Users/nutcase/Documents/mines/networks/kamailio-router
docker-compose up -d
```

This starts:
- Kamailio PostgreSQL database
- Kamailio SIP proxy on port 5060
- RPC control API on port 8081

### 3. Verify FreeSWITCH ESL Configuration

FreeSWITCH ESL is configured to listen on `0.0.0.0:8021` to allow external connections from Kamailio.

Configuration file: `multi-tenant-cdr/freeswitch/conf/autoload_configs/event_socket.conf.xml`

```xml
<param name="listen-ip" value="0.0.0.0"/>
<param name="listen-port" value="8021"/>
<param name="password" value="ClueCon"/>
```

### 4. Add FreeSWITCH to Kamailio Dispatcher

The FreeSWITCH instance is automatically added to the Kamailio dispatcher via the database schema initialization.

Default entry in `kamailio-router/sql/schema.sql`:
```sql
INSERT INTO dispatcher (setid, destination, flags, priority, description) VALUES
(1, 'sip:freeswitch:5060;transport=udp', 0, 0, 'Multi-Tenant CDR FreeSWITCH');
```

You can also add it dynamically via the RPC API:

```bash
curl -X POST http://localhost:8081/api/v1/dispatcher/add \
  -H "Content-Type: application/json" \
  -d '{
    "setid": 1,
    "destination": "sip:freeswitch:5060;transport=udp",
    "priority": 0,
    "description": "Multi-Tenant CDR FreeSWITCH"
  }'
```

### 5. Verify Dispatcher Configuration

```bash
curl http://localhost:8081/api/v1/dispatcher/list?setid=1
```

Expected response:
```json
{
  "destinations": [
    {
      "id": 1,
      "setid": 1,
      "destination": "sip:freeswitch:5060;transport=udp",
      "flags": 0,
      "priority": 0,
      "description": "Multi-Tenant CDR FreeSWITCH"
    }
  ]
}
```

### 6. Perform Health Check

```bash
curl -X POST http://localhost:8081/api/v1/health/check \
  -H "Content-Type: application/json" \
  -d '{
    "destination": "sip:freeswitch:5060;transport=udp"
  }'
```

Expected response:
```json
{
  "destination": "sip:freeswitch:5060;transport=udp",
  "status": "healthy",
  "response_time": 15
}
```

## Call Flow

### Incoming Call Flow

1. **SIP Client** sends INVITE to Kamailio (5060)
2. **Kamailio** receives request and applies routing logic
3. **Dispatcher** selects FreeSWITCH destination (round-robin)
4. **Kamailio** forwards INVITE to FreeSWITCH
5. **FreeSWITCH** processes the call
6. **CDR Service** captures call events via ESL
7. **CDR** is stored in multi-tenant database
8. **Analytics Engine** processes call data

### Outgoing Call Flow

1. **FreeSWITCH** initiates outbound call
2. **Kamailio** can route to carrier gateways
3. **Carrier routing** based on dialed number
4. **Call completion** triggers CDR capture

## Adding Multiple FreeSWITCH Instances

To add additional FreeSWITCH instances for load balancing:

### 1. Start Additional FreeSWITCH

Modify `multi-tenant-cdr/docker-compose.yml` to add another FreeSWITCH service:

```yaml
  freeswitch2:
    image: freeswitch/freeswitch:1.10
    container_name: cdr-freeswitch2
    ports:
      - "5062:5060/udp"
      - "5062:5060/tcp"
      - "8022:8021/tcp"
      - "16385-16585:16384-16484/udp"
    volumes:
      - ./freeswitch/conf:/etc/freeswitch
      - freeswitch_recordings2:/var/lib/freeswitch/recordings
    environment:
      - FS_PASSWORD=ClueCon
    networks:
      - telephony
```

### 2. Add to Kamailio Dispatcher

```bash
curl -X POST http://localhost:8081/api/v1/dispatcher/add \
  -H "Content-Type: application/json" \
  -d '{
    "setid": 1,
    "destination": "sip:freeswitch2:5060;transport=udp",
    "priority": 0,
    "description": "FreeSWITCH Instance 2"
  }'
```

### 3. Reload Dispatcher

```bash
curl -X POST http://localhost:8081/api/v1/dispatcher/reload
```

## Carrier Routing

Configure carrier-specific routing for different number ranges:

### Add Carrier

```bash
curl -X POST http://localhost:8081/api/v1/carrier/add \
  -H "Content-Type: application/json" \
  -d '{
    "carrierid": 2,
    "carrier_name": "carrier_a",
    "gwlist": "2",
    "description": "Carrier A for international calls"
  }'
```

### Add Route

```bash
curl -X POST http://localhost:8081/api/v1/carrier/route \
  -H "Content-Type: application/json" \
  -d '{
    "groupid": 1,
    "prefix": "001",
    "priority": 10,
    "routeid": 2,
    "gwlist": "2"
  }'
```

## Monitoring

### Check Health Status

```bash
curl http://localhost:8081/api/v1/health/status
```

### View Kamailio Stats

```bash
docker-compose exec kamailio kamcmd core.stats
```

### View Dispatcher Status

```bash
docker-compose exec kamailio kamcmd dispatcher.list
```

## Troubleshooting

### FreeSWITCH Not Reachable

1. Check FreeSWITCH is running:
   ```bash
   docker-compose -f multi-tenant-cdr/docker-compose.yml ps freeswitch
   ```

2. Check ESL configuration:
   ```bash
   docker-compose -f multi-tenant-cdr/docker-compose.yml exec freeswitch cat /etc/freeswitch/autoload_configs/event_socket.conf.xml
   ```

3. Test ESL connection from Kamailio:
   ```bash
   docker-compose -f kamailio-router/docker-compose.yml exec kamailio nc -zv freeswitch 8021
   ```

### Kamailio Not Routing to FreeSWITCH

1. Check dispatcher list:
   ```bash
   curl http://localhost:8081/api/v1/dispatcher/list
   ```

2. Reload dispatcher:
   ```bash
   curl -X POST http://localhost:8081/api/v1/dispatcher/reload
   ```

3. Check Kamailio logs:
   ```bash
   docker-compose -f kamailio-router/docker-compose.yml logs kamailio
   ```

### Network Issues

1. Verify both systems use the same network:
   ```bash
   docker network inspect telephony
   ```

2. Check container connectivity:
   ```bash
   docker-compose -f kamailio-router/docker-compose.yml exec kamailio ping freeswitch
   ```

## API Reference

### Dispatcher Endpoints

- `POST /api/v1/dispatcher/add` - Add destination
- `POST /api/v1/dispatcher/remove` - Remove destination
- `GET /api/v1/dispatcher/list?setid=1` - List destinations
- `POST /api/v1/dispatcher/reload` - Reload dispatcher

### Carrier Endpoints

- `POST /api/v1/carrier/add` - Add carrier
- `POST /api/v1/carrier/remove` - Remove carrier
- `GET /api/v1/carrier/list` - List carriers
- `POST /api/v1/carrier/route` - Add route
- `GET /api/v1/carrier/route/:prefix` - Get route for prefix

### Health Check Endpoints

- `GET /api/v1/health/status` - Get health status
- `POST /api/v1/health/check` - Perform health check

## Security Considerations

1. **Change Default Passwords**: Update ESL password in FreeSWITCH configuration
2. **Network Isolation**: Use separate networks for production deployments
3. **TLS Encryption**: Enable TLS for SIP signaling in production
4. **Authentication**: Implement authentication for RPC API endpoints
5. **Firewall Rules**: Restrict access to ESL and RPC ports

## Performance Tuning

### Kamailio Performance

- Increase `children` parameter in `kamailio.cfg` for higher concurrency
- Adjust dispatcher ping interval based on network conditions
- Enable TCP keepalive for long-lived connections

### FreeSWITCH Performance

- Adjust ESL event subscription to reduce load
- Configure appropriate RTP port ranges
- Enable media proxy if needed for NAT traversal

## Scaling

### Horizontal Scaling

1. Add more FreeSWITCH instances
2. Add them to Kamailio dispatcher
3. Kamailio automatically load balances across all instances

### Vertical Scaling

1. Increase container resource limits
2. Adjust database connection pools
3. Optimize caching strategies

## Backup and Recovery

### Database Backups

```bash
# Backup Kamailio database
docker-compose -f kamailio-router/docker-compose.yml exec postgres pg_dump -U kamailio kamailio > kamailio_backup.sql

# Backup Multi-Tenant CDR database
docker-compose -f multi-tenant-cdr/docker-compose.yml exec postgres pg_dump -U cdr_user cdr_db > cdr_backup.sql
```

### Configuration Backups

```bash
# Backup Kamailio configuration
tar -czf kamailio-config-backup.tar.gz kamailio-router/cfg/

# Backup FreeSWITCH configuration
tar -czf freeswitch-config-backup.tar.gz multi-tenant-cdr/freeswitch/conf/
```

## Next Steps

- [ ] Implement authentication for RPC API
- [ ] Add TLS support for SIP signaling
- [ ] Configure carrier-specific routing rules
- [ ] Set up monitoring and alerting
- [ ] Implement disaster recovery procedures
- [ ] Add integration tests
