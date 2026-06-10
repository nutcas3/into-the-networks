# WireGuard Zero-Rated VoIP

VPN-based VoIP system using WireGuard for zero-rated calling and secure communication.

## Overview

This service provides WireGuard VPN tunneling specifically optimized for VoIP traffic, enabling zero-rated data usage for voice calls through carrier partnerships.

## Features

- **WireGuard VPN Tunneling**: Modern, fast, secure VPN protocol
- **Zero-Rated Traffic**: VoIP traffic exempt from data caps
- **Dynamic Peer Provisioning**: Automatic key and IP allocation
- **Health Monitoring**: Real-time tunnel and peer metrics
- **Secure Communication**: End-to-end encrypted VoIP packets

## Architecture

```
Mobile Client (WireGuard)
    |
    v
WireGuard VoIP Server (wg0)
    |
    +---> VoIP Traffic -> Zero-rated route
    +---> Other Traffic -> Standard route
```

## Components

- **WireGuard Service**: Interface configuration and peer management
- **Peer Manager**: Dynamic key generation and IP allocation
- **Tunnel Service**: Route management for VoIP subnets
- **Zero-Rater**: Carrier zero-rating policy enforcement
- **Monitor**: Health metrics collection and reporting

## Quick Start

```bash
# Run locally (requires WireGuard kernel module)
go run ./cmd

# Or with Docker Compose
docker-compose up -d
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP API port | `8085` |
| `WG_PORT` | WireGuard UDP port | `51820` |
| `WG_SUBNET` | VPN subnet | `10.200.0.0/24` |
| `SERVER_IP` | Server public IP | auto-detect |
| `LOG_LEVEL` | Logging level | `info` |
| `ZERO_RATING_ENABLED` | Enable zero-rating | `true` |
| `CARRIER_API` | Carrier integration URL | - |

## API Endpoints

### Health Check

```
GET /health
```

Response:
```json
{
  "status": "healthy",
  "tunnel_up": true,
  "peer_count": 5,
  "connected_peers": 3,
  "timestamp": 1704067200
}
```

### Provision Peer

```
POST /provision
```

Request:
```json
{
  "user_id": "alice",
  "device_info": "iPhone 14"
}
```

Response:
```json
{
  "success": true,
  "private_key": "client-private-key...",
  "public_key": "client-public-key...",
  "client_ip": "10.200.0.2/32",
  "server_port": 51820,
  "allowed_ips": ["0.0.0.0/0"],
  "timestamp": 1704067200
}
```

### Revoke Peer

```
POST /revoke
```

Request:
```json
{
  "public_key": "peer-public-key..."
}
```

### List Peers

```
GET /peers
```

Response:
```json
{
  "peers": [...],
  "count": 5,
  "timestamp": 1704067200
}
```

### Metrics

```
GET /metrics
```

Response:
```json
{
  "timestamp": 1704067200,
  "tunnel_up": true,
  "peer_count": 5,
  "connected_peers": 3,
  "total_rx": 1048576,
  "total_tx": 2097152,
  "peer_details": [...]
}
```

### Configuration

```
GET /config
```

Response:
```json
{
  "interface": {...},
  "tunnel": {...},
  "zero_rated": true,
  "policies": [...],
  "timestamp": 1704067200
}
```

## Zero-Rating

### Default Policy

All traffic through the VPN tunnel is zero-rated by default:
- **Subnets**: `0.0.0.0/0` (all traffic)
- **Ports**: 5060 (SIP), 5061 (SIPS), 10000-20000 (RTP)
- **Protocols**: UDP, TCP

### Carrier Integration

Configure carrier API for session tracking:
```bash
CARRIER_API=https://carrier.example.com/api/v1
```

## Client Configuration

### iOS/Android

Use the official WireGuard app with the configuration returned by `/provision`:

```
[Interface]
PrivateKey = <client-private-key>
Address = <client-ip>
DNS = 1.1.1.1

[Peer]
PublicKey = <server-public-key>
Endpoint = <server-ip>:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
```

### Desktop

```bash
wg-quick up ./client-config.conf
```

## Security

1. **Keys**: Curve25519 key pairs generated per peer
2. **IPs**: /32 addresses allocated from configured subnet
3. **Rotation**: Revoke and re-provision peers periodically
4. **Firewall**: Only WireGuard UDP port exposed

## Development

### Running Tests

```bash
go test ./... -v
```

### Building

```bash
go build -o wireguard-voip ./cmd
```

### Docker

```bash
docker build -f docker/Dockerfile -t wireguard-voip:latest .
```

## Troubleshooting

### Tunnel Won't Start

1. Check NET_ADMIN capability
2. Verify kernel module: `lsmod | grep wireguard`
3. Check port binding: `ss -lunp | grep 51820`

### No Connectivity

1. Verify peer is provisioned
2. Check AllowedIPs in client config
3. Ensure PersistentKeepalive is set

### High Latency

1. Check MTU settings (default: 1420)
2. Verify endpoint IP is correct
3. Consider regional server placement

## License

MIT
