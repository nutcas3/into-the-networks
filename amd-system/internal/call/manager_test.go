package call

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	require.NotNil(t, mgr)
	assert.Equal(t, 0, len(mgr.GetAllSessions()))
}

func TestStartSession(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	session := mgr.StartSession("+1-555-0123", "campaign-1")

	require.NotNil(t, session)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "+1-555-0123", session.PhoneNumber)
	assert.Equal(t, "campaign-1", session.CampaignID)
	assert.Equal(t, ResultUnknown, session.Result)
	assert.False(t, session.Completed)

	// Should be retrievable
	retrieved, exists := mgr.GetSession(session.ID)
	assert.True(t, exists)
	assert.Equal(t, session.ID, retrieved.ID)
}

func TestRecordAudioChunk(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	session := mgr.StartSession("+1-555-0123", "campaign-1")

	updated, err := mgr.RecordAudioChunk(session.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, updated.AudioChunks)

	updated, _ = mgr.RecordAudioChunk(session.ID)
	assert.Equal(t, 2, updated.AudioChunks)
}

func TestRecordAudioChunkNotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	_, err := mgr.RecordAudioChunk("nonexistent")
	assert.Error(t, err)
}

func TestCompleteSession(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	session := mgr.StartSession("+1-555-0123", "campaign-1")

	completed, err := mgr.CompleteSession(session.ID, ResultHuman, 0.95)
	require.NoError(t, err)
	assert.True(t, completed.Completed)
	assert.Equal(t, ResultHuman, completed.Result)
	assert.Equal(t, 0.95, completed.Confidence)
	assert.False(t, completed.CompletedAt.IsZero())
}

func TestCompleteSessionNotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	_, err := mgr.CompleteSession("nonexistent", ResultHuman, 0.5)
	assert.Error(t, err)
}

func TestRouteSession(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	session := mgr.StartSession("+1-555-0123", "campaign-1")

	err := mgr.RouteSession(session.ID, "agent_queue")
	require.NoError(t, err)

	retrieved, _ := mgr.GetSession(session.ID)
	assert.Equal(t, "agent_queue", retrieved.RoutedTo)
}

func TestRouteSessionNotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	err := mgr.RouteSession("nonexistent", "queue")
	assert.Error(t, err)
}

func TestGetActiveSessions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})

	s1 := mgr.StartSession("+1-555-0001", "campaign-1")
	mgr.CompleteSession(s1.ID, ResultHuman, 0.9)

	s2 := mgr.StartSession("+1-555-0002", "campaign-1")

	active := mgr.GetActiveSessions()
	assert.Len(t, active, 1)
	assert.Equal(t, s2.ID, active[0].ID)
}

func TestGetStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})

	mgr.StartSession("+1-555-0001", "campaign-1")
	s2 := mgr.StartSession("+1-555-0002", "campaign-1")
	mgr.CompleteSession(s2.ID, ResultHuman, 0.9)

	stats := mgr.GetStats()
	assert.Equal(t, 2, stats["total_sessions"])
	assert.Equal(t, 1, stats["active_sessions"])
	assert.Equal(t, 1, stats["completed_sessions"])
}

func TestCleanupOldSessions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(Config{Logger: logger})
	s1 := mgr.StartSession("+1-555-0001", "campaign-1")
	mgr.CompleteSession(s1.ID, ResultHuman, 0.9)

	// Manually set completion time to be very old
	s1.CompletedAt = time.Now().Add(-48 * time.Hour)

	s2 := mgr.StartSession("+1-555-0002", "campaign-1")

	removed := mgr.CleanupOldSessions(24 * time.Hour)
	assert.Equal(t, 1, removed)

	// s1 should be gone, s2 should remain
	_, exists := mgr.GetSession(s1.ID)
	assert.False(t, exists)
	_, exists = mgr.GetSession(s2.ID)
	assert.True(t, exists)
}
