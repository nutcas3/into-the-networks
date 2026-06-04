package esl

import (
	"context"
	"sync"
	"time"
)

type HealthChecker struct {
	interval time.Duration
	timeout  time.Duration
	client   *Client
	mu       sync.RWMutex
	running  bool
}

func NewHealthChecker(interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		interval: interval,
		timeout:  timeout,
	}
}

func (hc *HealthChecker) Start(ctx context.Context, client *Client) {
	hc.mu.Lock()
	hc.client = client
	hc.running = true
	hc.mu.Unlock()
	
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.checkHealth()
		}
	}
}

func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	hc.running = false
	hc.mu.Unlock()
}

func (hc *HealthChecker) checkHealth() {
	hc.mu.RLock()
	client := hc.client
	running := hc.running
	hc.mu.RUnlock()
	
	if !running || client == nil {
		return
	}
	
	_, err := client.SendCommand("api status")
	
	if client.monitor != nil {
		if err != nil {
			client.monitor.RecordError(err)
		}
	}
	
	if client.metrics != nil {
		if err != nil {
			client.metrics.IncrementCounter("health_check_failures_total", nil)
		} else {
			client.metrics.IncrementCounter("health_check_successes_total", nil)
		}
	}
}
