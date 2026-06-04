package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/nutcas3/esl-resilience/internal/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type InternalMetrics struct {
	// SLA tracking
	totalCalls         int64
	successfulCalls    int64
	failedCalls        int64
	connectionUptime   time.Duration
	totalDowntime      time.Duration
	lastConnectionTime time.Time
	connectionHistory  []ConnectionEvent
	callDurations      []time.Duration

	// Circuit breaker tracking
	circuitBreakerState     string // "closed", "open", "half-open"
	circuitBreakerOpenTime  time.Time
	circuitBreakerFailures  int64
	circuitBreakerThreshold int64         // Open after 5 consecutive failures
	circuitBreakerTimeout   time.Duration // Reset timeout

	mu sync.RWMutex
}

type ConnectionEvent struct {
	Timestamp time.Time     `json:"timestamp"`
	Connected bool          `json:"connected"`
	Duration  time.Duration `json:"duration"`
}

type EnhancedMonitor struct {
	// Basic metrics
	connectionStatus    prometheus.Gauge
	connectionAttempts  prometheus.Counter
	connectionFailures  prometheus.Counter
	reconnections       prometheus.Counter
	bufferSize          prometheus.Gauge
	bufferDropped       prometheus.Counter
	circuitBreakerState prometheus.Gauge

	// Enhanced metrics
	callMetrics        *CallMetrics
	performanceMetrics *PerformanceMetrics
	tenantMetrics      *TenantMetrics
	slaMetrics         *SLAMetrics

	// Alerting
	alertManager  *AlertManager
	healthChecker *HealthChecker

	// Analytics
	analyticsEngine *AnalyticsEngine

	// Database connection for health checks
	database *db.Database

	// Internal tracking for SLA calculations
	internalMetrics *InternalMetrics

	logger *logrus.Logger
	mu     sync.RWMutex
}

type CallMetrics struct {
	totalCalls         prometheus.Counter
	activeCalls        prometheus.Gauge
	successfulCalls    prometheus.Counter
	failedCalls        prometheus.Counter
	callDuration       prometheus.Histogram
	callSetupTime      prometheus.Histogram
	callsByHangupCause *prometheus.CounterVec
	callsByTenant      *prometheus.CounterVec
}

type PerformanceMetrics struct {
	memoryUsage     prometheus.Gauge
	cpuUsage        prometheus.Gauge
	goroutineCount  prometheus.Gauge
	gcDuration      prometheus.Histogram
	dbConnections   prometheus.Gauge
	dbQueryDuration prometheus.Histogram
	eslEventLatency prometheus.Histogram
}

type TenantMetrics struct {
	activeTenants       prometheus.Gauge
	tenantCallVolume    *prometheus.CounterVec
	tenantErrors        *prometheus.CounterVec
	tenantResourceUsage *prometheus.GaugeVec
}

type SLAMetrics struct {
	callSuccessRate     prometheus.Gauge
	averageCallDuration prometheus.Gauge
	systemUptime        prometheus.Counter
	errorRate           prometheus.Gauge
	availability        prometheus.Gauge
}

type Alert struct {
	ID          string            `json:"id"`
	Level       string            `json:"level"` // critical, warning, info
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Timestamp   time.Time         `json:"timestamp"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Rules       []AlertRule       `json:"rules"`
	Status      string            `json:"status"` // firing, resolved
}

type AlertRule struct {
	Name      string        `json:"name"`
	Condition string        `json:"condition"`
	Threshold float64       `json:"threshold"`
	Duration  time.Duration `json:"duration"`
	Severity  string        `json:"severity"`
	Enabled   bool          `json:"enabled"`
}

type AlertManager struct {
	rules              map[string]*AlertRule
	alerts             map[string]*Alert
	handlers           []AlertHandler
	conditionStartTime map[string]time.Time // Track when conditions first became true
	monitor            *EnhancedMonitor     // Reference to fetch actual metrics
	mu                 sync.RWMutex
	logger             *logrus.Logger
}

type AlertHandler interface {
	Handle(ctx context.Context, alert *Alert) error
}

type HealthStatus struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]HealthCheck `json:"checks"`
	Uptime    time.Duration          `json:"uptime"`
	Version   string                 `json:"version"`
}

type HealthCheck struct {
	Status    string        `json:"status"`
	Message   string        `json:"message"`
	LastCheck time.Time     `json:"last_check"`
	Duration  time.Duration `json:"duration"`
}

type HealthChecker struct {
	checks map[string]func(ctx context.Context) HealthCheck
	logger *logrus.Logger
	mu     sync.RWMutex
}

type AnalyticsEngine struct {
	metrics map[string]*prometheus.HistogramVec
	logger  *logrus.Logger
	mu      sync.RWMutex
}
