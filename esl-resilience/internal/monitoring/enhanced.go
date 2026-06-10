package monitoring

import (
	"fmt"
	"time"

	"github.com/nutcas3/esl-resilience/internal/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

func NewEnhancedMonitor(database *db.Database) *EnhancedMonitor {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	em := &EnhancedMonitor{
		logger:   logger,
		database: database,
		internalMetrics: &InternalMetrics{
			connectionHistory:       make([]ConnectionEvent, 0),
			callDurations:           make([]time.Duration, 0),
			circuitBreakerState:     "closed",
			circuitBreakerThreshold: 5,
			circuitBreakerTimeout:   30 * time.Second,
		},
	}

	em.connectionStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "esl_connection_status",
		Help: "ESL connection status (1 = connected, 0 = disconnected)",
	})

	em.connectionAttempts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "esl_connection_attempts_total",
		Help: "Total number of ESL connection attempts",
	})

	em.connectionFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "esl_connection_failures_total",
		Help: "Total number of ESL connection failures",
	})

	em.reconnections = promauto.NewCounter(prometheus.CounterOpts{
		Name: "esl_reconnections_total",
		Help: "Total number of ESL reconnections",
	})

	em.bufferSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "esl_buffer_size",
		Help: "Current size of ESL event buffer",
	})

	em.bufferDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "esl_buffer_dropped_total",
		Help: "Total number of dropped ESL events",
	})

	em.circuitBreakerState = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "esl_circuit_breaker_state",
		Help: "ESL circuit breaker state (0 = closed, 1 = open, 2 = half-open)",
	})

	em.initCallMetrics()
	em.initPerformanceMetrics()
	em.initTenantMetrics()
	em.initSLAMetrics()

	em.alertManager = NewAlertManager(em)
	em.initDefaultAlertRules()

	em.healthChecker = NewHealthChecker()
	em.initDefaultHealthChecks()

	em.analyticsEngine = NewAnalyticsEngine()
	em.initDefaultAnalytics()

	return em
}

func (em *EnhancedMonitor) IsRunning() bool {
	return true
}

func (em *EnhancedMonitor) Stop() error {
	em.logger.Info("Enhanced monitor stopped")
	return nil
}

func NewAnalyticsEngine() *AnalyticsEngine {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &AnalyticsEngine{
		metrics: make(map[string]*prometheus.HistogramVec),
		logger:  logger,
	}
}

func (ae *AnalyticsEngine) AddMetric(name string, labels []string, buckets []float64) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	ae.metrics[name] = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    fmt.Sprintf("Analytics metric: %s", name),
		Buckets: buckets,
	}, labels)

	ae.logger.WithField("metric", name).Info("Analytics metric added")
}

func (ae *AnalyticsEngine) RecordMetric(name string, value float64, labels ...string) {
	ae.mu.RLock()
	metric, exists := ae.metrics[name]
	ae.mu.RUnlock()

	if !exists {
		return
	}

	metric.WithLabelValues(labels...).Observe(value)
}
