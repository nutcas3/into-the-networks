package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func NewAlertManager() *AlertManager {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &AlertManager{
		rules:    make(map[string]*AlertRule),
		alerts:   make(map[string]*Alert),
		handlers: []AlertHandler{},
		logger:   logger,
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
	// Simplified alert evaluation
	// In production, this would be more sophisticated
	return true // Placeholder
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
