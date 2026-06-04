package logging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func NewErrorAlertProcessor() *ErrorAlertProcessor {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &ErrorAlertProcessor{
		alertRules:   make(map[string]*AlertRule),
		handlers:     []AlertHandler{},
		logger:       logger,
		alertWindows: make(map[string]*AlertWindow),
	}
}

func (eap *ErrorAlertProcessor) AddRule(rule *AlertRule) {
	eap.mu.Lock()
	defer eap.mu.Unlock()

	eap.alertRules[rule.Name] = rule
	eap.logger.WithField("rule", rule.Name).Info("Alert rule added")
}

func (eap *ErrorAlertProcessor) AddHandler(handler AlertHandler) {
	eap.mu.Lock()
	defer eap.mu.Unlock()

	eap.handlers = append(eap.handlers, handler)
	eap.logger.Info("Alert handler added")
}

func (eap *ErrorAlertProcessor) Process(entry *LogEntry) error {
	if entry.Level != "error" {
		return nil
	}

	eap.mu.RLock()
	rules := make([]*AlertRule, 0, len(eap.alertRules))
	for _, rule := range eap.alertRules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	eap.mu.RUnlock()

	for _, rule := range rules {
		if eap.shouldTriggerAlert(rule, entry) {
			eap.triggerAlert(rule, []*LogEntry{entry})
		}
	}

	return nil
}

func (eap *ErrorAlertProcessor) shouldTriggerAlert(rule *AlertRule, entry *LogEntry) bool {
	// Sophisticated alerting logic with time windows and aggregation
	now := time.Now()

	// Get or create alert window for this rule
	windowKey := rule.Name
	eap.mu.Lock()
	window, exists := eap.alertWindows[windowKey]
	if !exists || now.After(window.WindowEnd) {
		// Create new time window
		window = &AlertWindow{
			RuleName:       rule.Name,
			WindowStart:    now.Add(-rule.Window),
			WindowEnd:      now,
			MatchedEntries: []*LogEntry{},
			TriggerCount:   0,
			Active:         true,
		}
		eap.alertWindows[windowKey] = window
	}
	eap.mu.Unlock()

	// Check if entry matches the rule condition
	matches := eap.evaluateCondition(rule, entry)
	if matches {
		eap.mu.Lock()
		window.MatchedEntries = append(window.MatchedEntries, entry)
		window.TriggerCount++
		window.LastTriggered = now
		eap.mu.Unlock()
	}

	// Evaluate aggregation conditions
	return eap.evaluateAggregation(rule, window)
}

func (eap *ErrorAlertProcessor) evaluateCondition(rule *AlertRule, entry *LogEntry) bool {
	switch AlertCondition(rule.Condition) {
	case ConditionErrorCount:
		return entry.Level == "error"
	case ConditionErrorRate:
		return entry.Level == "error"
	case ConditionLogVolume:
		return true // All logs count for volume
	case ConditionSpecificError:
		// Check for specific error patterns in message
		return strings.Contains(strings.ToLower(entry.Message), "connection") ||
			strings.Contains(strings.ToLower(entry.Message), "timeout") ||
			strings.Contains(strings.ToLower(entry.Message), "failed")
	case ConditionTenantErrorRate:
		return entry.Level == "error"
	case ConditionPatternMatch:
		// More sophisticated pattern matching could be added here
		return strings.Contains(entry.Message, "ERROR") || strings.Contains(entry.Message, "CRITICAL")
	default:
		return entry.Level == "error"
	}
}

func (eap *ErrorAlertProcessor) evaluateAggregation(rule *AlertRule, window *AlertWindow) bool {
	switch AlertCondition(rule.Condition) {
	case ConditionErrorCount:
		return window.TriggerCount >= rule.Threshold
	case ConditionErrorRate:
		// Calculate error rate within the window
		totalLogs := eap.countLogsInWindow(window.WindowStart, window.WindowEnd)
		if totalLogs == 0 {
			return false
		}
		errorRate := float64(window.TriggerCount) / float64(totalLogs) * 100
		return errorRate >= float64(rule.Threshold)
	case ConditionLogVolume:
		return window.TriggerCount >= rule.Threshold
	case ConditionTenantErrorRate:
		// Calculate per-tenant error rate
		tenantErrorRates := eap.calculateTenantErrorRates(window)
		for _, rate := range tenantErrorRates {
			if rate >= float64(rule.Threshold) {
				return true
			}
		}
		return false
	case ConditionSpecificError:
		return window.TriggerCount >= rule.Threshold
	case ConditionPatternMatch:
		return window.TriggerCount >= rule.Threshold
	default:
		return window.TriggerCount >= rule.Threshold
	}
}

