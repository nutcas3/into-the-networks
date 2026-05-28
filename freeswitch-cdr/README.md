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
├── docker-compose.build.yml    # Working Docker Compose configuration for FreeSWITCH and PostgreSQL
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
│           ├── external.xml
│           └── internal.xml
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
docker compose -f docker-compose.build.yml up -d
```

This will start:
- FreeSWITCH on host-networked SIP ports 5060/5080, ESL on 127.0.0.1:8021, and RTP ports 16384-32768
- PostgreSQL on host port 5433

### 2. Verify Services

Check that containers are running:
```bash
docker compose -f docker-compose.build.yml ps
```

### 3. Configure SIP Softphones

Register two SIP extensions:

**Extension 1000:**
- Username: 1000
- Password: 1234
- Domain/Server: localhost (or your host IP if running in Docker)
- Port: 5060 for internal registration, or 5080 for external profile testing

**Extension 1001:**
- Username: 1001
- Password: 1234
- Domain/Server: localhost (or your host IP if running in Docker)
- Port: 5060 for internal registration, or 5080 for external profile testing

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
docker exec -it freeswitch-postgres-build psql -U freeswitch -d freeswitch_cdr

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
    "port": "5433",
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

## Testing the Setup

### ⚠️ Important ARM64/Mac Notice

**This setup has a known limitation on ARM64/Mac (Apple Silicon) systems:**
- FreeSWITCH's Event Socket Library (ESL) fails to bind to port 8021 on ARM64 due to an IPv6 binding bug
- This affects ALL FreeSWITCH Docker images on ARM64 platforms
- The issue occurs in `mod_event_socket.c` when trying to resolve IPv6 addresses

**Working Solutions:**
1. **Run on x86_64/AMD64 hardware** (Intel/AMD machine, cloud instance, GitHub Actions)
2. **Use XML-RPC API instead of ESL** (different protocol, no event socket needed)

### Quick Test (x86_64/AMD64)

```bash
# 1. Start services
docker compose -f docker-compose.build.yml up -d

# 2. Wait for FreeSWITCH to start (10-15 seconds)
sleep 15

# 3. Check if ESL port is listening
docker exec freeswitch-build sh -lc 'grep -i ":1F55" /proc/net/tcp /proc/net/tcp6 || true'

# 4. Test ESL connection manually
docker exec freeswitch-build fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x 'status'

# 5. Verify Sofia profiles
docker exec freeswitch-build fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x 'sofia status'

# 6. Start the Go CDR service
go run main.go

# 7. Make test calls with SIP softphones
#   - Extension 1000: password 1234, port 5060
#   - Extension 1001: password 1234, port 5060

# 8. Check CDRs in database
docker exec -it freeswitch-postgres-build psql -U freeswitch -d freeswitch_cdr -c "SELECT * FROM cdr ORDER BY created_at DESC LIMIT 5;"
```

### ARM64/Mac Testing (Limited)

```bash
# 1. Start services (FreeSWITCH will run but ESL won't work)
docker compose -f docker-compose.build.yml up -d

# 2. Verify FreeSWITCH is running (but ESL port won't bind)
docker logs freeswitch-build | grep -i "freeswitch.*ready"

# 3. Test SIP functionality (this works on ARM64)
#   - Register SIP extensions 1000 and 1001
#   - Make test calls between extensions

# 4. Check FreeSWITCH status
docker exec freeswitch-build fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x "status"
```

### Expected Results

**On x86_64/AMD64:**
- ✅ ESL port 8021 binds successfully
- ✅ Go CDR service connects and captures CDRs
- ✅ Database stores call records
- ✅ Full functionality working

**On ARM64/Mac:**
- ❌ ESL port 8021 does not bind
- ❌ Go CDR service cannot connect
- ✅ FreeSWITCH runs and SIP calls work
- ✅ Can use XML-RPC API as alternative

### Verification Commands

```bash
# Check container status
docker compose -f docker-compose.build.yml ps

# View FreeSWITCH logs
docker logs freeswitch-build --tail 20

# Check ESL module status
docker logs freeswitch-build 2>&1 | grep -i "event_socket"

# Test database connection
docker exec -it freeswitch-postgres-build psql -U freeswitch -d freeswitch_cdr -c "\dt"
```

## Troubleshooting

### FreeSWITCH not starting
```bash
docker compose -f docker-compose.build.yml logs freeswitch
```

### PostgreSQL connection issues
```bash
docker compose -f docker-compose.build.yml logs postgres
```

### ESL connection issues (x86_64 only)
- Verify FreeSWITCH is running: `docker compose -f docker-compose.build.yml ps`
- Check ESL port binding: `docker exec freeswitch-build sh -lc 'grep -i ":1F55" /proc/net/tcp /proc/net/tcp6 || true'`
- Use explicit CLI settings: `docker exec freeswitch-build fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x 'status'`
- Check ESL configuration in `freeswitch/conf/autoload_configs/event_socket.conf.xml`
- On ARM64: This is expected to fail due to IPv6 binding issue

### SIP registration issues
- Check if FreeSWITCH is listening on SIP ports: `docker exec freeswitch-build fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x 'sofia status'`
- Verify softphone configuration matches extension credentials
- Try using the host IP instead of localhost if running in Docker

## Learning Resources

Refer to `network-learning.md` for detailed learning resources on:
- FreeSWITCH ESL
- SIP fundamentals
- Kamailio routing
- RTPengine media handling
- And more telephony stack topics
