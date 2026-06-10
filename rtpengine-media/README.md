# RTPengine Media Anchoring

Media relay and transcoding system using RTPengine for NAT traversal and WebRTC support.

## Overview

This project implements a production-grade media server wrapper using RTPengine with the following capabilities:
- Media anchoring for NAT traversal
- Codec transcoding capabilities
- WebRTC media support (ICE/DTLS-SRTP)
- Session lifecycle management
- Media quality monitoring (MOS, jitter, packet loss)

## Architecture

```
SIP Clients -> Kamailio -> RTPengine Media Server
                              |
                              +---> Session Manager (Go)
                              +---> Metrics Collector
                              +---> Recording Storage
```

## Components

- **RTPengine**: Core media relay and transcoding engine
- **NG Protocol Client**: Go implementation for RTPengine communication
- **Session Manager**: Media session lifecycle management
- **SDP Parser**: SDP manipulation and codec handling
- **Metrics Collector**: Real-time media quality monitoring
- **HTTP API**: RESTful control interface

## Quick Start

```bash
# Build and start with Docker Compose
docker-compose up -d

# Verify RTPengine is running
docker-compose exec rtpengine rtpengine -v

# Test the API
curl http://localhost:8082/health
```

## Configuration

### Environment Variables

- `RTPEngine_ADDRESS`: RTPengine ng protocol address (default: `127.0.0.1:2223`)
- `PORT`: HTTP API port (default: `8082`)
- `LOG_LEVEL`: Logging level (default: `info`)

### RTPengine Kernel Module

For production deployments with high traffic:

```bash
# Load the kernel module
docker-compose exec rtpengine modprobe xt_RTPENGINE

# Configure iptables rules for media forwarding
docker-compose exec rtpengine iptables -I INPUT -p udp -j RTPENGINE --id 0
```

## API Endpoints

### Session Management

- `POST /api/v1/offer` - Create media session offer
  - Parameters: `callid`, `fromtag`, `sdp`, `direction` (optional), `ICE` (optional)
  - Returns: Modified SDP for RTPengine

- `POST /api/v1/answer` - Answer a media session
  - Parameters: `callid`, `fromtag`, `totag`, `sdp`
  - Returns: Modified SDP for RTPengine

- `POST /api/v1/delete` - Delete a media session
  - Parameters: `callid`, `fromtag` (optional), `totag` (optional)
  - Returns: Status confirmation

### Monitoring

- `GET /health` - Health check
- `GET /api/v1/sessions` - List all active sessions
- `GET /api/v1/sessions/{callid}` - Get specific session
- `GET /api/v1/metrics` - Global system metrics
- `GET /api/v1/metrics/session/{callid}` - Per-session metrics

### Recording

- `POST /api/v1/record/start` - Start recording
  - Parameters: `callid`, `fromtag` (optional), `totag` (optional)

- `POST /api/v1/record/stop` - Stop recording
  - Parameters: `callid`, `fromtag` (optional), `totag` (optional)

## NG Protocol Commands

The Go client implements the following RTPengine ng protocol commands:

- `ping` - Health check
- `offer` - Create/update session (SDP offer)
- `answer` - Update session (SDP answer)
- `delete` - Remove session
- `query` - Query session status
- `list` - List active sessions
- `start recording` - Begin recording
- `stop recording` - End recording

## SDP Handling

The SDP parser supports:
- Session and media-level parsing
- Codec extraction and manipulation
- ICE candidate handling
- Connection address management
- Bandwidth attribute processing
- WebRTC fingerprint attributes

### Codec Operations

```go
// Parse SDP
session, err := sdp.Parse(sdpString)

// Get codecs from first media section
codecs := session.MediaSections[0].GetCodecs()

// Replace codecs
newCodecs := []sdp.Codec{
    {PayloadType: 0, Name: "PCMU", ClockRate: 8000},
    {PayloadType: 8, Name: "PCMA", ClockRate: 8000},
}
session.ReplaceCodecs(newCodecs)

// Update media addresses
session.SetMediaAddress("192.168.1.100")

// Serialize back
newSDP := session.String()
```

## Quality Monitoring

The metrics collector tracks:

