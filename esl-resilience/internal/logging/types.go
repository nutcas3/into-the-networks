package logging

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type LogAggregator struct {
	entries    []*LogEntry
	index      map[string][]*LogEntry // tenant_id, level, etc.
	bufferSize int
	mu         sync.RWMutex
	logger     *logrus.Logger
	processors []LogProcessor
}

type LogEntry struct {
	ID        uuid.UUID      `json:"id"`
	TenantID  uuid.UUID      `json:"tenant_id"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Timestamp time.Time      `json:"timestamp"`
	Fields    map[string]any `json:"fields"`
	Source    string         `json:"source"` // esl, state, monitor, etc.
	Tags      []string       `json:"tags"`
}

type LogProcessor interface {
	Process(entry *LogEntry) error
}

type LogFilter struct {
	TenantID  *uuid.UUID `json:"tenant_id,omitempty"`
	Level     string     `json:"level,omitempty"`
	Source    string     `json:"source,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Message   string     `json:"message,omitempty"`
}

type LogAnalytics struct {
	TotalLogs       int64            `json:"total_logs"`
	LogsByLevel     map[string]int64 `json:"logs_by_level"`
	LogsBySource    map[string]int64 `json:"logs_by_source"`
	LogsByTenant    map[string]int64 `json:"logs_by_tenant"`
	ErrorRate       float64          `json:"error_rate"`
	AlertsGenerated int64            `json:"alerts_generated"`
	TopErrors       []LogEntry       `json:"top_errors"`
}

type AlertRule struct {
	Name      string        `json:"name"`
	Condition string        `json:"condition"`
	Threshold int           `json:"threshold"`
	Window    time.Duration `json:"window"`
	Severity  string        `json:"severity"`
	Enabled   bool          `json:"enabled"`
	Handler   AlertHandler  `json:"-"`
}

type AlertHandler interface {
	HandleAlert(ctx context.Context, rule *AlertRule, entries []*LogEntry) error
}

type LogAlert struct {
	ID        uuid.UUID      `json:"id"`
	RuleName  string         `json:"rule_name"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Timestamp time.Time      `json:"timestamp"`
	Entries   []*LogEntry    `json:"entries"`
	Metadata  map[string]any `json:"metadata"`
}

// Sophisticated alerting types
type AlertWindow struct {
	RuleName       string      `json:"rule_name"`
	WindowStart    time.Time   `json:"window_start"`
	WindowEnd      time.Time   `json:"window_end"`
	MatchedEntries []*LogEntry `json:"matched_entries"`
	TriggerCount   int         `json:"trigger_count"`
	LastTriggered  time.Time   `json:"last_triggered"`
	Active         bool        `json:"active"`
}

type AlertCondition string

const (
	ConditionErrorCount      AlertCondition = "error_count"
	ConditionErrorRate       AlertCondition = "error_rate"
	ConditionLogVolume       AlertCondition = "log_volume"
	ConditionSpecificError   AlertCondition = "specific_error"
	ConditionTenantErrorRate AlertCondition = "tenant_error_rate"
	ConditionPatternMatch    AlertCondition = "pattern_match"
)

type AlertAggregation struct {
	Type       string        `json:"type"`  // count, rate, sum, avg
	Field      string        `json:"field"` // field to aggregate
	TimeWindow time.Duration `json:"time_window"`
	GroupBy    []string      `json:"group_by"` // tenant_id, source, etc.
}

type ErrorAlertProcessor struct {
	alertRules   map[string]*AlertRule
	handlers     []AlertHandler
	mu           sync.RWMutex
	logger       *logrus.Logger
	alertWindows map[string]*AlertWindow // Time window tracking
}
