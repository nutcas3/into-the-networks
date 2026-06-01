package esl

import (
	"sync"
	"time"
)

// Metrics tracks ESL client metrics
type Metrics struct {
	mu                        sync.RWMutex
	counters                  map[string]map[string]int64
	gauges                    map[string]map[string]float64
	histograms                map[string]map[string][]time.Duration
	connectionStatus          bool
	lastConnectionChange      time.Time
	totalEventsProcessed      int64
	totalConnectionFailures   int64
	totalReconnectionAttempts int64
}

// NewMetrics creates a new metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		counters:             make(map[string]map[string]int64),
		gauges:               make(map[string]map[string]float64),
		histograms:           make(map[string]map[string][]time.Duration),
		lastConnectionChange: time.Now(),
	}
}

// IncrementCounter increments a counter metric
func (m *Metrics) IncrementCounter(name string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.counters[name] == nil {
		m.counters[name] = make(map[string]int64)
	}

	key := m.labelsKey(labels)
	m.counters[name][key]++

	// Update specific counters
	switch name {
	case "esl_events_processed_total":
		m.totalEventsProcessed++
	case "esl_connection_failures_total":
		m.totalConnectionFailures++
	case "esl_reconnection_attempts_total":
		m.totalReconnectionAttempts++
	}
}

// SetGauge sets a gauge metric
func (m *Metrics) SetGauge(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.gauges[name] == nil {
		m.gauges[name] = make(map[string]float64)
	}

	key := m.labelsKey(labels)
	m.gauges[name][key] = value
}

// RecordHistogram records a histogram metric
func (m *Metrics) RecordHistogram(name string, value time.Duration, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.histograms[name] == nil {
		m.histograms[name] = make(map[string][]time.Duration)
	}

	key := m.labelsKey(labels)
	m.histograms[name][key] = append(m.histograms[name][key], value)

	// Keep only last 1000 values to prevent memory leaks
	if len(m.histograms[name][key]) > 1000 {
		m.histograms[name][key] = m.histograms[name][key][1:]
	}
}

// SetConnectionStatus sets the connection status
func (m *Metrics) SetConnectionStatus(connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connectionStatus = connected
	m.lastConnectionChange = time.Now()
}

// GetAll returns all metrics as a map
func (m *Metrics) GetAll() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]any{
		"counters":                    m.counters,
		"gauges":                      m.gauges,
		"histograms":                  m.histograms,
		"connection_status":           m.connectionStatus,
		"last_connection_change":      m.lastConnectionChange,
		"total_events_processed":      m.totalEventsProcessed,
		"total_connection_failures":   m.totalConnectionFailures,
		"total_reconnection_attempts": m.totalReconnectionAttempts,
	}
}

// GetCounter returns a specific counter value
func (m *Metrics) GetCounter(name string, labels map[string]string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.counters[name] == nil {
		return 0
	}

	key := m.labelsKey(labels)
	return m.counters[name][key]
}

// GetGauge returns a specific gauge value
func (m *Metrics) GetGauge(name string, labels map[string]string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.gauges[name] == nil {
		return 0
	}

	key := m.labelsKey(labels)
	return m.gauges[name][key]
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters = make(map[string]map[string]int64)
	m.gauges = make(map[string]map[string]float64)
	m.histograms = make(map[string]map[string][]time.Duration)
	m.totalEventsProcessed = 0
	m.totalConnectionFailures = 0
	m.totalReconnectionAttempts = 0
}

// labelsKey creates a key from labels
func (m *Metrics) labelsKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	key := ""
	for k, v := range labels {
		if key != "" {
			key += ","
		}
		key += k + "=" + v
	}

	return key
}
