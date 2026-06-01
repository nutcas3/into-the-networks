# ESL Resilience - Quick Start Guide

## Prerequisites

- Docker 20.10+
- Docker Compose 2.0+

## Quick Start

### 1. Start All Services

```bash
# Clone and navigate to the project
cd esl-resilience

# Start all services
docker-compose up -d

# Check service status
docker-compose ps
```

### 2. Access Services

- **ESL Resilience Metrics**: http://localhost:9090/metrics
- **Prometheus**: http://localhost:9091
- **Grafana**: http://localhost:3000 (admin/admin)
- **PostgreSQL**: localhost:5433 (freeswitch/freeswitch_pass)
- **Redis**: localhost:6379

### 3. Verify FreeSWITCH Connection

```bash
# Check FreeSWITCH logs
docker logs freeswitch

# Test ESL connection
docker exec esl-resilience curl -s http://localhost:9090/metrics | grep esl_connection_status
```

### 4. Monitor the System

```bash
# View ESL resilience logs
docker-compose logs -f esl-resilience

# View metrics
curl http://localhost:9090/metrics

# Access Grafana dashboards
open http://localhost:3000
```

## Configuration

### Environment Variables

Edit `docker-compose.yml` to modify:

```yaml
environment:
  - FREESWITCH_HOST=freeswitch
  - FREESWITCH_PORT=8021
  - FREESWITCH_PASSWORD=ClueCon
  - ESL_MAX_RETRIES=10
  - ESL_BUFFER_SIZE=10000
```

### FreeSWITCH Configuration

Place your FreeSWITCH configuration files in:
- `./freeswitch/conf/` - FreeSWITCH configuration
- `./freeswitch/log/` - FreeSWITCH logs
- `./freeswitch/recordings/` - Call recordings

## Troubleshooting

### Connection Issues

```bash
# Check FreeSWITCH ESL status
docker exec freeswitch fs_cli -x "esl status"

# Verify network connectivity
docker exec esl-resilience ping freeswitch

# Check ESL resilience logs
docker-compose logs esl-resilience
```

### Service Health

```bash
# Check all service health
docker-compose ps

# Restart specific service
docker-compose restart esl-resilience

# View resource usage
docker stats
```

### Database Issues

```bash
# Check PostgreSQL connection
docker exec -it postgres psql -U freeswitch -d freeswitch_cdr

# Reset database
docker-compose down -v
docker-compose up -d postgres
```

## Development

### Build and Test

```bash
# Build the application
go build ./cmd/main.go

# Run tests
go test ./internal/...

# Run with local FreeSWITCH
FREESWITCH_HOST=localhost go run ./cmd/main.go
```

### Logs and Debugging

```bash
# Enable debug logging
docker-compose exec esl-resilience env LOG_LEVEL=debug

# View real-time logs
docker-compose logs -f

# Access container shell
docker exec -it esl-resilience sh
```

## Production Deployment

### Scaling

```bash
# Scale ESL resilience service
docker-compose up -d --scale esl-resilience=3

# Check scaled instances
docker-compose ps
```

### Monitoring

- **Prometheus metrics**: http://localhost:9091/targets
- **Grafana dashboards**: http://localhost:3000/dashboards
- **Health checks**: All services include health checks

### Backup

```bash
# Backup volumes
docker run --rm -v esl-resilience_postgres-data:/data -v $(pwd):/backup ubuntu tar czf /backup/postgres-backup.tar.gz /data

# Restore volumes
docker run --rm -v esl-resilience_postgres-data:/data -v $(pwd):/backup ubuntu tar xzf /backup/postgres-backup.tar.gz -C /
```

## Next Steps

1. Configure FreeSWITCH ESL settings
2. Set up custom Grafana dashboards
3. Configure alerting rules
4. Deploy to production environment
5. Monitor and scale as needed

## Support

For detailed documentation, see `DEPLOYMENT.md` or check the application logs for troubleshooting information.
