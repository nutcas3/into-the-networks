# VoIP Push Notifications

Mobile wake-up system using FCM/APNs for reliable incoming calls on suspended mobile devices.

## Overview

This service provides VoIP push notification delivery to mobile devices, enabling reliable incoming call wake-up even when apps are suspended or terminated by the OS.

## Features

- **Firebase Cloud Messaging (FCM)**: Android push notifications
- **Apple Push Notification Service (APNs)**: iOS VoIP push notifications
- **Device Token Management**: Registration, updates, and cleanup
- **Retry Logic**: Automatic retry with configurable delays
- **Platform Detection**: Automatic platform-specific push routing
- **Token Validation**: Invalid token detection and device cleanup
- **Statistics**: Device count and platform distribution metrics

## Architecture

```
SIP/WebRTC Gateway
    |
    v
VoIP Push Service
    |
    +---> FCM (Android)
    |
    +---> APNs (iOS)
```

## Components

- **FCM Service**: Android push notification delivery
- **APNS Service**: iOS VoIP push notification delivery
- **Device Manager**: Token registration and lifecycle management
- **Push Orchestrator**: Coordinates platform-specific delivery with retries
- **HTTP API**: REST endpoints for device registration and push triggering

## Quick Start

```bash
# Build and run locally
go run ./cmd

# Or with Docker Compose
docker-compose up -d
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8084` |
| `LOG_LEVEL` | Logging level | `info` |
| `FCM_SERVER_KEY` | Firebase server key | - |
| `FCM_PROJECT_ID` | Firebase project ID | - |
| `APNS_BUNDLE_ID` | iOS app bundle ID | - |
| `APNS_CERT_PATH` | APNS certificate path | - |
| `APNS_CERT_PASSWORD` | Certificate password | - |
| `APNS_USE_JWT` | Use JWT authentication | `false` |
| `APNS_KEY_ID` | APNS key ID | - |
| `APNS_TEAM_ID` | Apple team ID | - |
| `APNS_AUTH_KEY_PATH` | Auth key path (.p8) | - |

## API Endpoints

### Health Check

```
GET /health
```

Response:
```json
{"status":"healthy","timestamp":"2024-01-01T00:00:00Z"}
```

### Register Device

```
POST /register
```

Request:
```json
{
  "user_id": "alice",
  "device_id": "device-123",
  "platform": "ios",
  "push_token": "apns-device-token",
  "voip_token": "apns-voip-token",
  "app_version": "1.0.0",
  "os_version": "16.0",
  "device_model": "iPhone14,2"
}
```

Response:
```json
{
  "success": true,
  "message": "Device registered successfully",
  "timestamp": 1704067200
}
```

### Unregister Device

```
POST /unregister
```

Request:
```json
{
  "user_id": "alice"
}
```

### Send Push Notification

```
POST /push
```

Request:
```json
{
  "user_id": "alice",
  "caller_id": "bob",
  "caller_name": "Bob Smith",
  "session_id": "session-abc123"
}
```

Response:
```json
{
  "success": true,
  "message": "Push sent to ios device",
  "timestamp": 1704067200
}
```

### Statistics

```
GET /stats
```

Response:
```json
{
  "total_devices": 42,
  "devices_by_platform": {
    "ios": 25,
    "android": 17
  },
  "timestamp": 1704067200
}
```

## Platform-Specific Details

### iOS (APNs)

Uses VoIP push notifications with the `.voip` topic suffix:
- High priority immediate delivery
- Can wake up terminated apps
- Integrates with CallKit for native call UI
- Requires VoIP certificate or JWT authentication

### Android (FCM)

Uses high-priority data messages:
- `priority: high` for immediate delivery
- `ttl: 0s` for no delay
- Custom notification channel for calls
- Can trigger foreground service for call handling

## Retry Configuration

Default retry settings:
- **Retry Count**: 3 attempts
- **Retry Delay**: 2 seconds between attempts
- **Push Timeout**: 10 seconds per attempt

## Token Management

### Registration

Devices register with:
- User ID (SIP username or identifier)
- Platform (ios/android)
- Push token (FCM or APNS token)
- VoIP token (iOS only, optional)

### Updates

Tokens are updated automatically when:
- New registration with same user ID
- Token refresh from mobile app

### Cleanup

Invalid tokens are detected and devices are disabled:
- APNS 410 Gone response
- FCM NotRegistered error
- Failed validation checks

### Inactive Device Cleanup

Configure periodic cleanup of inactive devices:
```go
removed := deviceMgr.CleanupInactive(30 * 24 * time.Hour) // 30 days
```

## Integration with WebRTC Gateway

When an incoming call arrives at the WebRTC gateway:

1. Gateway checks if callee has a registered mobile device
2. If yes, sends push request to VoIP Push service
3. Push service wakes up the mobile device
4. Mobile app registers with the WebSocket signaling server
5. Call proceeds via WebRTC

Example integration:
```go
// In WebRTC gateway
func handleIncomingCall(callee string) {
    // Send push notification first
    http.Post("http://voip-push:8084/push", "application/json", body)
    
    // Then proceed with call setup
    // ...
}
```

## Mobile App Integration

### iOS (Swift)

```swift
import PushKit

class VoIPPushDelegate: NSObject, PKPushRegistryDelegate {
    func pushRegistry(_ registry: PKPushRegistry, didUpdate pushCredentials: PKPushCredentials, for type: PKPushType) {
        let token = pushCredentials.token.map { String(format: "%02.2hhx", $0) }.joined()
        // Send token to server
    }
    
    func pushRegistry(_ registry: PKPushRegistry, didReceiveIncomingPushWith payload: PKPushPayload, for type: PKPushType, completion: @escaping () -> Void) {
        // Handle incoming call
        let callerID = payload.dictionaryPayload["caller_id"] as? String
        // Report to CallKit
    }
}
```

### Android (Kotlin)

```kotlin
class VoIPFirebaseService : FirebaseMessagingService() {
    override fun onMessageReceived(remoteMessage: RemoteMessage) {
        if (remoteMessage.data["type"] == "voip_incoming_call") {
            val callerId = remoteMessage.data["caller_id"]
            // Wake up app and handle call
        }
    }
    
    override fun onNewToken(token: String) {
        // Send token to server
    }
}
```

## Security Considerations

1. **Token Storage**: Store push tokens securely
2. **Authentication**: Require API authentication for push endpoints
3. **Rate Limiting**: Prevent push abuse
4. **Certificate Security**: Keep APNS certificates/keys secure
5. **TLS**: Use HTTPS for all API communication

## Development

### Running Tests

```bash
go test ./... -v
```

### Building

```bash
go build -o voip-push ./cmd
```

### Docker

```bash
docker build -f docker/Dockerfile -t voip-push:latest .
docker run -p 8084:8084 -e FCM_SERVER_KEY=xxx voip-push:latest
```

## Troubleshooting

### Push Not Received

1. Check device token is valid
2. Verify push certificate/key is valid
3. Check platform-specific restrictions
4. Review service logs for errors

### Token Invalid Errors

1. Device may have uninstalled the app
2. Token may have expired
3. Certificate may have expired
4. Re-register device

### High Latency

1. Check network connectivity to FCM/APNs
2. Verify retry configuration
3. Consider regional push servers

## License

MIT