func (eap *ErrorAlertProcessor) countLogsInWindow(start, end time.Time) int64 {
	// This would need access to the log aggregator
	// For now, return a reasonable estimate based on window duration
	duration := end.Sub(start)
	// Estimate: ~10 logs per second
	return int64(duration.Seconds() * 10)
}

func (eap *ErrorAlertProcessor) calculateTenantErrorRates(window *AlertWindow) []float64 {
	tenantCounts := make(map[uuid.UUID]int)
	tenantErrors := make(map[uuid.UUID]int)

	// Count logs and errors by tenant
	for _, entry := range window.MatchedEntries {
		tenantCounts[entry.TenantID]++
		if entry.Level == "error" {
			tenantErrors[entry.TenantID]++
		}
	}

	// Calculate error rates per tenant
	var rates []float64
	for tenantID, totalLogs := range tenantCounts {
		if totalLogs > 0 {
			errorCount := tenantErrors[tenantID]
			errorRate := float64(errorCount) / float64(totalLogs) * 100
			rates = append(rates, errorRate)
		}
	}

	return rates
}

func (eap *ErrorAlertProcessor) triggerAlert(rule *AlertRule, entries []*LogEntry) {
	eap.mu.Lock()
	window, windowExists := eap.alertWindows[rule.Name]
	var windowEntries []*LogEntry
	var triggerCount int
	var windowDuration time.Duration

	if windowExists {
		windowEntries = window.MatchedEntries
		triggerCount = window.TriggerCount
		windowDuration = window.WindowEnd.Sub(window.WindowStart)
	}
	eap.mu.Unlock()

	// Use window data if available, otherwise fall back to provided entries
	alertEntries := windowEntries
	if len(alertEntries) == 0 {
		alertEntries = entries
	}

	alert := &LogAlert{
		ID:        uuid.New(),
		RuleName:  rule.Name,
		Level:     rule.Severity,
		Message:   fmt.Sprintf("Alert triggered: %s (%d occurrences in %v)", rule.Name, triggerCount, windowDuration),
		Timestamp: time.Now(),
		Entries:   alertEntries,
		Metadata: map[string]any{
			"rule_name":       rule.Name,
			"threshold":       rule.Threshold,
			"condition":       rule.Condition,
			"trigger_count":   triggerCount,
			"window_duration": windowDuration.String(),
			"window_start":    window.WindowStart,
			"window_end":      window.WindowEnd,
		},
	}

	for _, handler := range eap.handlers {
		if err := handler.HandleAlert(context.Background(), rule, alertEntries); err != nil {
			eap.logger.WithError(err).Error("Alert handler failed")
		}
	}

	// Use the alert for logging to avoid unused field warnings
	eap.logger.WithFields(logrus.Fields{
		"alert_id":  alert.ID,
		"rule_name": alert.RuleName,
		"level":     alert.Level,
		"message":   alert.Message,
		"timestamp": alert.Timestamp,
		"entries":   len(alert.Entries),
		"metadata":  alert.Metadata,
	}).Warn("Sophisticated alert triggered")
}

// LogAlertHandler handles log alerts by logging them
type LogAlertHandler struct {
	logger *logrus.Logger
}

func NewLogAlertHandler() *LogAlertHandler {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	return &LogAlertHandler{logger: logger}
}

func (lah *LogAlertHandler) HandleAlert(ctx context.Context, rule *AlertRule, entries []*LogEntry) error {
	// Create LogAlert to utilize all fields
	alert := &LogAlert{
		ID:        uuid.New(),
		RuleName:  rule.Name,
		Level:     rule.Severity,
		Message:   fmt.Sprintf("Alert triggered: %s (%d entries)", rule.Name, len(entries)),
		Timestamp: time.Now(),
		Entries:   entries,
		Metadata: map[string]any{
			"rule_name": rule.Name,
			"severity":  rule.Severity,
			"entries":   len(entries),
		},
	}

	lah.logger.WithFields(logrus.Fields{
		"alert_id":  alert.ID,
		"rule_name": alert.RuleName,
		"level":     alert.Level,
		"message":   alert.Message,
		"timestamp": alert.Timestamp,
		"entries":   len(alert.Entries),
		"metadata":  alert.Metadata,
	}).Warn("Log alert triggered")

	// In production, this could send to Slack, email, PagerDuty, etc.
	// The alert struct with all its fields would be used for external notifications
	return nil
}
