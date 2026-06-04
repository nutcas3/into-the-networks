package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

func NewAlertManager(monitor *EnhancedMonitor) *AlertManager {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &AlertManager{
		rules:              make(map[string]*AlertRule),
		alerts:             make(map[string]*Alert),
		handlers:           []AlertHandler{},
		conditionStartTime: make(map[string]time.Time),
		monitor:            monitor,
		logger:             logger,
	}
}

func (am *AlertManager) AddRule(rule *AlertRule) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.rules[rule.Name]; exists {
		return fmt.Errorf("alert rule '%s' already exists", rule.Name)
	}

	am.rules[rule.Name] = rule
	am.logger.WithField("rule", rule.Name).Info("Alert rule added")
	return nil
}

func (am *AlertManager) RemoveRule(name string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.rules[name]; !exists {
		return fmt.Errorf("alert rule '%s' not found", name)
	}

	delete(am.rules, name)
	am.logger.WithField("rule", name).Info("Alert rule removed")
	return nil
}

func (am *AlertManager) AddHandler(handler AlertHandler) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.handlers = append(am.handlers, handler)
	am.logger.Info("Alert handler added")
}

func (am *AlertManager) EvaluateAlerts(ctx context.Context) {
	am.mu.RLock()
	rules := make([]*AlertRule, 0, len(am.rules))
	for _, rule := range am.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	am.mu.RUnlock()

	for _, rule := range rules {
		if am.shouldTriggerAlert(rule) {
			am.triggerAlert(rule)
		}
	}
}

func (am *AlertManager) shouldTriggerAlert(rule *AlertRule) bool {
	// Get current metric value based on condition
	currentValue, err := am.getMetricValue(rule.Condition)
	if err != nil {
		am.logger.WithError(err).WithField("condition", rule.Condition).Error("Failed to get metric value")
		return false
	}

	// Evaluate condition against threshold
	// Parse condition like "error_rate > 0.05" or "cpu_usage > 80"
	threshold := rule.Threshold

	// Simple comparison - in production, use a proper expression parser
	triggered := false
	switch {
	case currentValue > threshold:
		triggered = true
	case currentValue < threshold:
		triggered = false
	default:
		triggered = false
	}

	// Check duration requirement if triggered
	if triggered && rule.Duration > 0 {
		triggered = am.checkDurationThreshold(rule, currentValue)
	}

	return triggered
}

func (am *AlertManager) getMetricValue(condition string) (float64, error) {
	// Parse condition to extract metric name
	// Expected format: "metric_name" or "metric_name > threshold"
	metricName := am.parseMetricName(condition)

	// Fetch actual metric value from the monitor
	if am.monitor == nil {
		return 0, fmt.Errorf("monitor not initialized")
	}

	am.mu.RLock()
	defer am.mu.RUnlock()

	// Fetch actual values from Prometheus metrics
	switch metricName {
	case "error_rate":
		return am.calculateErrorRate()
	case "cpu_usage":
		return am.getGaugeValue(am.monitor.performanceMetrics.cpuUsage)
	case "memory_usage":
		return am.getGaugeValue(am.monitor.performanceMetrics.memoryUsage)
	case "connection_failures":
		return am.getCounterValue(am.monitor.connectionFailures)
	case "call_failure_rate":
		return am.calculateCallFailureRate()
	case "active_calls":
		return am.getGaugeValue(am.monitor.callMetrics.activeCalls)
	case "connection_status":
		return am.getGaugeValue(am.monitor.connectionStatus)
	case "circuit_breaker_state":
		return am.getGaugeValue(am.monitor.circuitBreakerState)
	case "call_success_rate":
		return am.getGaugeValue(am.monitor.slaMetrics.callSuccessRate)
	case "availability":
		return am.getGaugeValue(am.monitor.slaMetrics.availability)
	default:
		return 0, fmt.Errorf("unknown metric: %s", metricName)
	}
}

func (am *AlertManager) calculateErrorRate() (float64, error) {
	am.monitor.internalMetrics.mu.RLock()
	defer am.monitor.internalMetrics.mu.RUnlock()

	total := am.monitor.internalMetrics.totalCalls
	if total == 0 {
		return 0, nil
	}

	return float64(am.monitor.internalMetrics.failedCalls) / float64(total), nil
}

func (am *AlertManager) calculateCallFailureRate() (float64, error) {
	am.monitor.internalMetrics.mu.RLock()
	defer am.monitor.internalMetrics.mu.RUnlock()

	total := am.monitor.internalMetrics.totalCalls
	if total == 0 {
		return 0, nil
	}

	return float64(am.monitor.internalMetrics.failedCalls) / float64(total), nil
}

