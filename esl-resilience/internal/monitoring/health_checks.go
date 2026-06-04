package monitoring

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

func (em *EnhancedMonitor) initDefaultHealthChecks() {
	em.healthChecker.AddCheck("database", func(ctx context.Context) HealthCheck {
		start := time.Now()

		if em.database == nil {
			return HealthCheck{
				Status:   "unhealthy",
				Message:  "Database connection not initialized",
				Duration: time.Since(start),
			}
		}

		if err := em.database.Health(); err != nil {
			return HealthCheck{
				Status:   "unhealthy",
				Message:  fmt.Sprintf("Database health check failed: %v", err),
				Duration: time.Since(start),
			}
		}

		stats := em.database.Stats()
		openConnections := stats["open_connections"].(int64)
		inUse := stats["in_use"].(int64)

		if inUse > 20 { // Threshold for too many connections in use
			return HealthCheck{
				Status:   "degraded",
				Message:  fmt.Sprintf("Database OK but high connection usage: %d/%d connections in use", inUse, openConnections),
				Duration: time.Since(start),
			}
		}

		return HealthCheck{
			Status:   "healthy",
			Message:  fmt.Sprintf("Database connection OK: %d open connections, %d in use", openConnections, inUse),
			Duration: time.Since(start),
		}
	})

	em.healthChecker.AddCheck("esl_connection", func(ctx context.Context) HealthCheck {
		if em.GetCurrentConnectionStatus() {
			return HealthCheck{
				Status:   "healthy",
				Message:  "ESL connection OK",
				Duration: 0,
			}
		}
		return HealthCheck{
			Status:   "unhealthy",
			Message:  "ESL connection lost",
			Duration: 0,
		}
	})

	em.healthChecker.AddCheck("memory", func(ctx context.Context) HealthCheck {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		allocMB := float64(m.Alloc) / 1024 / 1024
		sysMB := float64(m.Sys) / 1024 / 1024
		numGC := m.NumGC

		if allocMB > 500 { // 500MB allocation threshold
			return HealthCheck{
				Status:   "degraded",
				Message:  fmt.Sprintf("High memory usage: %.1f MB allocated, %.1f MB system", allocMB, sysMB),
				Duration: 0,
			}
		}

		if allocMB > 1000 { // 1GB critical threshold
			return HealthCheck{
				Status:   "unhealthy",
				Message:  fmt.Sprintf("Critical memory usage: %.1f MB allocated, %.1f MB system", allocMB, sysMB),
				Duration: 0,
			}
		}

		return HealthCheck{
			Status:   "healthy",
			Message:  fmt.Sprintf("Memory usage OK: %.1f MB allocated, %d GC cycles", allocMB, numGC),
			Duration: 0,
		}
	})

	em.healthChecker.AddCheck("circuit_breaker", func(ctx context.Context) HealthCheck {
		start := time.Now()

		state := em.GetCircuitBreakerState()

		em.internalMetrics.mu.RLock()
		failures := em.internalMetrics.circuitBreakerFailures
		threshold := em.internalMetrics.circuitBreakerThreshold
		openTime := em.internalMetrics.circuitBreakerOpenTime
		em.internalMetrics.mu.RUnlock()

		switch state {
		case "open":
			timeUntilReset := em.internalMetrics.circuitBreakerTimeout - time.Since(openTime)
			return HealthCheck{
				Status:   "unhealthy",
				Message:  fmt.Sprintf("Circuit breaker OPEN - %d/%d failures, resets in %v", failures, threshold, timeUntilReset.Round(time.Second)),
				Duration: time.Since(start),
			}

		case "half-open":
			return HealthCheck{
				Status:   "degraded",
				Message:  fmt.Sprintf("Circuit breaker HALF-OPEN - testing with %d recent failures", failures),
				Duration: time.Since(start),
			}

		case "closed":
			if failures > 0 {
				return HealthCheck{
					Status:   "healthy",
					Message:  fmt.Sprintf("Circuit breaker CLOSED - %d recent failures (below threshold %d)", failures, threshold),
					Duration: time.Since(start),
				}
			}
			return HealthCheck{
				Status:   "healthy",
				Message:  "Circuit breaker CLOSED - no recent failures",
				Duration: time.Since(start),
			}

		default:
			return HealthCheck{
				Status:   "degraded",
				Message:  fmt.Sprintf("Circuit breaker in UNKNOWN state: %s", state),
				Duration: time.Since(start),
			}
		}
	})
}

func (em *EnhancedMonitor) initDefaultAlertRules() {
	rules := []*AlertRule{
		{
			Name:      "high_error_rate",
			Condition: "sla_error_rate",
			Threshold: 5.0,
			Duration:  5 * time.Minute,
			Severity:  "critical",
			Enabled:   true,
		},
		{
			Name:      "low_availability",
			Condition: "sla_availability",
			Threshold: 95.0,
			Duration:  2 * time.Minute,
			Severity:  "critical",
			Enabled:   true,
		},
		{
			Name:      "high_memory_usage",
			Condition: "process_memory_bytes",
			Threshold: 1000000000, // 1GB
			Duration:  10 * time.Minute,
			Severity:  "warning",
			Enabled:   true,
		},
		{
			Name:      "esl_connection_lost",
			Condition: "esl_connection_status",
			Threshold: 0.5,
			Duration:  1 * time.Minute,
			Severity:  "critical",
			Enabled:   true,
		},
		{
			Name:      "buffer_overflow",
			Condition: "esl_buffer_size",
			Threshold: 9000.0, // 90% of 10,000
			Duration:  5 * time.Minute,
			Severity:  "warning",
			Enabled:   true,
		},
	}

	for _, rule := range rules {
		em.alertManager.AddRule(rule)
	}
}

func (em *EnhancedMonitor) initDefaultAnalytics() {
	em.analyticsEngine.AddMetric("call_analytics_duration", []string{"tenant", "direction"}, []float64{1, 5, 15, 30, 60, 300, 600})
	em.analyticsEngine.AddMetric("call_analytics_quality", []string{"tenant", "codec"}, []float64{0.1, 0.5, 0.8, 0.9, 0.95, 0.99, 1.0})
	em.analyticsEngine.AddMetric("call_analytics_resource_usage", []string{"tenant", "resource"}, []float64{10, 25, 50, 75, 90, 95, 99})

	em.analyticsEngine.AddMetric("performance_analytics_latency", []string{"component", "operation"}, []float64{0.001, 0.01, 0.1, 1, 5, 10})
	em.analyticsEngine.AddMetric("performance_analytics_throughput", []string{"component"}, []float64{10, 50, 100, 500, 1000, 5000})
}
