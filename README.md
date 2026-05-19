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
└── freeswitch-cdr/              # FreeSWITCH CDR learning project
    ├── Makefile                 # Build and run commands
    ├── README.md                # Implementation documentation
    ├── docker-compose.yml      # FreeSWITCH + PostgreSQL
    ├── config.json              # Service configuration
    ├── go.mod                   # Go module
    ├── main.go                  # Application entry point
    ├── internal/                # Internal packages
    │   ├── cdr/                # CDR models and repository
    │   ├── config/              # Configuration loading
    │   ├── db/                  # Database operations
    │   ├── esl/                 # Custom ESL library
    │   └── service/            # Service logic
    ├── freeswitch/              # FreeSWITCH configuration
    │   └── conf/
    └── sql/                     # Database schema
```

## Adding New Learning Projects

To add a new learning project to this repository:

1. Create a new directory for the project
2. Add a detailed README.md in the project directory
3. Update this main README.md to include the new project in the "Learning Projects" section
4. Add any relevant learning resources to the "Learning Resources" section
5. Document key learnings and takeaways
