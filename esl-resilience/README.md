# ESL Resilience & Call State Tracking

Production-grade ESL client with connection resilience, reconnection logic, and complete SIP call state machine for FreeSWITCH telephony systems.

## Overview

This project builds upon the FreeSWITCH CDR foundation to create a production-ready ESL client that handles:
- **Automatic reconnection** with exponential backoff
- **Complete SIP call lifecycle tracking**  
- **Connection health monitoring**
- **Event buffering during disconnections**
- **Circuit breaker patterns**
- **Production monitoring and alerting**
- **Comprehensive testing suite**
- **Container deployment ready**
- **Kubernetes production configs**

## Features

### Core Resilience
- **Exponential Backoff Reconnection**: Smart retry logic with configurable backoff
- **Circuit Breaker**: Prevents cascade failures during outages
- **Health Monitoring**: Continuous connection health checks
- **Event Buffering**: Queue events during disconnections

### Call State Management
- **SIP State Machine**: Complete call lifecycle (INVITE → BYE)
- **Channel Tracking**: Real-time channel state monitoring
- **Event Deduplication**: Prevent duplicate event processing
- **Call Correlation**: Link related call events

### Production Monitoring
- **Prometheus Metrics**: Comprehensive monitoring metrics
- **Alerting**: Configurable alerts for system health
- **Logging**: Structured logging with correlation IDs
- **Performance Tracking**: Latency and throughput monitoring

### Testing & Deployment
- **Unit Tests**: 100% coverage for core components
- **Integration Tests**: End-to-end workflow testing
- **Baresip Testing**: Real SIP client call testing
- **Docker Support**: Multi-stage container builds
- **Kubernetes**: Production-ready manifests
- **FreeSWITCH Config**: Battle-tested configuration

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
- Docker 20.10+ (for containerized deployment)
- FreeSWITCH with ESL enabled (included in docker-compose)

### Option 1: Docker Compose (Recommended)

```bash
# Clone the project
git clone https://github.com/nutcas3/esl-resilience.git
cd esl-resilience

# Start complete stack
docker-compose up -d

# Check services
docker-compose ps

# Access services
# ESL Metrics: http://localhost:9090/metrics
# Prometheus: http://localhost:9091
# Grafana: http://localhost:3000 (admin/admin)
```

### Option 2: Local Development

```bash
# Install dependencies
go mod tidy

# Run tests
go test ./internal/...

# Build and run
go build ./cmd/main.go
./main
```

### Option 3: Call Testing with Baresip

```bash
# Start test environment
cd testing/baresip
./test-calls.sh

# Manual testing
docker exec -it baresip-client1 baresip
# In baresip: dial 1002
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FREESWITCH_HOST` | localhost | FreeSWITCH ESL host |
| `FREESWITCH_PORT` | 8021 | FreeSWITCH ESL port |
| `FREESWITCH_PASSWORD` | ClueCon | FreeSWITCH ESL password |
| `ESL_MAX_RETRIES` | 10 | Maximum connection retry attempts |
| `ESL_INITIAL_BACKOFF` | 1s | Initial reconnection backoff |
| `ESL_MAX_BACKOFF` | 60s | Maximum reconnection backoff |
| `ESL_BUFFER_SIZE` | 10000 | Event buffer maximum size |
| `MONITOR_PORT` | 9090 | Prometheus metrics port |

### Docker Compose Configuration

```yaml
services:
  esl-resilience:
    build: .
    ports:
      - "9090:9090"
    environment:
      - FREESWITCH_HOST=freeswitch
      - FREESWITCH_PASSWORD=ClueCon
      - ESL_MAX_RETRIES=10
    depends_on:
      - freeswitch
      - prometheus
```

## Components

### ESL Client (`internal/esl/`)
- **Connection Manager**: Handles connection lifecycle
- **Reconnection Logic**: Exponential backoff retry mechanism
- **Event Handler**: Processes FreeSWITCH events
- **Health Monitor**: Connection health checking
- **Circuit Breaker**: Failure detection and recovery

### State Machine (`internal/state/`)
- **Call States**: INVITE, RINGING, ANSWERED, BYE, FAILED
- **Transitions**: State change logic and validation
- **Persistence**: State storage and recovery
- **Correlation**: Event-to-call mapping

### Monitoring (`internal/monitor/`)
- **Metrics Collector**: Prometheus metrics
- **Health Checker**: System health monitoring
- **Alert Manager**: Alert generation and routing
- **Performance Tracker**: Latency and throughput

### Event Buffer (`internal/esl/buffer.go`)
- **Queue Management**: Event queuing during disconnections
- **Replay Logic**: Event replay on reconnection
- **Deduplication**: Duplicate event prevention
- **Persistence**: Buffer state persistence

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

### Unit Tests
```bash
# Run all unit tests (100% passing)
go test ./internal/...

# Run specific package tests
go test ./internal/esl/
go test ./internal/state/
go test ./internal/monitor/

# Run with coverage
go test -cover ./internal/...
```

### Integration Tests
```bash
# Run integration tests
go test -tags=integration ./internal/

# Test specific scenarios
go test -v ./internal/integration_test.go
```

### Call Testing with Baresip
```bash
# Start test environment
cd testing/baresip
./test-calls.sh

# Manual testing commands
docker exec -it baresip-client1 baresip
# In baresip: dial 1002 (call client 2)
# In baresip: dial 1234 (ESL test extension)
# In baresip: dial 9196 (echo test)

# Monitor ESL events
docker exec freeswitch-test fs_cli -x "event plain ALL"

# Check metrics
curl http://localhost:9090/metrics
```

