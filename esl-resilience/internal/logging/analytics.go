package logging

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// AnalyticsEngine provides log analytics capabilities
type AnalyticsEngine struct {
	aggregator *LogAggregator
}

func NewAnalyticsEngine(aggregator *LogAggregator) *AnalyticsEngine {
	return &AnalyticsEngine{
		aggregator: aggregator,
	}
}

// GetAnalytics returns comprehensive analytics for the given filter
func (ae *AnalyticsEngine) GetAnalytics(filter LogFilter) *LogAnalytics {
	entries := ae.aggregator.Query(filter)

	analytics := &LogAnalytics{
		TotalLogs:    int64(len(entries)),
		LogsByLevel:  make(map[string]int64),
		LogsBySource: make(map[string]int64),
		LogsByTenant: make(map[string]int64),
		TopErrors:    []LogEntry{},
	}

	var errorCount int64

	for _, entry := range entries {
		// Count by level
		analytics.LogsByLevel[entry.Level]++

		// Count by source
		analytics.LogsBySource[entry.Source]++

		// Count by tenant
		analytics.LogsByTenant[entry.TenantID.String()]++

		// Count errors
		if entry.Level == "error" {
			errorCount++
			analytics.TopErrors = append(analytics.TopErrors, *entry)
		}
	}

	if analytics.TotalLogs > 0 {
		analytics.ErrorRate = float64(errorCount) / float64(analytics.TotalLogs) * 100
	}

	// Sort top errors (simplified - just take first 10)
	if len(analytics.TopErrors) > 10 {
		analytics.TopErrors = analytics.TopErrors[:10]
	}

	return analytics
}

// GetTrendAnalysis provides trend analysis over time periods
func (ae *AnalyticsEngine) GetTrendAnalysis(timeRange time.Duration) *TrendAnalysis {
	now := time.Now()
	startTime := now.Add(-timeRange)

	filter := LogFilter{
		StartTime: &startTime,
		EndTime:   &now,
	}

	entries := ae.aggregator.Query(filter)

	analysis := &TrendAnalysis{
		TimeRange:      timeRange,
		TotalEntries:   int64(len(entries)),
		EntriesByHour:  make(map[string]int64),
		ErrorTrend:     make(map[string]int64),
		TenantActivity: make(map[string]int64),
	}

	// Group entries by hour
	for _, entry := range entries {
		hourKey := entry.Timestamp.Format("2006-01-02T15:00")
		analysis.EntriesByHour[hourKey]++

		if entry.Level == "error" {
			analysis.ErrorTrend[hourKey]++
		}

		tenantKey := entry.TenantID.String()
		analysis.TenantActivity[tenantKey]++
	}

	return analysis
}

// GetErrorPatterns analyzes common error patterns
func (ae *AnalyticsEngine) GetErrorPatterns(limit int) *ErrorPatterns {
	filter := LogFilter{
		Level: "error",
	}

	entries := ae.aggregator.Query(filter)

	patterns := &ErrorPatterns{
		CommonMessages:   make(map[string]int64),
		ErrorSources:     make(map[string]int64),
		TenantErrors:     make(map[string]int64),
		TimeDistribution: make(map[string]int64),
	}

	for _, entry := range entries {
		// Analyze message patterns (simplified)
		message := strings.ToLower(entry.Message)
		if strings.Contains(message, "connection") {
			patterns.CommonMessages["connection_errors"]++
		} else if strings.Contains(message, "timeout") {
			patterns.CommonMessages["timeout_errors"]++
		} else if strings.Contains(message, "failed") {
			patterns.CommonMessages["failure_errors"]++
		} else {
			patterns.CommonMessages["other_errors"]++
		}

		// Count by source
		patterns.ErrorSources[entry.Source]++

		// Count by tenant
		patterns.TenantErrors[entry.TenantID.String()]++

		// Count by hour of day
		hourKey := entry.Timestamp.Format("15:00")
		patterns.TimeDistribution[hourKey]++
	}

	return patterns
}

// GetTenantSummary provides per-tenant log summaries
func (ae *AnalyticsEngine) GetTenantSummary(tenantID uuid.UUID) *TenantSummary {
	filter := LogFilter{
		TenantID: &tenantID,
	}

	entries := ae.aggregator.Query(filter)

	summary := &TenantSummary{
		TenantID:     tenantID,
		TotalLogs:    int64(len(entries)),
		LogsByLevel:  make(map[string]int64),
		LogsBySource: make(map[string]int64),
		ErrorCount:   0,
		LastActivity: time.Time{},
	}

	for _, entry := range entries {
		summary.LogsByLevel[entry.Level]++
		summary.LogsBySource[entry.Source]++

		if entry.Level == "error" {
			summary.ErrorCount++
		}

		if entry.Timestamp.After(summary.LastActivity) {
			summary.LastActivity = entry.Timestamp
		}
	}

	if summary.TotalLogs > 0 {
		summary.ErrorRate = float64(summary.ErrorCount) / float64(summary.TotalLogs) * 100
	}

	return summary
}

