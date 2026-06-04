package monitoring

import (
	"context"
	"time"
)

func (em *EnhancedMonitor) StartSLAMonitoring(ctx context.Context) {
	go em.slaMonitoringLoop(ctx)
}

func (em *EnhancedMonitor) slaMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			em.updateSLAMetrics(startTime)
		}
	}
}

func (em *EnhancedMonitor) updateSLAMetrics(startTime time.Time) {
	em.slaMetrics.systemUptime.Add(time.Since(startTime).Seconds())

	em.internalMetrics.mu.RLock()
	totalCalls := em.internalMetrics.totalCalls
	successfulCalls := em.internalMetrics.successfulCalls
	callDurations := em.internalMetrics.callDurations
	em.internalMetrics.mu.RUnlock()

	if totalCalls > 0 {
		successRate := float64(successfulCalls) / float64(totalCalls) * 100
		em.slaMetrics.callSuccessRate.Set(successRate)
	}

	if len(callDurations) > 0 {
		var totalDuration time.Duration
		for _, duration := range callDurations {
			totalDuration += duration
		}
		avgDuration := totalDuration.Seconds() / float64(len(callDurations))
		em.slaMetrics.averageCallDuration.Set(avgDuration)
	}

	em.internalMetrics.mu.RLock()
	uptime := em.internalMetrics.connectionUptime
	downtime := em.internalMetrics.totalDowntime
	em.internalMetrics.mu.RUnlock()

	totalTime := uptime + downtime
	if totalTime > 0 {
		availability := float64(uptime) / float64(totalTime) * 100
		em.slaMetrics.availability.Set(availability)
	}

	if totalCalls > 0 {
		errorRate := float64(em.internalMetrics.failedCalls) / float64(totalCalls) * 100
		em.slaMetrics.errorRate.Set(errorRate)
	}
}
