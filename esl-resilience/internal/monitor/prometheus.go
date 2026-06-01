package monitor

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// PrometheusMonitor implements monitoring with Prometheus metrics
type PrometheusMonitor struct {
	registry *prometheus.Registry
	server   *http.Server
	logger   *logrus.Logger

	// Metrics
	connectionStatus     prometheus.Gauge
	eventsProcessed      prometheus.Counter
	connectionFailures   prometheus.Counter
	reconnectionAttempts prometheus.Counter
	activeCalls          prometheus.Gauge
	bufferSize           prometheus.Gauge
	bufferDropped        prometheus.Counter
}

// Config holds Prometheus monitor configuration
type Config struct {
	Port int `yaml:"port"`
}

// NewPrometheusMonitor creates a new Prometheus monitor
func NewPrometheusMonitor() *PrometheusMonitor {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	registry := prometheus.NewRegistry()

	monitor := &PrometheusMonitor{
		registry: registry,
		logger:   logger,
	}

	// Initialize metrics
	monitor.initMetrics()
	monitor.registry.MustRegister(monitor.getAllMetrics()...)

	return monitor
}

// initMetrics initializes all Prometheus metrics
func (m *PrometheusMonitor) initMetrics() {
	m.connectionStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "esl_connection_status",
		Help: "Current ESL connection status (1 = connected, 0 = disconnected)",
	})

	m.eventsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "esl_events_processed_total",
		Help: "Total number of ESL events processed",
	})

	m.connectionFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "esl_connection_failures_total",
		Help: "Total number of ESL connection failures",
	})

	m.reconnectionAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "esl_reconnection_attempts_total",
		Help: "Total number of ESL reconnection attempts",
	})

	m.activeCalls = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sip_active_calls",
		Help: "Number of active SIP calls",
	})

	m.bufferSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "esl_event_buffer_size",
		Help: "Current size of the ESL event buffer",
	})

	m.bufferDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "esl_event_buffer_dropped_total",
		Help: "Total number of events dropped from buffer",
	})
}

// getAllMetrics returns all metrics for registration
func (m *PrometheusMonitor) getAllMetrics() []prometheus.Collector {
	return []prometheus.Collector{
		m.connectionStatus,
		m.eventsProcessed,
		m.connectionFailures,
		m.reconnectionAttempts,
		m.activeCalls,
		m.bufferSize,
		m.bufferDropped,
	}
}

// Start starts the Prometheus metrics server
func (m *PrometheusMonitor) Start() error {
	if m.server != nil {
		return fmt.Errorf("monitor already started")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	m.server = &http.Server{
		Addr:    ":9090",
		Handler: mux,
	}

	m.logger.Info("Starting Prometheus metrics server on :9090")
	return m.server.ListenAndServe()
}

// Stop stops the Prometheus metrics server
func (m *PrometheusMonitor) Stop() error {
	if m.server == nil {
		return nil
	}
	return m.server.Close()
}

// RecordConnection records connection status
func (m *PrometheusMonitor) RecordConnection(status bool) {
	if status {
		m.connectionStatus.Set(1)
	} else {
		m.connectionStatus.Set(0)
	}
}

// RecordEvent records an event type
func (m *PrometheusMonitor) RecordEvent(eventType string) {
	m.eventsProcessed.Inc()
}

// RecordReconnection records a reconnection attempt
func (m *PrometheusMonitor) RecordReconnection(attempt int) {
	m.reconnectionAttempts.Inc()
}

// RecordError records an error (currently just logs)
func (m *PrometheusMonitor) RecordError(err error) {
	m.logger.WithError(err).Error("ESL error recorded")
}

// IncrementCounter increments a counter metric
func (m *PrometheusMonitor) IncrementCounter(name string, labels map[string]string) {
	switch name {
	case "esl_events_processed_total":
		m.eventsProcessed.Inc()
	case "esl_connection_failures_total":
		m.connectionFailures.Inc()
	case "esl_reconnection_attempts_total":
		m.reconnectionAttempts.Inc()
	case "esl_event_buffer_dropped_total":
		m.bufferDropped.Inc()
	default:
		m.logger.WithField("metric", name).Warn("Unknown counter metric")
	}
}

// SetGauge sets a gauge metric
func (m *PrometheusMonitor) SetGauge(name string, value float64, labels map[string]string) {
	switch name {
	case "esl_connection_status":
		m.connectionStatus.Set(value)
	case "sip_active_calls":
		m.activeCalls.Set(value)
	case "esl_event_buffer_size":
		m.bufferSize.Set(value)
	default:
		m.logger.WithField("metric", name).Warn("Unknown gauge metric")
	}
}

// SetActiveCalls sets the number of active calls
func (m *PrometheusMonitor) SetActiveCalls(count int) {
	m.activeCalls.Set(float64(count))
}

// SetBufferSize sets the current buffer size
func (m *PrometheusMonitor) SetBufferSize(size int) {
	m.bufferSize.Set(float64(size))
}

// IncrementBufferDropped increments the buffer dropped counter
func (m *PrometheusMonitor) IncrementBufferDropped() {
	m.bufferDropped.Inc()
}

// IsRunning returns true if the monitor server is running
func (m *PrometheusMonitor) IsRunning() bool {
	return m.server != nil
}