// Supporting types for analytics
type TrendAnalysis struct {
	TimeRange      time.Duration    `json:"time_range"`
	TotalEntries   int64            `json:"total_entries"`
	EntriesByHour  map[string]int64 `json:"entries_by_hour"`
	ErrorTrend     map[string]int64 `json:"error_trend"`
	TenantActivity map[string]int64 `json:"tenant_activity"`
}

type ErrorPatterns struct {
	CommonMessages   map[string]int64 `json:"common_messages"`
	ErrorSources     map[string]int64 `json:"error_sources"`
	TenantErrors     map[string]int64 `json:"tenant_errors"`
	TimeDistribution map[string]int64 `json:"time_distribution"`
}

type TenantSummary struct {
	TenantID     uuid.UUID        `json:"tenant_id"`
	TotalLogs    int64            `json:"total_logs"`
	LogsByLevel  map[string]int64 `json:"logs_by_level"`
	LogsBySource map[string]int64 `json:"logs_by_source"`
	ErrorCount   int64            `json:"error_count"`
	ErrorRate    float64          `json:"error_rate"`
	LastActivity time.Time        `json:"last_activity"`
}

// PerformanceMetrics provides log processing performance metrics
type PerformanceMetrics struct {
	EntriesProcessed  int64         `json:"entries_processed"`
	ProcessingRate    float64       `json:"processing_rate"` // entries per second
	AverageLatency    time.Duration `json:"average_latency"`
	BufferUtilization float64       `json:"buffer_utilization"`
	MemoryUsageMB     float64       `json:"memory_usage_mb"`
	IndexSize         int64         `json:"index_size"`
}

func (ae *AnalyticsEngine) GetPerformanceMetrics() *PerformanceMetrics {
	stats := ae.aggregator.GetStats()

	metrics := &PerformanceMetrics{
		EntriesProcessed:  int64(stats["total_entries"].(int)),
		BufferUtilization: stats["buffer_utilization"].(float64),
		MemoryUsageMB:     stats["memory_usage_mb"].(float64),
		IndexSize:         int64(stats["index_size"].(int)),
	}

	// Calculate processing rate (simplified)
	if len(stats) > 0 {
		metrics.ProcessingRate = 100.0                 // Placeholder - would calculate from actual metrics
		metrics.AverageLatency = 10 * time.Millisecond // Placeholder
	}

	return metrics
}

// HealthStatus provides log system health status
type HealthStatus struct {
	Status          string         `json:"status"` // healthy, degraded, unhealthy
	Timestamp       time.Time      `json:"timestamp"`
	TotalEntries    int64          `json:"total_entries"`
	ErrorRate       float64        `json:"error_rate"`
	BufferStatus    map[string]any `json:"buffer_status"`
	ProcessorStatus map[string]any `json:"processor_status"`
	Recommendations []string       `json:"recommendations"`
}

func (ae *AnalyticsEngine) GetHealthStatus() *HealthStatus {
	stats := ae.aggregator.GetStats()

	status := &HealthStatus{
		Timestamp:       time.Now(),
		BufferStatus:    make(map[string]any),
		ProcessorStatus: make(map[string]any),
		Recommendations: []string{},
	}

	// Determine overall health
	totalEntries := int64(stats["total_entries"].(int))
	status.TotalEntries = totalEntries

	// Calculate error rate
	errorFilter := LogFilter{Level: "error"}
	errorEntries := ae.aggregator.Query(errorFilter)
	if totalEntries > 0 {
		status.ErrorRate = float64(len(errorEntries)) / float64(totalEntries) * 100
	}

	// Buffer health
	bufferUtilization := stats["buffer_utilization"].(float64)
	status.BufferStatus["utilization"] = bufferUtilization
	status.BufferStatus["capacity"] = stats["buffer_size"]

	// Processor health
	status.ProcessorStatus["active_processors"] = len(ae.aggregator.processors)

	// Determine status and recommendations
	if bufferUtilization > 0.9 {
		status.Status = "unhealthy"
		status.Recommendations = append(status.Recommendations, "Buffer utilization is critically high")
	} else if bufferUtilization > 0.7 {
		status.Status = "degraded"
		status.Recommendations = append(status.Recommendations, "Buffer utilization is elevated")
	} else if status.ErrorRate > 10.0 {
		status.Status = "degraded"
		status.Recommendations = append(status.Recommendations, "High error rate detected")
	} else {
		status.Status = "healthy"
	}

	return status
}
