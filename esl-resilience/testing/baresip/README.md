# Baresip Call Testing for ESL Resilience

This directory contains a comprehensive testing setup using Baresip SIP clients to test the ESL Resilience system with real call scenarios.

## Overview

The testing environment includes:
- **FreeSWITCH**: SIP server with ESL enabled
- **ESL Resilience**: Main application being tested
- **2 Baresip Clients**: For making test calls
- **PostgreSQL**: CDR database
- **Prometheus**: Metrics collection

## Quick Start

### 1. Start Test Environment

```bash
cd testing/baresip
./test-calls.sh
```

This will:
- Start all services
- Verify connectivity
- Run basic tests
- Display monitoring URLs

### 2. Manual Testing

After the environment is running, you can perform manual tests:

#### Access Baresip Clients

```bash
# Client 1 (Users 1000, 1001)
docker exec -it baresip-client1 baresip

# Client 2 (Users 1002, 1003)
docker exec -it baresip-client2 baresip
```

#### Make Test Calls

Within baresip, use these commands:
```
# Call from client1 to client2
dial 1002

# Call test extensions
dial 1234    # ESL resilience test
dial 9196    # Echo test
dial 9197    # Milliwatt test
```

#### Monitor ESL Events

```bash
# Monitor all FreeSWITCH events
docker exec freeswitch-test fs_cli -x "event plain ALL"

# Check ESL connection status
docker exec freeswitch-test fs_cli -x "esl status"
```

## Configuration

### Baresip Profiles

**Client 1 Configuration:**
- User 1000: `sip:1000@localhost` (password: 1000password)
- User 1001: `sip:1001@localhost` (password: 1001password)

**Client 2 Configuration:**
- User 1002: `sip:1002@localhost` (password: 1002password)
- User 1003: `sip:1003@localhost` (password: 1003password)

### Test Extensions

| Extension | Purpose | Expected Behavior |
|-----------|---------|-------------------|
| 1234 | ESL Resilience Test | Fires ESL events, plays message |
| 9196 | Echo Test | Echoes audio back |
| 9197 | Milliwatt Test | Plays test tone |
| 9198 | Hangup Test | Immediate hangup |

## Test Scenarios

### 1. Basic Call Flow Test
1. Start both baresip clients
2. Client 1 calls Client 2 (dial 1002)
3. Verify call establishment
4. Monitor ESL events
5. Check CDR records

### 2. ESL Resilience Test
1. Client 1 calls extension 1234
2. Monitor ESL event processing
3. Check metrics for event handling
4. Verify buffer behavior

### 3. Connection Failure Test
1. Stop FreeSWITCH temporarily
2. Verify ESL resilience reconnection
3. Check circuit breaker behavior
4. Resume calls after reconnection

### 4. High Volume Test
1. Make multiple concurrent calls
2. Monitor system performance
3. Check buffer overflow handling
4. Verify CDR database performance

## Monitoring

### ESL Resilience Metrics

Access metrics at: http://localhost:9090/metrics

Key metrics to monitor:
- `esl_connection_status` - Connection state
- `esl_events_processed_total` - Total events processed
- `sip_active_calls` - Active call count
- `esl_connection_failures_total` - Connection failures
- `esl_event_buffer_size` - Buffer utilization

### Prometheus Dashboard

Access Prometheus at: http://localhost:9091

### CDR Database

```bash
# Check call records
docker exec postgres-test psql -U freeswitch -d freeswitch_cdr -c "SELECT * FROM cdr ORDER BY start_timestamp DESC LIMIT 10;"

# Call statistics
docker exec postgres-test psql -U freeswitch -d freeswitch_cdr -c "SELECT COUNT(*) as total_calls, AVG(duration) as avg_duration FROM cdr;"
```

## Troubleshooting

### Common Issues

**Baresip Registration Fails:**
```bash
# Check FreeSWITCH status
docker exec freeswitch-test fs_cli -x "status"

# Check SIP profile
docker exec freeswitch-test fs_cli -x "sofia status profile internal"
```

**ESL Connection Issues:**
```bash
# Check ESL status
docker exec freeswitch-test fs_cli -x "esl status"

# Check ESL logs
docker logs esl-resilience-test
```

**No Audio in Calls:**
```bash
# Check RTP ports
docker exec freeswitch-test fs_cli -x "rtp status"

# Check network connectivity
docker network ls
```

### Debug Commands

```bash
# View all logs
docker-compose -f docker-compose.test.yml logs

# Follow specific service logs
docker-compose -f docker-compose.test.yml logs -f esl-resilience

# Check service status
docker-compose -f docker-compose.test.yml ps
```

## Advanced Testing

### Load Testing

For high-volume testing, you can:
1. Scale baresip clients
2. Use automated call scripts
3. Monitor system resources
4. Check performance metrics

### Failure Injection

Test resilience by:
1. Stopping/starting FreeSWITCH
2. Network partition simulation
3. Resource exhaustion testing
4. Database connection failure

## Cleanup

```bash
# Stop test environment
docker-compose -f docker-compose.test.yml down

# Remove volumes (optional)
docker-compose -f docker-compose.test.yml down -v

# Clean up containers
docker system prune -f
```

## Integration with CI/CD

The test setup can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions step
- name: Run Call Tests
  run: |
    cd testing/baresip
    ./test-calls.sh
    # Add assertions for test results
```

## Performance Benchmarks

Typical performance metrics:
- **Call Setup Time**: < 2 seconds
- **ESL Event Processing**: < 100ms per event
- **CDR Database Insert**: < 50ms per call
- **Memory Usage**: < 100MB per service
- **CPU Usage**: < 10% under normal load

## Next Steps

1. **Automated Test Scripts**: Create automated call sequences
2. **Performance Testing**: Load testing with multiple concurrent calls
3. **Monitoring Dashboards**: Grafana dashboards for visualization
4. **Alert Configuration**: Set up alerts for system failures
5. **Documentation**: Expand test case documentation
