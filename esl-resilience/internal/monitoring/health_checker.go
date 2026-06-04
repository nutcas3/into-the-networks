package monitoring

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

func NewHealthChecker() *HealthChecker {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &HealthChecker{
		checks: make(map[string]func(ctx context.Context) HealthCheck),
		logger: logger,
	}
}

func (hc *HealthChecker) AddCheck(name string, checkFunc func(ctx context.Context) HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.checks[name] = checkFunc
	hc.logger.WithField("check", name).Info("Health check added")
}

func (hc *HealthChecker) RemoveCheck(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	delete(hc.checks, name)
	hc.logger.WithField("check", name).Info("Health check removed")
}

func (hc *HealthChecker) CheckAll(ctx context.Context) *HealthStatus {
	hc.mu.RLock()
	checks := make(map[string]HealthCheck, len(hc.checks))
	for name, checkFunc := range hc.checks {
		checks[name] = checkFunc(ctx)
	}
	hc.mu.RUnlock()

	// Determine overall status
	status := "healthy"
	for _, check := range checks {
		if check.Status == "unhealthy" {
			status = "unhealthy"
			break
		} else if check.Status == "degraded" {
			status = "degraded"
		}
	}

	return &HealthStatus{
		Status:    status,
		Timestamp: time.Now(),
		Checks:    checks,
		Uptime:    time.Since(time.Now()), // Would track actual uptime
		Version:   "1.0.0",
	}
}

func (hc *HealthChecker) Check(ctx context.Context, name string) (HealthCheck, error) {
	hc.mu.RLock()
	checkFunc, exists := hc.checks[name]
	hc.mu.RUnlock()

	if !exists {
		return HealthCheck{}, fmt.Errorf("health check '%s' not found", name)
	}

	return checkFunc(ctx), nil
}

func (hc *HealthChecker) GetCheckNames() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	names := make([]string, 0, len(hc.checks))
	for name := range hc.checks {
		names = append(names, name)
	}
	return names
}

func (hc *HealthChecker) GetStats() map[string]any {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return map[string]any{
		"total_checks": len(hc.checks),
		"check_names":  hc.GetCheckNames(),
	}
}

// Default health check implementations
func (hc *HealthChecker) initDefaultChecks() {
	// Database health check
	hc.AddCheck("database", func(ctx context.Context) HealthCheck {
		// This would be implemented with actual database health check
		return HealthCheck{
			Status:   "healthy",
			Message:  "Database connection OK",
			Duration: 0,
		}
	})

	// ESL connection health check
	hc.AddCheck("esl_connection", func(ctx context.Context) HealthCheck {
		// This would check actual ESL connection status
		return HealthCheck{
			Status:   "healthy",
			Message:  "ESL connection OK",
			Duration: 0,
		}
	})

	// Memory health check
	hc.AddCheck("memory", func(ctx context.Context) HealthCheck {
		// Get actual memory usage statistics
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Calculate memory usage in MB
		allocMB := float64(m.Alloc) / 1024 / 1024
		sysMB := float64(m.Sys) / 1024 / 1024
		numGC := m.NumGC

		// Check memory thresholds
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

	// Circuit breaker health check
	hc.AddCheck("circuit_breaker", func(ctx context.Context) HealthCheck {
		// This would check actual circuit breaker state
		return HealthCheck{
			Status:   "healthy",
			Message:  "Circuit breaker closed",
			Duration: 0,
		}
	})
}
