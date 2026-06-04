package monitoring

import (
	"time"

	"github.com/sirupsen/logrus"
)

func (em *EnhancedMonitor) RecordCircuitBreakerFailure() {
	em.internalMetrics.mu.Lock()
	defer em.internalMetrics.mu.Unlock()

	em.internalMetrics.circuitBreakerFailures++
	
	if em.internalMetrics.circuitBreakerFailures >= em.internalMetrics.circuitBreakerThreshold {
		em.internalMetrics.circuitBreakerState = "open"
		em.internalMetrics.circuitBreakerOpenTime = time.Now()
		em.logger.WithFields(logrus.Fields{
			"failures":  em.internalMetrics.circuitBreakerFailures,
			"threshold": em.internalMetrics.circuitBreakerThreshold,
		}).Warn("Circuit breaker opened due to consecutive failures")
	}
}

func (em *EnhancedMonitor) RecordCircuitBreakerSuccess() {
	em.internalMetrics.mu.Lock()
	defer em.internalMetrics.mu.Unlock()

	em.internalMetrics.circuitBreakerFailures = 0
	
	if em.internalMetrics.circuitBreakerState == "open" || em.internalMetrics.circuitBreakerState == "half-open" {
		em.internalMetrics.circuitBreakerState = "closed"
		em.logger.Info("Circuit breaker closed after successful operation")
	}
}

func (em *EnhancedMonitor) GetCircuitBreakerState() string {
	em.internalMetrics.mu.RLock()
	defer em.internalMetrics.mu.RUnlock()

	if em.internalMetrics.circuitBreakerState == "open" {
		if time.Since(em.internalMetrics.circuitBreakerOpenTime) > em.internalMetrics.circuitBreakerTimeout {
			em.internalMetrics.mu.RUnlock()
			em.internalMetrics.mu.Lock()
			em.internalMetrics.circuitBreakerState = "half-open"
			em.internalMetrics.mu.Unlock()
			em.internalMetrics.mu.RLock()
			em.logger.Info("Circuit breaker transitioned to half-open state")
		}
	}

	return em.internalMetrics.circuitBreakerState
}

func (em *EnhancedMonitor) IsCircuitBreakerOpen() bool {
	state := em.GetCircuitBreakerState()
	return state == "open"
}

func (em *EnhancedMonitor) IsCircuitBreakerHalfOpen() bool {
	state := em.GetCircuitBreakerState()
	return state == "half-open"
}