- **Per-session metrics**:
  - Packets sent/received/lost
  - Bytes transferred
  - Jitter (ms)
  - Round-trip time (ms)
  - MOS score (1.0 - 4.5)
  - Packet loss rate (%)

- **Global metrics**:
  - Active sessions
  - Total sessions/calls
  - Failed calls
  - Average MOS
  - Average jitter
  - Average packet loss

### MOS Calculation

MOS scores are calculated using a simplified ITU-T G.107 E-model:

```
MOS = 1 + 0.035*R + R*(R-60)*(100-R)*7*0.000001

Where R = 93.2 - Ie_eff - Id
```

## Integration with Kamailio

### Kamailio Configuration

Add to `kamailio.cfg`:

```
# Load rtpengine module
loadmodule "rtpengine.so"

# Configure RTPengine
modparam("rtpengine", "rtpengine_sock", "UDP:rtpengine:2223")
modparam("rtpengine", "rtpengine_disable_tout", 60)
modparam("rtpengine", "rtpengine_tout_ms", 2000)

# In request route for INVITE
if (has_body("application/sdp")) {
    rtpengine_offer();
}

# On 200 OK with SDP
if (has_body("application/sdp")) {
    rtpengine_answer();
}

# On BYE
rtpengine_delete();
```

### Docker Network Integration

Connect to the `telephony` network used by Kamailio:

```yaml
networks:
  media:
    external:
      name: telephony
```

## WebRTC Support

RTPengine provides WebRTC media handling:

- ICE candidate processing
- DTLS-SRTP key exchange
- STUN/TURN relay
- NAT traversal for browser clients

Configure WebRTC in offer/answer:

```bash
curl -X POST http://localhost:8082/api/v1/offer \
  -d "callid=webrtc-001" \
  -d "fromtag=tag1" \
  -d "sdp=$SDP" \
  -d "ICE=force" \
  -d "DTLS=passive"
```

## Recording

Media recording is handled by RTPengine:

```bash
# Start recording
curl -X POST http://localhost:8082/api/v1/record/start \
  -d "callid=call-001"

# Stop recording
curl -X POST http://localhost:8082/api/v1/record/stop \
  -d "callid=call-001"
```

Recorded files are stored in the `recordings` volume.

## Scaling

### Multiple RTPengine Instances

```yaml
services:
  rtpengine1:
    image: rtpengine/rtpengine:mr11.5
    environment:
      - LISTEN_NG=2223
      - PORT_MIN=10000
      - PORT_MAX=15000
  
  rtpengine2:
    image: rtpengine/rtpengine:mr11.5
    environment:
      - LISTEN_NG=2223
      - PORT_MIN=15001
      - PORT_MAX=20000
```

### Load Balancing

Use Kamailio's `rtpengine` module to distribute across multiple RTPengine instances:

```
modparam("rtpengine", "rtpengine_sock", "UDP:rtpengine1:2223 UDP:rtpengine2:2223")
```

## Development

### Building

```bash
cd /Users/nutcase/Documents/mines/networks/rtpengine-media
go build -o rtpengine-media ./cmd
```

### Testing

```bash
go test ./...
```

### Local Development

```bash
# Start RTPengine locally
rtpengine --listen-ng=2223 --no-fallback

# Run the Go server
RTPEngine_ADDRESS=127.0.0.1:2223 go run ./cmd
```

## Security Considerations

1. **Network Isolation**: Use dedicated Docker networks
2. **Firewall Rules**: Restrict RTP port ranges
3. **DTLS Certificates**: Use proper certificates for WebRTC
4. **Access Control**: Implement authentication for HTTP API
5. **Recording Encryption**: Encrypt stored recordings

## Troubleshooting

### RTPengine Connection Issues

```bash
# Check RTPengine is listening
docker-compose exec rtpengine netstat -ulnp | grep 2223

# Test with ng protocol client
echo "d2 ping" | nc -u rtpengine 2223
```

### No Audio

1. Check RTPengine kernel module is loaded
2. Verify iptables rules are configured
3. Check media addresses in SDP
4. Review RTPengine logs for errors

### High Jitter/Latency

1. Check network path between endpoints
2. Verify codec selection (avoid transcoding if possible)
3. Monitor RTPengine CPU usage
4. Consider dedicated network interfaces

## License

MIT
