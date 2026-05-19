# FreeSWITCH CDR Service with Custom ESL Library in Go

This project implements a local FreeSWITCH lab with a custom-built ESL (Event Socket Library) service in Go that captures Call Detail Records (CDRs) and stores them in PostgreSQL.

## Custom ESL Library Features

The project includes a complete custom ESL library in `internal/esl` with:

### Core Features
- **Inbound ESL Connection** - Connect to FreeSWITCH as a client
- **Outbound ESL Server** - Accept connections from FreeSWITCH
- **Event Listeners** - Subscribe to events by UUID (Unique-Id, Application-UUID, Job-UUID) or ALL events
- **Context Support** - Cancel requests using Go context
- **Command Abstraction** - All command types implement the Command interface
- **Custom Commands** - Implement BuildMessage() string for custom data

### Helper Functions
- **DTMF** - Send DTMF digits to calls
- **Call Origination** - Originate calls with aLeg/bLeg support
- **Call Answer/Hangup** - Answer and hangup calls
- **Audio Playback** - Play audio on channels

## Prerequisites

- Docker and Docker Compose (Docker Desktop or OrbStack must be running)
- Go 1.21 or higher
- A SIP softphone (Linphone, Zoiper, or any SIP client)

## Project Structure

```
.
├── docker-compose.yml          # Docker Compose configuration for FreeSWITCH and PostgreSQL
├── sql/
│   └── init.sql               # PostgreSQL schema initialization
├── freeswitch/
│   └── conf/                  # FreeSWITCH configuration files
│       ├── autoload_configs/
│       │   └── event_socket.conf.xml
│       ├── dialplan/
│       │   └── default.xml
│       ├── directory/
│       │   └── default/
│       │       ├── 1000.xml
│       │       └── 1001.xml
│       └── sip_profiles/
│           └── external.xml
├── internal/
│   └── esl/                  # Custom ESL library
│       ├── command.go        # Command interface and implementations
│       ├── event.go          # Event structure and parsing
│       ├── connection.go     # Connection handling
│       ├── outbound.go       # Outbound server
│       └── helpers.go        # Helper functions
├── main.go                    # CDR service using custom ESL library
├── config.json                # Service configuration
└── go.mod                     # Go module dependencies
```

## Setup Instructions

### Using Makefile

The project includes a Makefile for common operations:

```bash
make help              # Show available targets
make build            # Build the application
make run              # Run the application
make test             # Run tests
make clean            # Clean build artifacts
make docker-up        # Start Docker containers (FreeSWITCH + PostgreSQL)
make docker-down      # Stop Docker containers
make docker-logs      # View Docker container logs
make install-deps     # Install Go dependencies
make fmt              # Format Go code
make lint             # Run linter
```

### 1. Start FreeSWITCH and PostgreSQL

```bash
docker-compose up -d
```

This will start:
- FreeSWITCH on ports 5060/5080 (SIP), 8021 (ESL), and 16384-32768 (RTP)
- PostgreSQL on port 5432

### 2. Verify Services

Check that containers are running:
```bash
docker-compose ps
```

### 3. Configure SIP Softphones

Register two SIP extensions:

**Extension 1000:**
- Username: 1000
- Password: 1234
- Domain/Server: localhost (or your host IP if running in Docker)
- Port: 5080

**Extension 1001:**
- Username: 1001
- Password: 1234
- Domain/Server: localhost (or your host IP if running in Docker)
- Port: 5080

### 4. Test Call Flow

Make a call from 1000 to 1001 (or vice versa) to verify basic call flow.

### 5. Start the Go CDR Service

```bash
# Install dependencies
go mod download

# Run the service
go run main.go
```

The service will:
- Connect to FreeSWITCH via ESL on port 8021
- Authenticate with the configured password
- Enable event reception for CHANNEL_HANGUP_COMPLETE
- Listen for events using the custom ESL library
- Extract CDR data (caller, destination, duration, disposition)
- Store CDRs in PostgreSQL

### 6. Make Test Calls and Verify CDRs

Make calls between the extensions and check the database:

```bash
# Connect to PostgreSQL
docker exec -it postgres psql -U freeswitch -d freeswitch_cdr

# Query CDRs
SELECT * FROM cdr ORDER BY created_at DESC;
```

## Custom ESL Library Usage

### Inbound Connection

```go
opts := esl.Options{Password: "ClueCon"}
conn, err := esl.Dial("localhost:8021", opts)
if err != nil {
    log.Fatal(err)
}
defer conn.ExitAndClose()
```

### Event Listeners

```go
// Listen to all events
listenerID := conn.RegisterEventListener(esl.EventListenAll, func(event *esl.Event) {
    log.Println("Event:", event.GetName())
})

// Listen to specific channel
listenerID := conn.RegisterEventListener("channel-uuid", func(event *esl.Event) {
    log.Println("Channel event:", event.GetName())
})
```

### Call Origination

```go
aLeg := esl.Leg{CallURL: "user/1000"}
bLeg := esl.Leg{CallURL: "user/1001"}
response, err := conn.OriginateCall(ctx, false, aLeg, bLeg, nil)
```

### DTMF, Answer, Hangup, Playback

```go
// Answer call
conn.AnswerCall(ctx, uuid)

// Send DTMF
conn.DTMF(ctx, uuid, "1234")

// Play audio
conn.Playback(ctx, uuid, "misc/ivr-to_hear_screaming_monkeys.wav")

// Hangup call
conn.HangupCall(ctx, uuid, "NORMAL_CLEARING")
```

## Configuration

Edit `config.json` to change connection settings:

```json
{
  "freeswitch": {
    "host": "localhost",
    "port": "8021",
    "password": "ClueCon"
  },
  "postgresql": {
    "host": "localhost",
    "port": "5432",
    "user": "freeswitch",
    "password": "freeswitch_pass",
    "dbname": "freeswitch_cdr"
  }
}
```

## CDR Schema

The CDR table contains:
- `call_uuid`: Unique call identifier
- `caller`: Calling extension
- `destination`: Called extension
- `call_start_time`: When the call started
- `call_end_time`: When the call ended
- `duration_seconds`: Call duration in seconds
- `disposition`: Call result (answered, missed, failed)
- `hangup_cause`: FreeSWITCH hangup cause
- `created_at`: Record creation timestamp

## Troubleshooting

### FreeSWITCH not starting
```bash
docker-compose logs freeswitch
```

### PostgreSQL connection issues
```bash
docker-compose logs postgres
```

### ESL connection issues
- Verify FreeSWITCH is running: `docker-compose ps`
- Check ESL configuration in `freeswitch/conf/autoload_configs/event_socket.conf.xml`
- Ensure port 8021 is accessible

### SIP registration issues
- Check if FreeSWITCH is listening on the correct port: `docker exec freeswitch netstat -tulpn`
- Verify softphone configuration matches extension credentials
- Try using the host IP instead of localhost if running in Docker

## Learning Resources

Refer to `network-learning.md` for detailed learning resources on:
- FreeSWITCH ESL
- SIP fundamentals
- Kamailio routing
- RTPengine media handling
- And more telephony stack topics
