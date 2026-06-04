package monitoring

import (
	"context"
	"time"
)

func (em *EnhancedMonitor) RecordConnectionEvent(connected bool) {
	em.internalMetrics.mu.Lock()
	defer em.internalMetrics.mu.Unlock()

	now := time.Now()

	if connected {
		event := ConnectionEvent{
			Timestamp: now,
			Connected: true,
		}
		em.internalMetrics.connectionHistory = append(em.internalMetrics.connectionHistory, event)
		em.internalMetrics.lastConnectionTime = now

		em.connectionStatus.Set(1)
	} else {
		if !em.internalMetrics.lastConnectionTime.IsZero() {
			uptime := now.Sub(em.internalMetrics.lastConnectionTime)
			em.internalMetrics.connectionUptime += uptime

			if len(em.internalMetrics.connectionHistory) > 0 {
				lastIndex := len(em.internalMetrics.connectionHistory) - 1
				em.internalMetrics.connectionHistory[lastIndex].Duration = uptime
			}
		}

		event := ConnectionEvent{
			Timestamp: now,
			Connected: false,
		}
		em.internalMetrics.connectionHistory = append(em.internalMetrics.connectionHistory, event)

		em.connectionStatus.Set(0)
	}

	if len(em.internalMetrics.connectionHistory) > 1000 {
		em.internalMetrics.connectionHistory = em.internalMetrics.connectionHistory[1:]
	}
}

func (em *EnhancedMonitor) GetCurrentConnectionStatus() bool {
	em.internalMetrics.mu.RLock()
	defer em.internalMetrics.mu.RUnlock()

	if len(em.internalMetrics.connectionHistory) == 0 {
		return false
	}

	lastEvent := em.internalMetrics.connectionHistory[len(em.internalMetrics.connectionHistory)-1]
	return lastEvent.Connected
}

func (em *EnhancedMonitor) GetHealthStatus(ctx context.Context) *HealthStatus {
	return em.healthChecker.CheckAll(ctx)
}
