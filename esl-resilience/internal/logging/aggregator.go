package logging

import (
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func NewLogAggregator(bufferSize int) *LogAggregator {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &LogAggregator{
		entries:    make([]*LogEntry, 0, bufferSize),
		index:      make(map[string][]*LogEntry),
		bufferSize: bufferSize,
		logger:     logger,
		processors: []LogProcessor{},
	}
}

func (la *LogAggregator) AddProcessor(processor LogProcessor) {
	la.mu.Lock()
	defer la.mu.Unlock()

	la.processors = append(la.processors, processor)
	la.logger.Info("Log processor added")
}

func (la *LogAggregator) Log(tenantID uuid.UUID, level, message, source string, fields map[string]any, tags []string) {
	entry := &LogEntry{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
		Fields:    fields,
		Source:    source,
		Tags:      tags,
	}

	la.addEntry(entry)
}

func (la *LogAggregator) addEntry(entry *LogEntry) {
	la.mu.Lock()
	defer la.mu.Unlock()

	// Add to main buffer
	la.entries = append(la.entries, entry)

	// Maintain buffer size
	if len(la.entries) > la.bufferSize {
		// Remove oldest entry
		oldEntry := la.entries[0]
		la.entries = la.entries[1:]

		// Remove from index
		la.removeFromIndex(oldEntry)
	}

	// Add to index
	la.addToIndex(entry)

	// Process entry
	for _, processor := range la.processors {
		if err := processor.Process(entry); err != nil {
			la.logger.WithError(err).Error("Log processor failed")
		}
	}
}

func (la *LogAggregator) addToIndex(entry *LogEntry) {
	// Index by tenant
	tenantKey := entry.TenantID.String()
	la.index[tenantKey] = append(la.index[tenantKey], entry)

	// Index by level
	levelKey := "level:" + entry.Level
	la.index[levelKey] = append(la.index[levelKey], entry)

	// Index by source
	sourceKey := "source:" + entry.Source
	la.index[sourceKey] = append(la.index[sourceKey], entry)

	// Index by tags
	for _, tag := range entry.Tags {
		tagKey := "tag:" + tag
		la.index[tagKey] = append(la.index[tagKey], entry)
	}
}

func (la *LogAggregator) removeFromIndex(entry *LogEntry) {
	// Remove from tenant index
	tenantKey := entry.TenantID.String()
	if entries, exists := la.index[tenantKey]; exists {
		la.index[tenantKey] = la.removeEntryFromSlice(entries, entry)
	}

	// Remove from other indexes
	levelKey := "level:" + entry.Level
	if entries, exists := la.index[levelKey]; exists {
		la.index[levelKey] = la.removeEntryFromSlice(entries, entry)
	}

	sourceKey := "source:" + entry.Source
	if entries, exists := la.index[sourceKey]; exists {
		la.index[sourceKey] = la.removeEntryFromSlice(entries, entry)
	}

	for _, tag := range entry.Tags {
		tagKey := "tag:" + tag
		if entries, exists := la.index[tagKey]; exists {
			la.index[tagKey] = la.removeEntryFromSlice(entries, entry)
		}
	}
}

func (la *LogAggregator) removeEntryFromSlice(entries []*LogEntry, target *LogEntry) []*LogEntry {
	for i, entry := range entries {
		if entry.ID == target.ID {
			return append(entries[:i], entries[i+1:]...)
		}
	}
	return entries
}

func (la *LogAggregator) Query(filter LogFilter) []*LogEntry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	var results []*LogEntry

	// Start with all entries or filter by specific index
	var candidates []*LogEntry
	if filter.TenantID != nil {
		tenantKey := filter.TenantID.String()
		candidates = la.index[tenantKey]
	} else if filter.Level != "" {
		levelKey := "level:" + filter.Level
		candidates = la.index[levelKey]
	} else if filter.Source != "" {
		sourceKey := "source:" + filter.Source
		candidates = la.index[sourceKey]
	} else {
		candidates = la.entries
	}

	// Apply remaining filters
	for _, entry := range candidates {
		if la.matchesFilter(entry, filter) {
			results = append(results, entry)
		}
	}

	return results
}

func (la *LogAggregator) matchesFilter(entry *LogEntry, filter LogFilter) bool {
	// Time range filter
	if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
		return false
	}

	// Level filter
	if filter.Level != "" && entry.Level != filter.Level {
		return false
	}

	// Source filter
	if filter.Source != "" && entry.Source != filter.Source {
		return false
	}

	// Message filter
	if filter.Message != "" && !contains(entry.Message, filter.Message) {
		return false
	}

	// Tags filter
	if len(filter.Tags) > 0 {
		entryTags := make(map[string]bool)
		for _, tag := range entry.Tags {
			entryTags[tag] = true
		}
		for _, filterTag := range filter.Tags {
			if !entryTags[filterTag] {
				return false
			}
		}
	}

	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (la *LogAggregator) GetRecentLogs(count int) []*LogEntry {
	la.mu.RLock()
	defer la.mu.RUnlock()

	if count <= 0 || count >= len(la.entries) {
		// Return copy of all entries
		result := make([]*LogEntry, len(la.entries))
		copy(result, la.entries)
		return result
	}

	// Return most recent entries
	start := len(la.entries) - count
	result := make([]*LogEntry, count)
	copy(result, la.entries[start:])
	return result
}

func (la *LogAggregator) Clear() {
	la.mu.Lock()
	defer la.mu.Unlock()

	la.entries = la.entries[:0]
	la.index = make(map[string][]*LogEntry)
	la.logger.Info("Log aggregator cleared")
}

func (la *LogAggregator) GetStats() map[string]any {
	la.mu.RLock()
	defer la.mu.RUnlock()

	return map[string]any{
		"total_entries":      len(la.entries),
		"buffer_size":        la.bufferSize,
		"index_size":         len(la.index),
		"processors":         len(la.processors),
		"memory_usage_mb":    la.estimateMemoryUsage(),
		"buffer_utilization": float64(len(la.entries)) / float64(la.bufferSize),
	}
}

func (la *LogAggregator) estimateMemoryUsage() float64 {
	// Rough estimation
	entrySize := 200 // bytes per entry (rough estimate)
	return float64(len(la.entries)*entrySize) / 1024 / 1024
}