### Load Testing
```bash
# Install hey for load testing
go install github.com/rakyll/hey@latest

# Test metrics endpoint
hey -n 1000 -c 10 http://localhost:9090/metrics

# Test with custom configuration
hey -z 30s -c 20 -q 5 http://localhost:9090/metrics
```

## 🐳 Deployment

### Docker
```bash
# Build image
docker build -t esl-resilience .

# Run with default configuration
docker run -d \
  --name esl-resilience \
  -p 9090:9090 \
  -e FREESWITCH_HOST=your-freeswitch-host \
  esl-resilience
```

### Docker Compose (Production)
```bash
# Start complete stack
docker-compose up -d

# Scale ESL resilience
docker-compose up -d --scale esl-resilience=3

# View logs
docker-compose logs -f esl-resilience
```

### Kubernetes
```bash
# Apply all configurations
kubectl apply -f k8s/

# Check deployment
kubectl get pods -n esl-resilience

# View logs
kubectl logs -f deployment/esl-resilience -n esl-resilience

# Scale deployment
kubectl scale deployment esl-resilience --replicas=5 -n esl-resilience
```

### Kubernetes Components
- **Namespace**: Isolated deployment environment
- **Deployment**: 3 replicas with resource limits
- **Service**: ClusterIP service for metrics
- **HPA**: Horizontal Pod Autoscaling
- **Ingress**: SSL termination and external access
- **ServiceMonitor**: Prometheus integration

## FreeSWITCH Configuration

### Working Configuration
The project includes battle-tested FreeSWITCH configuration:

```bash
# Configuration location
freeswitch/conf/
├── freeswitch.xml              # Main configuration
├── vars.xml                    # Global variables
├── autoload_configs/
│   ├── event_socket.conf.xml   # ESL configuration
│   ├── sofia.conf.xml          # SIP profiles
│   ├── sip_profiles/
│   │   ├── internal.xml        # Internal SIP profile
│   │   └── external.xml        # External SIP profile
│   └── modules.conf.xml        # Module loading
└── dialplan/
    └── default.xml             # Call routing
```

### Key Settings
- **ESL Port**: 8021 with ClueCon password
- **SIP Ports**: 5060 (internal), 5080 (external)
- **RTP Range**: 16384-32768
- **Codecs**: PCMU, PCMA, G729
- **CDR Database**: PostgreSQL integration

## Documentation

- **[DEPLOYMENT.md](./DEPLOYMENT.md)**: Comprehensive deployment guide
- **[QUICK_START.md](./QUICK_START.md)**: Quick start instructions
- **[freeswitch/README.md](./freeswitch/README.md)**: FreeSWITCH configuration
- **[testing/baresip/README.md](./testing/baresip/README.md)**: Call testing guide

## Performance

### Benchmarks
- **Call Setup Time**: < 2 seconds
- **ESL Event Processing**: < 100ms per event
- **CDR Database Insert**: < 50ms per call
- **Memory Usage**: < 100MB per service
- **CPU Usage**: < 10% under normal load

### Scalability
- **Concurrent Calls**: 1000+ simultaneous calls
- **Event Processing**: 10,000+ events/second
- **Buffer Capacity**: 10,000+ events
- **Auto-scaling**: Kubernetes HPA support

## Troubleshooting

### Common Issues

**Connection Problems:**
```bash
# Check FreeSWITCH ESL status
docker exec freeswitch fs_cli -x "esl status"

# Check ESL resilience logs
docker logs esl-resilience

# Verify network connectivity
docker exec esl-resilience ping freeswitch
```

**Performance Issues:**
```bash
# Check resource usage
docker stats

# Monitor metrics
curl http://localhost:9090/metrics

# Check buffer utilization
curl http://localhost:9090/metrics | grep esl_event_buffer_size
```

**Testing Issues:**
```bash
# Check baresip registration
docker exec baresip-client1 baresip -e "status"

# Verify SIP profiles
docker exec freeswitch fs_cli -x "sofia status profile internal"

# Monitor call events
docker exec freeswitch fs_cli -x "event plain CHANNEL_CREATE CHANNEL_ANSWER CHANNEL_HANGUP_COMPLETE"
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests (`go test ./internal/...`)
5. Ensure all tests pass
6. Submit a pull request

### Development Setup
```bash
# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Run tests with coverage
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out
```

## 📄 License

MIT License - see LICENSE file for details

## 🗺️ Roadmap

### Completed ✅
- [x] Core ESL resilience features
- [x] SIP call state machine
- [x] Event buffering and replay
- [x] Circuit breaker implementation
- [x] Prometheus monitoring
- [x] Comprehensive testing suite
- [x] Docker containerization
- [x] Kubernetes deployment
- [x] FreeSWITCH configuration
- [x] Baresip call testing

### In Progress 🚧
- [ ] Enhanced monitoring features
- [ ] Multi-tenant support

### Future Features 🔮
- [ ] WebRTC gateway integration
- [ ] Kamailio integration
- [ ] RTPengine media handling
- [ ] Advanced analytics dashboard
- [ ] Real-time alerting system
- [ ] GraphQL API
- [ ] gRPC interface

## 📞 Support

For support and questions:
- **Issues**: [GitHub Issues](https://github.com/nutcas3/esl-resilience/issues)
- **Documentation**: See [DEPLOYMENT.md](./DEPLOYMENT.md) for detailed guides
- **Testing**: See [testing/baresip/README.md](./testing/baresip/README.md) for call testing

---

**Built with ❤️ for the FreeSWITCH community**