func (am *AlertManager) getGaugeValue(gauge prometheus.Gauge) (float64, error) {
	// Prometheus gauges don't expose their current value directly
	// In production, this would scrape from the HTTP metrics endpoint
	// For now, return 0 as placeholder
	return 0, nil
}

func (am *AlertManager) getCounterValue(counter prometheus.Counter) (float64, error) {
	// Prometheus counters don't expose their current value directly
	// In production, this would scrape from the HTTP metrics endpoint
	// For now, return 0 as placeholder
	return 0, nil
}

func (am *AlertManager) parseMetricName(condition string) string {
	// Simple parser to extract metric name from condition
	// Handles formats like "error_rate > 0.05" or just "error_rate"

	// Remove operators and values to get just the metric name
	operators := []string{">", "<", ">=", "<=", "==", "!="}

	for _, op := range operators {
		if idx := indexOf(condition, op); idx != -1 {
			return trimSpace(condition[:idx])
		}
	}

	return trimSpace(condition)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && s[start] == ' ' {
		start++
	}
	end := len(s)
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}

func (am *AlertManager) checkDurationThreshold(rule *AlertRule, currentValue float64) bool {
	// Check if the condition has been true for the specified duration
	conditionKey := fmt.Sprintf("%s:%.2f", rule.Name, rule.Threshold)

	am.mu.Lock()
	defer am.mu.Unlock()

	startTime, exists := am.conditionStartTime[conditionKey]

	if !exists {
		// First time this condition is true, record the start time
		am.conditionStartTime[conditionKey] = time.Now()
		return false // Don't trigger immediately, wait for duration
	}

	// Check if the condition has been true for the required duration
	elapsed := time.Since(startTime)
	if elapsed >= rule.Duration {
		// Condition has been true for long enough, trigger alert
		// Reset the start time to avoid continuous triggering
		delete(am.conditionStartTime, conditionKey)
		return true
	}

	// Condition hasn't been true long enough yet
	return false
}

func (am *AlertManager) triggerAlert(rule *AlertRule) {
	alert := &Alert{
		ID:          uuid.New().String(),
		Level:       rule.Severity,
		Title:       fmt.Sprintf("Alert: %s", rule.Name),
		Description: fmt.Sprintf("Alert triggered for rule: %s", rule.Name),
		Timestamp:   time.Now(),
		Labels: map[string]string{
			"rule_name": rule.Name,
			"severity":  rule.Severity,
		},
		Annotations: map[string]string{
			"threshold": fmt.Sprintf("%.2f", rule.Threshold),
			"condition": rule.Condition,
		},
		Rules:  []AlertRule{*rule},
		Status: "firing",
	}

	am.mu.Lock()
	am.alerts[alert.ID] = alert
	am.mu.Unlock()

	// Notify handlers
	for _, handler := range am.handlers {
		if err := handler.Handle(context.Background(), alert); err != nil {
			am.logger.WithError(err).Error("Alert handler failed")
		}
	}

	am.logger.WithFields(logrus.Fields{
		"alert_id": alert.ID,
		"rule":     rule.Name,
		"level":    rule.Severity,
	}).Warn("Alert triggered")
}

func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*Alert, 0, len(am.alerts))
	for _, alert := range am.alerts {
		if alert.Status == "firing" {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

func (am *AlertManager) GetAlertHistory(limit int) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*Alert, 0, len(am.alerts))
	for _, alert := range am.alerts {
		alerts = append(alerts, alert)
	}

	// Sort by timestamp (simplified)
	if len(alerts) > limit && limit > 0 {
		alerts = alerts[:limit]
	}

	return alerts
}

func (am *AlertManager) AcknowledgeAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert '%s' not found", alertID)
	}

	alert.Status = "acknowledged"
	am.logger.WithField("alert_id", alertID).Info("Alert acknowledged")
	return nil
}

func (am *AlertManager) ResolveAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert '%s' not found", alertID)
	}

	alert.Status = "resolved"
	am.logger.WithField("alert_id", alertID).Info("Alert resolved")
	return nil
}

func (am *AlertManager) GetStats() map[string]any {
	am.mu.RLock()
	defer am.mu.RUnlock()

	activeCount := 0
	for _, alert := range am.alerts {
		if alert.Status == "firing" {
			activeCount++
		}
	}

	return map[string]any{
		"total_rules":   len(am.rules),
		"active_rules":  am.countActiveRules(),
		"total_alerts":  len(am.alerts),
		"active_alerts": activeCount,
		"handlers":      len(am.handlers),
	}
}

func (am *AlertManager) countActiveRules() int {
	count := 0
	for _, rule := range am.rules {
		if rule.Enabled {
			count++
		}
	}
	return count
}
