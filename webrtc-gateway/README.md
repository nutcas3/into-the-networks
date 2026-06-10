# WebRTC to SIP Gateway

Complete WebRTC to SIP bridging system allowing web browsers to make/receive traditional SIP calls.

## Overview

This project implements a production-grade WebRTC gateway with the following capabilities:
- WebSocket-based signaling server
- SIP to WebRTC protocol translation
- ICE candidate exchange for NAT traversal
- DTLS-SRTP key management
- Browser-based softphone UI
- Integration with RTPengine for media relay

## Architecture

```
Browser (WebRTC Client)
    |
    v
WebSocket Signaling Server
    |
    +---> SIP Translator
    |       |
    |       +---> Kamailio SIP Proxy
    |
    +---> RTPengine Media Server
            |
            +---> FreeSWITCH
```

## Components

- **Signaling Server**: WebSocket-based signaling for WebRTC
- **SIP Translator**: SIP to WebRTC protocol translation
- **WebRTC Peer Manager**: Pion WebRTC peer connection management
- **Media Manager**: RTPengine integration for media relay
- **Softphone UI**: Browser-based calling interface

## Quick Start

```bash
# Build and start with Docker Compose
docker-compose up -d

# Access the softphone
open http://localhost:8083
```

## Configuration

### Environment Variables

- `RTPENGINE_ADDRESS`: RTPengine ng protocol address (default: `127.0.0.1:2223`)
- `SIP_SERVER`: SIP server address (default: `127.0.0.1`)
- `SIP_PORT`: SIP server port (default: `5060`)
- `PORT`: HTTP server port (default: `8083`)
- `LOG_LEVEL`: Logging level (default: `info`)

## Using the Softphone

1. **Register**: Enter a username and click Register
2. **Make a Call**: Enter the callee's username and click Call
3. **Answer Incoming Call**: Click Answer when receiving a call
4. **Hangup**: Click Hangup to end the call

### Dialpad

Use the on-screen dialpad to enter phone numbers:
- Digits 0-9
- * and # for special functions

## Signaling Protocol

### Message Types

- `register` - User registration
- `call` - Call initiation
- `offer` - WebRTC SDP offer
- `answer` - WebRTC SDP answer
- `ice` - ICE candidate exchange
- `hangup` - Call termination
- `ping` - Connection keep-alive
- `pong` - Ping response

### Message Format

```json
{
  "type": "offer",
  "from": "user1",
  "to": "user2",
  "session_id": "session-abc123",
  "sdp": "v=0\r\no=- ...",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

## SIP Translation

The SIP translator handles:

- **INVITE** → WebRTC offer
- **200 OK** → WebRTC answer
- **BYE** → Hangup
- **REGISTER** → User registration

### SDP Conversion

WebRTC SDP includes:
- ICE candidates
- DTLS fingerprints
- SRTP crypto attributes

SIP SDP includes:
- Media addresses
- Codec lists
- Bandwidth information

The translator converts between these formats.

## WebRTC Implementation

### Peer Connection Management

```go
// Create peer connection
peer, err := manager.CreatePeerConnection(userID, sessionID)

// Create offer
offerSDP, err := peer.CreateOffer()

// Create answer
answerSDP, err := peer.CreateAnswer(offerSDP)

// Add ICE candidate
err := peer.AddICECandidate(candidate, sdpMid, sdpMLineIndex)
```

### ICE Configuration

STUN servers:
- `stun:stun.l.google.com:19302` (default)

TURN servers can be configured for relay:
```go
turnConfig := webrtc.TURNConfig{
    URL:       "turn:turn.example.com:3478",
    Username:  "user",
    Password:  "pass",
}
```

## Media Integration

### RTPengine Integration

The gateway uses RTPengine for media relay:

```go
// Send offer to RTPengine
localSDP, err := mediaManager.Offer(callID, fromTag, remoteSDP, options)

// Send answer to RTPengine
localSDP, err := mediaManager.Answer(callID, fromTag, toTag, remoteSDP, options)

// Delete session
err := mediaManager.Delete(callID, fromTag, toTag)
```

### Media Flow

1. Browser creates WebRTC peer connection
2. SDP offer/answer exchanged via WebSocket
3. SDP sent to RTPengine for media anchoring
4. RTPengine relays media between WebRTC and SIP
5. SIP endpoint receives media via FreeSWITCH

## Integration with Kamailio

### Kamailio Configuration

Add to `kamailio.cfg`:

```
# Load WebSocket module
loadmodule "websocket.so"
loadmodule "siptrace.so"

# Configure WebSocket
modparam("websocket", "enable", 1)
modparam("websocket", "port", 8083)

# Route WebRTC calls
if ($ua =~ "WebRTC") {
    route(WEBRTC);
}

route[WEBRTC] {
    # Forward to WebRTC gateway
    t_relay_to("udp:webrtc-gateway:5060");
}
```

### Docker Network Integration

Connect to the `telephony` network:

```yaml
networks:
  webrtc:
    external:
      name: telephony
```

## Security Considerations

1. **Authentication**: Implement user authentication
2. **TLS**: Use WSS for WebSocket connections
3. **DTLS**: WebRTC uses DTLS for encryption
4. **SRTP**: Media is encrypted via SRTP
5. **TURN**: Use authenticated TURN servers
6. **CORS**: Configure CORS headers properly

## STUN/TURN Configuration

### STUN Server

Default: `stun:stun.l.google.com:19302`

### TURN Server

Configure for NAT traversal:

```javascript
const pc = new RTCPeerConnection({
    iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        {
            urls: 'turn:turn.example.com:3478',
            username: 'user',
            credential: 'pass'
        }
    ]
});
```

## Development

### Building

```bash
cd /Users/nutcase/Documents/mines/networks/webrtc-gateway
go build -o webrtc-gateway ./cmd
```

### Running Locally

```bash
# Start RTPengine
rtpengine --listen-ng=2223 --no-fallback

# Start gateway
RTPENGINE_ADDRESS=127.0.0.1:2223 go run ./cmd
```

### Testing

Open two browser windows to `http://localhost:8083`:
1. Register as "user1"
2. Register as "user2"
3. Call between users

## Troubleshooting

### WebSocket Connection Failed

1. Check if the server is running
2. Verify the WebSocket URL
3. Check browser console for errors

### No Audio

1. Verify RTPengine is running
2. Check ICE candidates are exchanged
3. Verify media permissions in browser
4. Check RTPengine logs

### Call Not Establishing

1. Check signaling messages in log
2. Verify SDP offer/answer exchange
3. Check peer connection state
4. Verify SIP translator is working

## API Endpoints

- `GET /health` - Health check
- `GET /` - Softphone UI
- `WS /ws` - WebSocket signaling endpoint

## Browser Compatibility

- Chrome/Edge: Full support
- Firefox: Full support
- Safari: Full support (iOS 11+)
- Opera: Full support

## Performance Tuning

### Peer Connection Pooling

Reuse peer connections for multiple calls to the same user.

### Media Optimization

- Use codec preference (Opus preferred)
- Adjust bitrate based on network conditions
- Enable audio processing filters

### Signaling Optimization

- Use binary WebSocket messages
- Implement message batching
- Use WebSocket compression

## Scaling

### Horizontal Scaling

Deploy multiple gateway instances behind a load balancer:

```yaml
services:
  gateway1:
    build: .
    ports: ["8083:8083"]
  
  gateway2:
    build: .
    ports: ["8084:8083"]
```

### Session Affinity

Use sticky sessions to ensure WebSocket connections stay with the same instance.

## License

MIT
