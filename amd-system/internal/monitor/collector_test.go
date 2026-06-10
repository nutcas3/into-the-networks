package monitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nutcas3/amd-system/internal/call"
	"github.com/nutcas3/amd-system/internal/classifier"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollector(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})

	c := NewCollector(Config{
		CallMgr:  callMgr,
		Logger:   logger,
		Interval: 100 * time.Millisecond,
	})
	require.NotNil(t, c)
	defer c.Close()

	// Wait for first collection
	time.Sleep(150 * time.Millisecond)
	metrics := c.GetMetrics()
	assert.NotNil(t, metrics)
	assert.NotZero(t, metrics.Timestamp)
}

func TestCollectorWithData(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})

	// Create some sessions with different results
	s1 := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(s1.ID, call.ResultHuman, 0.95)
	callMgr.RouteSession(s1.ID, "agent_queue")

	s2 := callMgr.StartSession("+1-555-0002", "c1")
	callMgr.CompleteSession(s2.ID, call.ResultAnsweringMachine, 0.88)
	callMgr.RouteSession(s2.ID, "voicemail_drop")

	s3 := callMgr.StartSession("+1-555-0003", "c1")
	callMgr.CompleteSession(s3.ID, call.ResultBeep, 0.92)
	callMgr.RouteSession(s3.ID, "voicemail_drop")

	c := NewCollector(Config{
		CallMgr:  callMgr,
		Logger:   logger,
		Interval: 1 * time.Hour, // Don't auto-collect, we'll call manually
	})
	defer c.Close()

	c.collect()

	metrics := c.GetMetrics()
	assert.Equal(t, 3, metrics.TotalCalls)
	assert.Equal(t, 3, metrics.CompletedCalls)
	assert.Equal(t, 0, metrics.ActiveCalls)
	assert.Equal(t, 1, metrics.HumanDetected)
	assert.Equal(t, 1, metrics.AMDetected)
	assert.Equal(t, 1, metrics.BeepDetected)
	assert.True(t, metrics.AvgConfidence > 0.9)
	assert.Equal(t, 2, metrics.RoutingBreakdown["voicemail_drop"])
	assert.Equal(t, 1, metrics.RoutingBreakdown["agent_queue"])
	assert.Len(t, metrics.SessionHistory, 3)
}

func TestCollectorClassifierHealth(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create mock ML service
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(classifier.HealthResponse{
				Status:  "healthy",
				Model:   "test",
				Version: "1.0.0",
			})
		}
	}))
	defer server.Close()

	callMgr := call.NewManager(call.Config{Logger: logger})
	cls := classifier.NewClient(classifier.Config{Endpoint: server.URL})

	c := NewCollector(Config{
		CallMgr:    callMgr,
		Classifier: cls,
		Logger:     logger,
		Interval:   1 * time.Hour,
	})
	defer c.Close()

	c.collect()

	metrics := c.GetMetrics()
	assert.True(t, metrics.ClassifierHealthy)
	assert.GreaterOrEqual(t, metrics.ClassifierLatency, float64(0))
}

func TestCollectorClassifierUnavailable(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	cls := classifier.NewClient(classifier.Config{Endpoint: "http://invalid-url-12345"})

	c := NewCollector(Config{
		CallMgr:    callMgr,
		Classifier: cls,
		Logger:     logger,
		Interval:   1 * time.Hour,
	})
	defer c.Close()

	c.collect()

	metrics := c.GetMetrics()
	assert.False(t, metrics.ClassifierHealthy)
}

func TestCheckHealth(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	c := NewCollector(Config{
		CallMgr:  callMgr,
		Logger:   logger,
		Interval: 1 * time.Hour,
	})
	defer c.Close()

	health := c.CheckHealth()
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, 0, health["total_calls"])
}

func TestCheckHealthDegraded(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	cls := classifier.NewClient(classifier.Config{Endpoint: "http://invalid-url-12345"})

	c := NewCollector(Config{
		CallMgr:    callMgr,
		Classifier: cls,
		Logger:     logger,
		Interval:   1 * time.Hour,
	})
	defer c.Close()

	health := c.CheckHealth()
	assert.Equal(t, "degraded", health["status"])
	assert.False(t, health["classifier_healthy"].(bool))
}

func TestThroughput(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})

	// Create multiple sessions within recent window
	for i := 0; i < 10; i++ {
		s := callMgr.StartSession("+1-555-0000", "c1")
		callMgr.CompleteSession(s.ID, call.ResultHuman, 0.9)
	}

	c := NewCollector(Config{
		CallMgr:  callMgr,
		Logger:   logger,
		Interval: 1 * time.Hour,
	})
	defer c.Close()

	c.collect()

	metrics := c.GetMetrics()
	assert.Equal(t, 10, metrics.TotalCalls)
	assert.Equal(t, 10, metrics.CompletedCalls)
	assert.Equal(t, 2.0, metrics.Throughput) // 10 calls / 5 min = 2 per min
}
