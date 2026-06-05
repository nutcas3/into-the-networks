# Multi-Tenant CDR System

An enterprise-grade multi-tenant Call Detail Record (CDR) system with advanced analytics, reporting, and fraud detection capabilities.

## Features

- **Multi-Tenant Architecture**: Complete tenant isolation at the database level with PostgreSQL partitioning
- **Advanced Analytics**: Real-time analytics engine with Redis caching for performance
- **REST API**: Gin-based REST API with JWT authentication and Role-Based Access Control (RBAC)
- **Reporting System**: Template-based report generation supporting PDF and Excel formats
- **Data Enrichment**: Automatic enrichment with geographic data, carrier information, and fraud detection
- **Audit Logging**: Comprehensive audit trail for all operations
- **Docker Support**: Complete containerization with Docker Compose for easy deployment

## Architecture

### Core Components

- **Database**: PostgreSQL with time-based and tenant-aware partitioning
- **Cache**: Redis for analytics caching
- **API**: Gin framework with middleware for authentication, RBAC, and logging
- **Analytics Engine**: Processes CDR data for daily/hourly statistics, call patterns, and quality metrics
- **Report Generator**: Generates PDF and Excel reports from templates
- **Enrichment Pipeline**: Adds geographic, carrier, and fraud detection data to CDRs

### Directory Structure

```
multi-tenant-cdr/
├── cmd/
│   └── api/
│       └── main.go          # Application entry point
├── internal/
│   ├── analytics/          # Analytics engine and caching
│   │   ├── engine.go
│   │   └── cache.go
│   ├── api/                # REST API handlers and server
│   │   ├── handlers.go
│   │   ├── routes.go
│   │   └── server.go
│   ├── auth/               # JWT and RBAC
│   │   ├── jwt.go
│   │   └── rbac.go
│   ├── enrichment/         # Data enrichment pipeline
│   │   ├── pipeline.go
│   │   ├── geo.go
│   │   ├── carrier.go
│   │   └── fraud.go
│   └── reports/            # Report generation
│       ├── templates.go
│       └── generator.go
├── sql/
│   └── schema.sql          # Database schema
├── Dockerfile              # Container definition
├── docker-compose.yml      # Multi-container orchestration
├── go.mod                  # Go module dependencies
└── README.md               # This file
```

## Getting Started

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 15 or higher
- Redis 7 or higher
- Docker and Docker Compose (optional)

### Quick Start with Docker Compose

1. Clone the repository:
```bash
git clone <repository-url>
cd multi-tenant-cdr
```

2. Start all services:
```bash
docker-compose up -d
```

3. The API will be available at `http://localhost:8080`

### Manual Setup

1. Install dependencies:
```bash
go mod download
```

2. Set up PostgreSQL:
```bash
psql -U postgres -f sql/schema.sql
```

3. Configure environment variables:
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=cdr_db
export DB_USER=cdr_user
export DB_PASSWORD=cdr_password
export REDIS_HOST=localhost
export REDIS_PORT=6379
export JWT_SECRET=your-secret-key
```

4. Run the application:
```bash
go run cmd/api/main.go
```

## API Endpoints

### Authentication

- `POST /api/v1/auth/login` - User login and token generation
- `POST /api/v1/auth/refresh` - Refresh JWT token

### CDR Operations

- `GET /api/v1/cdrs` - Query CDRs with filters
- `GET /api/v1/cdrs/:id` - Get specific CDR
- `POST /api/v1/cdrs` - Create new CDR

### Analytics

- `GET /api/v1/analytics/daily` - Daily analytics
- `GET /api/v1/analytics/hourly` - Hourly analytics
- `GET /api/v1/analytics/patterns` - Call patterns
- `GET /api/v1/analytics/quality` - Quality metrics
- `GET /api/v1/analytics/trends` - Trend analysis
- `GET /api/v1/analytics/statistics` - General statistics

### Reports

- `GET /api/v1/reports/templates` - List report templates
- `POST /api/v1/reports/templates` - Create report template
- `GET /api/v1/reports/templates/:id` - Get specific template
- `PUT /api/v1/reports/templates/:id` - Update template
- `DELETE /api/v1/reports/templates/:id` - Delete template
- `POST /api/v1/reports/generate` - Generate report
- `GET /api/v1/reports/:id` - Get generated report
- `GET /api/v1/reports/:id/download` - Download report file

### Health

- `GET /health` - Health check endpoint

## Database Schema

The system uses a multi-tenant PostgreSQL schema with the following key tables:

- `tenants` - Tenant configuration and settings
- `users` - User accounts with roles
- `roles` - Role definitions and permissions
- `cdrs` - Call detail records (partitioned by date and tenant)
- `cdr_partitions` - Partition management
- `daily_analytics` - Daily aggregated analytics
- `hourly_analytics` - Hourly aggregated analytics
- `call_patterns` - Call pattern analysis
- `quality_metrics` - Call quality metrics
- `report_templates` - Report template definitions
- `generated_reports` - Generated report records
- `audit_logs` - Audit trail
- `enriched_cdrs` - Enriched CDR data

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PORT` | PostgreSQL port | 5432 |
| `DB_NAME` | Database name | cdr_db |
| `DB_USER` | Database user | cdr_user |
| `DB_PASSWORD` | Database password | cdr_password |
| `REDIS_HOST` | Redis host | localhost |
| `REDIS_PORT` | Redis port | 6379 |
| `JWT_SECRET` | JWT signing secret | your-secret-key |
| `JWT_DURATION` | Token duration | 24h |
| `LOG_LEVEL` | Logging level | info |
| `ENVIRONMENT` | Environment | development |

## Testing

Run tests with:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

## Building

Build the application:
```bash
go build -o bin/api cmd/api/main.go
```

Build Docker image:
```bash
docker build -t multi-tenant-cdr:latest .
```

## Deployment

### Production Considerations

1. **Security**: Change default JWT secret and database passwords
2. **SSL**: Enable SSL for database connections in production
3. **Scaling**: The API can be scaled horizontally behind a load balancer
4. **Monitoring**: Integrate with Prometheus for metrics collection
5. **Logging**: Configure centralized logging (e.g., ELK stack)
6. **Backups**: Set up regular PostgreSQL backups

## License

MIT License

## Contributing

Contributions are welcome! Please submit pull requests or open issues for bugs and feature requests.
