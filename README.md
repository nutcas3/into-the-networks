# Network Learning Repository

This repository documents learning journeys and implementations across various networking, telephony, and distributed systems technologies.

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
- Docker and Kubernetes deployment ready
- Real SIP client testing with Baresip

**Key Learnings**:
- Production-ready ESL client architecture
- Circuit breaker and resilience patterns
- SIP call state machine implementation
- Event buffering and replay mechanisms
- Prometheus monitoring integration
- Container orchestration with Kubernetes

**Documentation**: [esl-resilience/README.md](esl-resilience/README.md)

### 3. Kamailio Router & Load Balancer
**Location**: `kamailio-router/`

SIP routing and load balancing with Kamailio, including:
- Dispatcher-based load balancing across FreeSWITCH nodes
- JSON-RPC management interface
- NAT traversal and registrar support
- Carrier routing and failover logic

**Key Learnings**:
- Kamailio configuration and scripting
- SIP load balancing strategies
- Dispatcher module and dynamic routing
- JSON-RPC for runtime management
- NAT traversal techniques

**Documentation**: [kamailio-router/README.md](kamailio-router/README.md)

### 4. Multi-Tenant CDR Service
**Location**: `multi-tenant-cdr/`

Multi-tenant call detail record management system with:
- Tenant-isolated CDR storage and querying
- REST API for CDR ingestion and retrieval
- Aggregated reporting and analytics
- PostgreSQL with row-level security

**Key Learnings**:
- Multi-tenant architecture patterns
- Data isolation strategies
- CDR aggregation and reporting
- REST API design for telephony data

**Documentation**: [multi-tenant-cdr/README.md](multi-tenant-cdr/README.md)

### 5. RTPengine Media Proxy
**Location**: `rtpengine-media/`

RTP media proxy for handling media streams, including:
- UDP ng-protocol communication with RTPengine
- RTP relay and transcoding orchestration
- Recording and media manipulation

**Key Learnings**:
- RTP/RTCP protocol fundamentals
- RTPengine ng-protocol over UDP
- Media stream relay and proxying
- Real-time media handling

**Documentation**: [rtpengine-media/README.md](rtpengine-media/README.md)

### 6. WebRTC Gateway
**Location**: `webrtc-gateway/`

Browser-to-SIP bridge enabling WebRTC clients to connect to the telephony platform:
- WebRTC peer management and signaling
- SIP translation for browser clients
- RTPengine integration for media handling
- WebSocket-based signaling

**Key Learnings**:
- WebRTC protocol stack (ICE, DTLS, SRTP)
- Browser-to-SIP bridging
- Signaling server design
- Media path negotiation

**Documentation**: [webrtc-gateway/README.md](webrtc-gateway/README.md)

### 7. VoIP Push Notifications
**Location**: `voip-push/`

Push notification service for mobile VoIP clients:
- FCM (Firebase Cloud Messaging) integration
- APNs (Apple Push Notification service) integration
- Device registration and token management
- Push orchestration for incoming calls

**Key Learnings**:
- FCM and APNs API integration
- Push token lifecycle management
- Wake-up patterns for VoIP apps
- Cross-platform push orchestration

**Documentation**: [voip-push/README.md](voip-push/README.md)

### 8. WireGuard VoIP VPN
**Location**: `wireguard-voip/`

Zero-rated VPN service for VoIP traffic using WireGuard:
- WireGuard peer provisioning and management
- Tunnel metrics and monitoring
- Zero-rated data routing for VoIP
- REST API for peer lifecycle

**Key Learnings**:
- WireGuard protocol and configuration
- VPN tunnel management
- Network namespace and routing
- Peer provisioning automation

**Documentation**: [wireguard-voip/README.md](wireguard-voip/README.md)

### 9. AMD System (Answering Machine Detection)
**Location**: `amd-system/`

ML-powered call classification system:
- Go API service for call session management
- Python FastAPI ML service for audio classification
- MFCC feature extraction with librosa
- Scikit-learn models (RandomForest/LogisticRegression)
- Real-time audio streaming and classification

