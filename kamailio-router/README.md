# Kamailio Load Balancer & Routing

SIP proxy system using Kamailio for load balancing, routing logic, and carrier management.

## Overview

This project implements a production-grade SIP proxy using Kamailio with the following capabilities:
- Load balancing across multiple FreeSWITCH instances
- Dynamic routing based on carrier rules
- JSON-RPC control interface
- Health checking and automatic failover
- Real-time routing updates

## Architecture

```
SIP Clients -> Kamailio -> FreeSWITCH Instances (Load Balanced)
                      -> Carrier Routes
                      -> PSTN Gateways
```

## Components

- **Kamailio SIP Proxy**: Core SIP routing engine
- **Dispatcher Module**: Load balancing across FreeSWITCH instances
- **JSON-RPC Interface**: Dynamic control and configuration
- **Carrier Management**: Route calls to different carriers based on rules
- **Health Checker**: Monitor FreeSWITCH instance health

## Quick Start

```bash
# Build and start with Docker Compose
docker-compose up -d

# Check Kamailio status
docker-compose exec kamailio kamctl rpc core.stats

# View dispatcher destinations
docker-compose exec kamailio kamctl dispatcher show
```

## Configuration

### Environment Variables

- `KAMAILIO_LISTEN_PORT`: SIP listening port (default: 5060)
- `KAMAILIO_JSONRPC_PORT`: JSON-RPC port (default: 5060)
- `FREESWITCH_HOSTS`: Comma-separated list of FreeSWITCH hosts
- `DB_HOST`: Database host for routing rules
- `DB_PORT`: Database port
- `DB_NAME`: Database name
- `DB_USER`: Database user
- `DB_PASSWORD`: Database password

## API Endpoints

### JSON-RPC

- `dispatcher.add`: Add a destination to dispatcher
- `dispatcher.remove`: Remove a destination
- `dispatcher.list`: List all destinations
- `carrier.add`: Add a carrier route
- `carrier.remove`: Remove a carrier route
- `carrier.list`: List all carrier routes

## Integration with Multi-Tenant CDR

This project integrates with the multi-tenant-cdr system by:
- Routing SIP traffic to FreeSWITCH instances managed by multi-tenant-cdr
- Providing health-based load balancing
- Supporting carrier-specific routing for tenant isolation

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for detailed development instructions.

## License

MIT
