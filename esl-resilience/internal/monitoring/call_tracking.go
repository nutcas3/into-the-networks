package monitoring

import "time"

func (em *EnhancedMonitor) RecordCallStart(tenantID string) {
	em.callMetrics.totalCalls.Inc()
	em.callMetrics.activeCalls.Inc()

	em.internalMetrics.mu.Lock()
	em.internalMetrics.totalCalls++
	em.internalMetrics.lastConnectionTime = time.Now()
	em.internalMetrics.mu.Unlock()

	if tenantID != "" {
		em.callMetrics.callsByTenant.WithLabelValues(tenantID).Inc()
		em.tenantMetrics.tenantCallVolume.WithLabelValues(tenantID).Inc()
	}
}

func (em *EnhancedMonitor) RecordCallEnd(tenantID, hangupCause string, duration time.Duration) {
	em.callMetrics.activeCalls.Dec()

	if duration > 0 {
		em.callMetrics.callDuration.Observe(duration.Seconds())
	}

	if hangupCause != "" {
		em.callMetrics.callsByHangupCause.WithLabelValues(hangupCause).Inc()
	}

	em.internalMetrics.mu.Lock()
	if duration > 0 {
		em.internalMetrics.callDurations = append(em.internalMetrics.callDurations, duration)
		if len(em.internalMetrics.callDurations) > 1000 {
			em.internalMetrics.callDurations = em.internalMetrics.callDurations[1:]
		}
	}

	if hangupCause == "NORMAL_CLEARING" {
		em.internalMetrics.successfulCalls++
		em.callMetrics.successfulCalls.Inc()
	} else {
		em.internalMetrics.failedCalls++
		em.callMetrics.failedCalls.Inc()
		if tenantID != "" {
			em.tenantMetrics.tenantErrors.WithLabelValues(tenantID, "call_failure").Inc()
		}
	}
	em.internalMetrics.mu.Unlock()
}

func (em *EnhancedMonitor) RecordCallSetupTime(tenantID string, setupTime time.Duration) {
	em.callMetrics.callSetupTime.Observe(setupTime.Seconds())
}

func (em *EnhancedMonitor) UpdateTenantMetrics(tenantID string, activeCalls int) {
	em.tenantMetrics.tenantResourceUsage.WithLabelValues(tenantID, "active_calls").Set(float64(activeCalls))
}

func (em *EnhancedMonitor) UpdateSLAMetrics(successRate, avgDuration, errorRate, availability float64) {
	em.slaMetrics.callSuccessRate.Set(successRate)
	em.slaMetrics.averageCallDuration.Set(avgDuration)
	em.slaMetrics.errorRate.Set(errorRate)
	em.slaMetrics.availability.Set(availability)
}