**Key Learnings**:
- Answering machine detection algorithms
- MFCC audio feature extraction
- Python-Go service integration
- ML model serving patterns
- Real-time audio classification pipeline

**Documentation**: [amd-system/README.md](amd-system/README.md)

## System Design Framework

### The Smartphone System Design Framework
**Location**: `phone-system-design-framework.md`

A comprehensive framework using smartphone hardware architecture as a blueprint for designing any distributed system at scale. Covers:
- Compute orchestration (big.LITTLE → workload segregation)
- Memory hierarchy (caching strategies)
- IPC and message brokers (System Bus → service mesh)
- Fault tolerance (baseband resilience → circuit breakers)
- Security (sandboxing → zero-trust microservices)
- Power management (Doze mode → cost optimization)
- Observability (diagnostics → golden signals)
- Deployment (OTA updates → blue/green deploys)

**Read it here**: [phone-system-design-framework.md](phone-system-design-framework.md)

## Learning Resources

### General Telephony Stack
- **[network-learning.md](network-learning.md)** - Comprehensive learning guide for telephony stack
- **[projects.md](projects.md)** - Project tracking and completion status

### Key Resources Referenced
- [FreeSWITCH ESL Documentation](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Client-and-Developer-Interfaces/Event-Socket-Library/)
- [FreeSWITCH XML Dialplan](https://developer.signalwire.com/freeswitch/FreeSWITCH-Explained/Dialplan/XML-Dialplan/)
- [SIP Fundamentals (RFC 3261)](https://www.ietf.org/rfc/rfc3261.txt)
- [Kamailio Documentation](https://www.kamailio.org/w/documentation/)
- [RTPengine Documentation](https://rtpengine.readthedocs.io/)
- [WebRTC Specification](https://webrtc.org/)

## Project Structure

```
networks/
├── README.md                           # This file - learning hub overview
├── network-learning.md                # Detailed learning resources
├── projects.md                        # Project tracking and status
├── phone-system-design-framework.md   # System design interview framework
│
├── freeswitch-cdr/                    # Project 1: FreeSWITCH CDR + Custom ESL
├── esl-resilience/                    # Project 2: Resilient ESL Client
├── kamailio-router/                   # Project 3: SIP Router / Load Balancer
├── multi-tenant-cdr/                  # Project 4: Multi-Tenant CDR Service
├── rtpengine-media/                   # Project 5: RTP Media Proxy
├── webrtc-gateway/                    # Project 6: Browser-to-SIP Bridge
├── voip-push/                         # Project 7: Mobile Push Notifications
├── wireguard-voip/                    # Project 8: VoIP VPN
└── amd-system/                        # Project 9: ML Call Classification
```

## Technology Stack Summary

| Technology | Projects |
|------------|----------|
| **Go** | All projects - services, clients, protocols |
| **Python** | AMD ML service (FastAPI, scikit-learn, librosa) |
| **FreeSWITCH** | Projects 1, 2 - SIP/media server |
| **Kamailio** | Project 3 - SIP router/load balancer |
| **RTPengine** | Projects 5, 6 - RTP media proxy |
| **PostgreSQL** | Projects 1, 2, 4 - persistent storage |
| **Redis** | Caching, session management |
| **Docker** | All projects - containerization |
| **Kubernetes** | Projects 2, 8 - production orchestration |
| **Prometheus** | Projects 2, 8 - metrics and monitoring |
| **gRPC/REST** | Projects 3, 4, 6, 7, 8, 9 - service APIs |
| **WebSocket** | Project 6 - WebRTC signaling |
| **WireGuard** | Project 8 - VPN tunnels |
| **Firebase/APNs** | Project 7 - push notifications |

## Getting Started

Each project contains its own README with setup instructions. General prerequisites:

- Go 1.26.3+
- Docker & Docker Compose
- PostgreSQL (or use provided docker-compose)
- Make (for build automation)

## Adding New Learning Projects

To add a new learning project to this repository:

1. Create a new directory for the project
2. Add a detailed README.md in the project directory
3. Update this main README.md to include the new project
4. Add any relevant learning resources
5. Document key learnings and takeaways
