# ESL Resilience & Call State Tracking

Production-grade ESL client with connection resilience, reconnection logic, and complete SIP call state machine for FreeSWITCH telephony systems.

## Overview

This project builds upon the FreeSWITCH CDR foundation to create a production-ready ESL client that handles:
- Automatic reconnection with exponential backoff
- Complete SIP call lifecycle tracking
- Connection health monitoring
- Event buffering during disconnections
- Circuit breaker patterns
- Production monitoring and alerting

## Features

### Core Resilience
- Exponential Backoff Reconnection: Smart retry logic with configurable backoff
- Circuit Breaker: Prevents cascade failures during outages
- Health Monitoring: Continuous connection health checks
- Event Buffering: Queue events during disconnections

### Call State Management
- SIP State Machine: Complete call lifecycle (INVITE → BYE)
- Channel Tracking: Real-time channel state monitoring
- Event Deduplication: Prevent duplicate event processing
- Call Correlation: Link related call events

### Production Monitoring
- Prometheus Metrics: Comprehensive monitoring metrics
- Alerting: Configurable alerts for system health
- Logging: Structured logging with correlation IDs
- Performance Tracking: Latency and throughput monitoring

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   FreeSWITCH   │◄──►│  ESL Resilience  │◄──►│  Monitoring     │
│   (ESL Port)   │    │     Client       │    │   System        │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │  Call State     │
                       │  Machine        │
                       └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │  Event Buffer   │
                       └─────────────────┘
```

## Quick Start

### Prerequisites
- Go 1.21+
- FreeSWITCH with ESL enabled
- PostgreSQL (for call state persistence)

### Installation

```bash
# Clone the project
git clone https://github.com/nutcas3/esl-resilience.git
cd esl-resilience

# Install dependencies
go mod tidy

# Configure
cp config/config.example.yaml config/config.yaml

# Run
go run cmd/main.go
```

### Configuration

```yaml
freeswitch:
  host: "localhost"
  port: 8021
  password: "ClueCon"
  timeout: "5s"

resilience:
  max_retries: 10
  initial_backoff: "1s"
  max_backoff: "60s"
  circuit_breaker_threshold: 5

monitoring:
  prometheus_port: 9090
  health_check_interval: "30s"
```

## Components

### ESL Client (`pkg/esl/`)
- Connection Manager: Handles connection lifecycle
- Reconnection Logic: Exponential backoff retry mechanism
- Event Handler: Processes FreeSWITCH events
- Health Monitor: Connection health checking

### State Machine (`pkg/state/`)
- **Call States**: INVITE, RINGING, ANSWERED, BYE, FAILED
- **Transitions**: State change logic and validation
- **Persistence**: State storage and recovery
- **Correlation**: Event-to-call mapping

### Monitoring (`pkg/monitor/`)
- **Metrics Collector**: Prometheus metrics
- **Health Checker**: System health monitoring
- **Alert Manager**: Alert generation and routing
- **Performance Tracker**: Latency and throughput

### Event Buffer (`pkg/buffer/`)
- **Queue Management**: Event queuing during disconnections
- **Replay Logic**: Event replay on reconnection
- **Deduplication**: Duplicate event prevention
- **Persistence**: Buffer state persistence

## Usage Examples

### Basic ESL Connection

```go
package main

import (
    "context"
    "log"
    
    "github.com/nutcas3/esl-resilience/pkg/esl"
    "github.com/nutcas3/esl-resilience/pkg/state"
)

func main() {
    // Create ESL client with resilience
    client := esl.NewClient(esl.Config{
        Host:     "localhost",
        Port:     8021,
        Password: "ClueCon",
        Timeout:  5 * time.Second,
    })
    
    // Create state machine
    stateMachine := state.NewMachine()
    
    // Start with context
    ctx := context.Background()
    if err := client.Start(ctx, stateMachine); err != nil {
        log.Fatal(err)
    }
}
```

### Custom Event Handling

```go
// Register event handlers
client.OnEvent("CHANNEL_CREATE", func(event *esl.Event) {
    stateMachine.HandleChannelCreate(event)
})

client.OnEvent("CHANNEL_ANSWER", func(event *esl.Event) {
    stateMachine.HandleChannelAnswer(event)
})

client.OnEvent("CHANNEL_HANGUP_COMPLETE", func(event *esl.Event) {
    stateMachine.HandleChannelHangup(event)
})
```

### Monitoring Integration

```go
// Enable Prometheus metrics
monitor := monitor.NewPrometheusMonitor(monitor.Config{
    Port: 9090,
})
client.SetMonitor(monitor)

// Custom metrics
client.IncrementCounter("esl_connections_total")
client.SetGauge("active_calls", stateMachine.ActiveCalls())
```

## Call State Machine

### States
- `CALL_INIT`: Initial call setup
- `CALL_EARLY`: Early media (180/183)
- `CALL_CONFIRMED`: 200 OK received
- `CALL_ESTABLISHED`: ACK completed
- `CALL_TERMINATED`: BYE processed

### Transitions
```
CALL_INIT ──► CALL_EARLY ──► CALL_CONFIRMED ──► CALL_ESTABLISHED
    │               │               │                    │
    ▼               ▼               ▼                    ▼
CALL_FAILED   CALL_REJECTED   CALL_CANCELLED    CALL_TERMINATED
```

## Resilience Features

### Exponential Backoff
```go
// Retry intervals: 1s, 2s, 4s, 8s, 16s, 32s, 60s, 60s, ...
backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
if backoff > maxBackoff {
    backoff = maxBackoff
}
```

### Circuit Breaker
```go
// Opens after 5 consecutive failures
// Closes after 3 successful attempts
if consecutiveFailures >= threshold {
    circuitBreaker.Open()
}
```

### Event Buffering
```go
// Buffer events during disconnection
buffer.Enqueue(event)

// Replay on reconnection
for event := range buffer.Events() {
    client.SendEvent(event)
}
```

## Monitoring

### Prometheus Metrics
- `esl_connection_status`: Connection status (0/1)
- `esl_reconnect_attempts_total`: Reconnection attempts
- `esl_events_processed_total`: Events processed
- `esl_events_buffered_total`: Events buffered
- `sip_calls_active`: Active calls count
- `sip_call_duration_seconds`: Call duration

### Health Checks
- Connection status
- Event processing lag
- Buffer utilization
- Memory usage

## Testing

```bash
# Run unit tests
go test ./...

# Run integration tests
go test -tags=integration ./...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...
```

## Deployment

### Docker
```bash
docker build -t esl-resilience .
docker run -p 9090:9090 esl-resilience
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: esl-resilience
spec:
  replicas: 3
  selector:
    matchLabels:
      app: esl-resilience
  template:
    metadata:
      labels:
        app: esl-resilience
    spec:
      containers:
      - name: esl-resilience
        image: esl-resilience:latest
        ports:
        - containerPort: 9090
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Next Steps

This project serves as the foundation for:
- Multi-tenant CDR systems
- Kamailio integration
- RTPengine media handling
- WebRTC gateway development
