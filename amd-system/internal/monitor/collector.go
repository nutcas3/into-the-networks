package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/nutcas3/amd-system/internal/call"
	"github.com/nutcas3/amd-system/internal/classifier"
	"github.com/sirupsen/logrus"
)

// Collector gathers and reports AMD system performance metrics
type Collector struct {
	callMgr    *call.Manager
	classifier *classifier.Client
	logger     *logrus.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	interval   time.Duration
	mu         sync.RWMutex
	metrics    *Metrics
}

// Metrics holds collected AMD system metrics
type Metrics struct {
	Timestamp         int64           `json:"timestamp"`
	TotalCalls        int             `json:"total_calls"`
	ActiveCalls       int             `json:"active_calls"`
	CompletedCalls    int             `json:"completed_calls"`
	HumanDetected     int             `json:"human_detected"`
	AMDetected        int             `json:"am_detected"`
	BeepDetected      int             `json:"beep_detected"`
	FaxDetected       int             `json:"fax_detected"`
	SilenceDetected   int             `json:"silence_detected"`
	UnknownDetected   int             `json:"unknown_detected"`
	AvgConfidence     float64         `json:"avg_confidence"`
	ClassifierHealthy bool            `json:"classifier_healthy"`
	ClassifierLatency float64         `json:"classifier_latency_ms"`
	RoutingBreakdown  map[string]int  `json:"routing_breakdown"`
	ErrorRate         float64         `json:"error_rate"`
	Throughput        float64         `json:"throughput_per_min"`
	SessionHistory    []SessionMetric `json:"recent_sessions,omitempty"`
}

// SessionMetric holds per-session metrics for recent history
type SessionMetric struct {
	SessionID  string      `json:"session_id"`
	Result     call.Result `json:"result"`
	Confidence float64     `json:"confidence"`
	RoutedTo   string      `json:"routed_to"`
	Duration   float64     `json:"duration_ms"`
	Timestamp  int64       `json:"timestamp"`
}

// Config holds collector configuration
type Config struct {
	CallMgr    *call.Manager
	Classifier *classifier.Client
	Interval   time.Duration
	Logger     *logrus.Logger
}

// NewCollector creates a new metrics collector
func NewCollector(cfg Config) *Collector {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 15 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Collector{
		callMgr:    cfg.CallMgr,
		classifier: cfg.Classifier,
		logger:     cfg.Logger,
		ctx:        ctx,
		cancel:     cancel,
		interval:   cfg.Interval,
		metrics: &Metrics{
			RoutingBreakdown: make(map[string]int),
		},
	}

	go c.run()

	return c
}

// Close stops the collector
func (c *Collector) Close() {
	c.cancel()
}

// GetMetrics returns current metrics
func (c *Collector) GetMetrics() *Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := *c.metrics
	// Copy maps/slices
	m.RoutingBreakdown = make(map[string]int)
	for k, v := range c.metrics.RoutingBreakdown {
		m.RoutingBreakdown[k] = v
	}
	if len(c.metrics.SessionHistory) > 0 {
		m.SessionHistory = make([]SessionMetric, len(c.metrics.SessionHistory))
		copy(m.SessionHistory, c.metrics.SessionHistory)
	}
	return &m
}

func (c *Collector) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Collector) collect() {
	metrics := &Metrics{
		Timestamp:        time.Now().Unix(),
		RoutingBreakdown: make(map[string]int),
	}

	// Call statistics
	if c.callMgr != nil {
		stats := c.callMgr.GetStats()
		metrics.TotalCalls = stats["total_sessions"].(int)
		metrics.ActiveCalls = stats["active_sessions"].(int)
		metrics.CompletedCalls = stats["completed_sessions"].(int)

		results, _ := stats["results"].(map[call.Result]int)
		metrics.HumanDetected = results[call.ResultHuman]
		metrics.AMDetected = results[call.ResultAnsweringMachine]
		metrics.BeepDetected = results[call.ResultBeep]
		metrics.FaxDetected = results[call.ResultFax]
		metrics.SilenceDetected = results[call.ResultSilence]
		metrics.UnknownDetected = results[call.ResultUnknown]

		// Calculate average confidence from recent completed sessions
		sessions := c.callMgr.GetAllSessions()
		var totalConfidence float64
		var confidenceCount int
		recentSessions := make([]SessionMetric, 0, 20)
		cutoff := time.Now().Add(-5 * time.Minute)

		for _, s := range sessions {
			if s.Completed {
				totalConfidence += s.Confidence
				confidenceCount++
				if s.CompletedAt.After(cutoff) {
					recentSessions = append(recentSessions, SessionMetric{
						SessionID:  s.ID,
						Result:     s.Result,
						Confidence: s.Confidence,
						RoutedTo:   s.RoutedTo,
						Duration:   float64(s.CompletedAt.Sub(s.StartedAt).Milliseconds()),
						Timestamp:  s.CompletedAt.Unix(),
					})
				}
			}
			// Build routing breakdown
			if s.RoutedTo != "" {
				metrics.RoutingBreakdown[s.RoutedTo]++
			}
		}

		if confidenceCount > 0 {
			metrics.AvgConfidence = totalConfidence / float64(confidenceCount)
		}

		// Keep only last 20 sessions in history
		if len(recentSessions) > 20 {
			recentSessions = recentSessions[len(recentSessions)-20:]
		}
		metrics.SessionHistory = recentSessions

		// Calculate throughput (calls per minute from last 5 min)
		metrics.Throughput = float64(len(recentSessions)) / 5.0
	}

	// Classifier health
	if c.classifier != nil {
		start := time.Now()
		health, err := c.classifier.Health()
		if err != nil {
			metrics.ClassifierHealthy = false
			c.logger.WithError(err).Warn("Classifier health check failed")
		} else {
			metrics.ClassifierHealthy = true
			metrics.ClassifierLatency = float64(time.Since(start).Milliseconds())
			_ = health
		}
	}

	c.mu.Lock()
	c.metrics = metrics
	c.mu.Unlock()

	c.logger.WithFields(logrus.Fields{
		"total_calls":        metrics.TotalCalls,
		"active_calls":       metrics.ActiveCalls,
		"human":              metrics.HumanDetected,
		"am":                 metrics.AMDetected,
		"classifier_healthy": metrics.ClassifierHealthy,
	}).Debug("Metrics collected")
}

// CheckHealth performs a quick system health check
func (c *Collector) CheckHealth() map[string]interface{} {
	metrics := c.GetMetrics()

	status := "healthy"
	// Only mark degraded if classifier is explicitly configured and unhealthy
	if c.classifier != nil && !metrics.ClassifierHealthy {
		status = "degraded"
	}
	if metrics.TotalCalls > 0 && metrics.ActiveCalls > metrics.TotalCalls/2 {
		status = "busy"
	}

	return map[string]interface{}{
		"status":             status,
		"classifier_healthy": metrics.ClassifierHealthy,
		"total_calls":        metrics.TotalCalls,
		"active_calls":       metrics.ActiveCalls,
		"avg_confidence":     metrics.AvgConfidence,
		"timestamp":          metrics.Timestamp,
	}
}
