# Network Learning Repository

This repository documents learning journeys and implementations across various networking and telephony technologies.

## Learning Projects

### 1. FreeSWITCH CDR Service with Custom ESL Library
**Location**: `freeswitch-cdr/`

A complete telephony stack implementation from scratch, including:
- Local FreeSWITCH setup with Docker
- Custom ESL library implementation in Go (no external dependencies)
- CDR (Call Detail Record) capture and storage in PostgreSQL
- Proper Go project structure following best practices

**Key Learnings**:
- FreeSWITCH & ESL (Event Socket Library)
- Custom Go implementation of ESL wire protocol
- Event listeners with UUID filtering
- SIP registration and call flow
- CDR capture and disposition logic

**Documentation**: [freeswitch-cdr/README.md](freeswitch-cdr/README.md)

### 2. ESL Resilience & Call State Tracking
**Location**: `esl-resilience/`

Production-grade ESL client with comprehensive resilience features and complete SIP call state tracking, including:
- Automatic reconnection with exponential backoff
- Circuit breaker patterns for failure isolation
- Complete SIP call lifecycle state machine
- Event buffering during disconnections
- Production monitoring with Prometheus metrics
- Comprehensive testing suite (unit, integration, and call testing)
- Docker and Kubernetes deployment ready
- Real SIP client testing with Baresip

**Key Learnings**:
- Production-ready ESL client architecture
- Circuit breaker and resilience patterns
- SIP call state machine implementation
- Event buffering and replay mechanisms
- Prometheus monitoring integration
- Container orchestration with Kubernetes
- Real-world call testing methodologies
- FreeSWITCH production configuration

**Documentation**: [esl-resilience/README.md](esl-resilience/README.md)

## Learning Resources

### General Telephony Stack
- **[network-learning.md](network-learning.md)** - Comprehensive learning guide for telephony stack

### Key Resources Referenced
- [FreeSWITCH ESL Documentation](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Client-and-Developer-Interfaces/Event-Socket-Library/)
- [FreeSWITCH XML Dialplan](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Dialplan/XML-Dialplan/)
- [SIP Fundamentals](https://www.ietf.org/rfc/rfc3261.txt)

## Project Structure

```
networks/
├── README.md                    # This file - learning hub overview
├── network-learning.md         # Detailed learning resources
├── freeswitch-cdr/              # FreeSWITCH CDR learning project
│   ├── Makefile                 # Build and run commands
│   ├── README.md                # Implementation documentation
│   ├── docker-compose.yml      # FreeSWITCH + PostgreSQL
│   ├── config.json              # Service configuration
│   ├── go.mod                   # Go module
│   ├── main.go                  # Application entry point
│   ├── internal/                # Internal packages
│   │   ├── cdr/                # CDR models and repository
│   │   ├── config/              # Configuration loading
│   │   ├── db/                  # Database operations
│   │   ├── esl/                 # Custom ESL library
│   │   └── service/            # Service logic
│   ├── freeswitch/              # FreeSWITCH configuration
│   │   └── conf/
│   └── sql/                     # Database schema
└── esl-resilience/              # ESL Resilience & Call State Tracking
    ├── README.md                # Main project documentation
    ├── PROJECT_OVERVIEW.md      # Complete project overview
    ├── QUICK_START.md           # Quick start guide
    ├── DEPLOYMENT.md            # Deployment documentation
    ├── go.mod                   # Go module
    ├── cmd/
    │   └── main.go              # Application entry point
    ├── internal/                # Internal packages
    │   ├── server.go            # Main server logic
    │   ├── esl/                 # ESL client implementation
    │   │   ├── client.go        # Connection management
    │   │   ├── buffer.go        # Event buffering
    │   │   ├── circuit_breaker.go # Circuit breaker
    │   │   └── *_test.go        # Unit tests
    │   ├── state/               # SIP state machine
    │   │   ├── machine.go       # State machine logic
    │   │   └── *_test.go        # Unit tests
    │   ├── monitor/             # Prometheus monitoring
    │   │   └── prometheus.go    # Metrics implementation
    │   └── integration_test.go  # Integration tests
    ├── freeswitch/              # FreeSWITCH configuration
    │   └── conf/                # Production-ready configs
    ├── k8s/                     # Kubernetes manifests
    │   ├── namespace.yaml
    │   ├── deployment.yaml
    │   ├── hpa.yaml
    │   └── ingress.yaml
    ├── testing/
    │   └── baresip/            # Call testing setup
    ├── docker-compose.yml      # Local development
    ├── Dockerfile              # Container build
    └── sql/                     # Database schema
```

## Adding New Learning Projects

To add a new learning project to this repository:

1. Create a new directory for the project
2. Add a detailed README.md in the project directory
3. Update this main README.md to include the new project in the "Learning Projects" section
4. Add any relevant learning resources to the "Learning Resources" section
5. Document key learnings and takeaways
