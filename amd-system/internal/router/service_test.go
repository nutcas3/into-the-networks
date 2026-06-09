package router

import (
	"testing"

	"github.com/nutcas3/amd-system/internal/call"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{
		CallMgr:      callMgr,
		DefaultRoute: "agents",
		AMRoute:      "voicemail",
		UnknownRoute: "retry",
		MaxRetries:   3,
		Logger:       logger,
	})

	require.NotNil(t, svc)
	assert.Equal(t, "agents", svc.defaultRoute)
	assert.Equal(t, "voicemail", svc.amRoute)
	assert.Equal(t, 3, svc.maxRetries)
}

func TestNewServiceDefaults(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr})

	assert.Equal(t, "agent_queue", svc.defaultRoute)
	assert.Equal(t, "voicemail_drop", svc.amRoute)
	assert.Equal(t, 3, svc.maxRetries)
}

func TestRouteHuman(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr, Logger: logger})

	session := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(session.ID, call.ResultHuman, 0.95)

	err := svc.Route(session.ID, call.ResultHuman, 0.95)
	require.NoError(t, err)

	retrieved, _ := callMgr.GetSession(session.ID)
	assert.Equal(t, "agent_queue", retrieved.RoutedTo)
}

func TestRouteAnsweringMachine(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr, Logger: logger})

	session := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(session.ID, call.ResultAnsweringMachine, 0.85)

	err := svc.Route(session.ID, call.ResultAnsweringMachine, 0.85)
	require.NoError(t, err)

	retrieved, _ := callMgr.GetSession(session.ID)
	assert.Equal(t, "voicemail_drop", retrieved.RoutedTo)
}

func TestRouteBeep(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr, Logger: logger})

	session := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(session.ID, call.ResultBeep, 0.9)

	err := svc.Route(session.ID, call.ResultBeep, 0.9)
	require.NoError(t, err)

	retrieved, _ := callMgr.GetSession(session.ID)
	assert.Equal(t, "voicemail_drop", retrieved.RoutedTo)
}

func TestRouteFax(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr, Logger: logger})

	session := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(session.ID, call.ResultFax, 0.88)

	err := svc.Route(session.ID, call.ResultFax, 0.88)
	require.NoError(t, err)

	retrieved, _ := callMgr.GetSession(session.ID)
	assert.Equal(t, "fax_handler", retrieved.RoutedTo)
}

func TestRouteUnknown(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr, Logger: logger})

	session := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(session.ID, call.ResultUnknown, 0.4)

	err := svc.Route(session.ID, call.ResultUnknown, 0.4)
	require.NoError(t, err)

	retrieved, _ := callMgr.GetSession(session.ID)
	assert.Equal(t, "retry_queue", retrieved.RoutedTo)
}

func TestRouteSilence(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr, Logger: logger})

	session := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(session.ID, call.ResultSilence, 0.3)

	err := svc.Route(session.ID, call.ResultSilence, 0.3)
	require.NoError(t, err)

	retrieved, _ := callMgr.GetSession(session.ID)
	assert.Equal(t, "retry_queue", retrieved.RoutedTo)
}

func TestShouldRetry(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr, MaxRetries: 3, Logger: logger})

	// Unknown with 0 retries - should retry
	s1 := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(s1.ID, call.ResultUnknown, 0.4)
	assert.True(t, svc.ShouldRetry(s1.ID))

	// Human - should not retry
	s2 := callMgr.StartSession("+1-555-0002", "c1")
	callMgr.CompleteSession(s2.ID, call.ResultHuman, 0.95)
	assert.False(t, svc.ShouldRetry(s2.ID))

	// Max retries reached
	s3 := callMgr.StartSession("+1-555-0003", "c1")
	callMgr.CompleteSession(s3.ID, call.ResultUnknown, 0.4)
	for i := 0; i < 3; i++ {
		svc.RecordRetry(s3.ID)
	}
	assert.False(t, svc.ShouldRetry(s3.ID))
}

func TestGetRoutingStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	callMgr := call.NewManager(call.Config{Logger: logger})
	svc := NewService(Config{CallMgr: callMgr, Logger: logger})

	s1 := callMgr.StartSession("+1-555-0001", "c1")
	callMgr.CompleteSession(s1.ID, call.ResultHuman, 0.95)
	callMgr.RouteSession(s1.ID, "agent_queue")

	s2 := callMgr.StartSession("+1-555-0002", "c1")
	callMgr.CompleteSession(s2.ID, call.ResultAnsweringMachine, 0.85)
	callMgr.RouteSession(s2.ID, "voicemail_drop")

	stats := svc.GetRoutingStats()
	assert.Equal(t, 2, stats["total_sessions"])
	assert.Equal(t, 1, stats["routed_to_agent"])
	assert.Equal(t, 1, stats["routed_to_voicemail"])
}
