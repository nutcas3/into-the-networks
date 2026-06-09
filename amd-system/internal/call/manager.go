package call

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Result represents the classification result
type Result string

const (
	ResultHuman            Result = "human"
	ResultAnsweringMachine Result = "answering_machine"
	ResultBeep             Result = "beep"
	ResultFax              Result = "fax"
	ResultSilence          Result = "silence"
	ResultUnknown          Result = "unknown"
)

// CallSession represents an active call being analyzed
type CallSession struct {
	ID          string
	PhoneNumber string
	CampaignID  string
	StartedAt   time.Time
	AudioChunks int
	LastChunkAt time.Time
	Result      Result
	Confidence  float64
	Completed   bool
	CompletedAt time.Time
	RoutedTo    string
	RetryCount  int
}

// Manager handles call sessions and their lifecycle
type Manager struct {
	sessions map[string]*CallSession
	mu       sync.RWMutex
	logger   *logrus.Logger
}

// Config holds call manager configuration
type Config struct {
	Logger *logrus.Logger
}

// NewManager creates a new call manager
func NewManager(cfg Config) *Manager {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	return &Manager{
		sessions: make(map[string]*CallSession),
		logger:   cfg.Logger,
	}
}

// StartSession creates a new call session
func (m *Manager) StartSession(phoneNumber, campaignID string) *CallSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	b := make([]byte, 8)
	rand.Read(b)
	session := &CallSession{
		ID:          fmt.Sprintf("call-%s", hex.EncodeToString(b)),
		PhoneNumber: phoneNumber,
		CampaignID:  campaignID,
		StartedAt:   time.Now(),
		Result:      ResultUnknown,
	}

	m.sessions[session.ID] = session

	m.logger.WithFields(logrus.Fields{
		"session_id":  session.ID,
		"phone":       phoneNumber,
		"campaign_id": campaignID,
	}).Info("Call session started")

	return session
}

// RecordAudioChunk increments the audio chunk counter for a session
func (m *Manager) RecordAudioChunk(sessionID string) (*CallSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.AudioChunks++
	session.LastChunkAt = time.Now()

	return session, nil
}

// CompleteSession marks a session as complete with classification result
func (m *Manager) CompleteSession(sessionID string, result Result, confidence float64) (*CallSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.Result = result
	session.Confidence = confidence
	session.Completed = true
	session.CompletedAt = time.Now()

	m.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"result":     result,
		"confidence": confidence,
		"chunks":     session.AudioChunks,
	}).Info("Call session completed")

	return session, nil
}

// RouteSession marks a session as routed to a destination
func (m *Manager) RouteSession(sessionID, destination string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.RoutedTo = destination

	m.logger.WithFields(logrus.Fields{
		"session_id":  sessionID,
		"destination": destination,
	}).Info("Call session routed")

	return nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) (*CallSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	return session, exists
}

// GetAllSessions returns all sessions
func (m *Manager) GetAllSessions() []*CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*CallSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// GetActiveSessions returns non-completed sessions
func (m *Manager) GetActiveSessions() []*CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*CallSession, 0)
	for _, s := range m.sessions {
		if !s.Completed {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// GetStats returns call statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.sessions)
	completed := 0
	active := 0
	results := make(map[Result]int)

	for _, s := range m.sessions {
		if s.Completed {
			completed++
			results[s.Result]++
		} else {
			active++
		}
	}

	return map[string]interface{}{
		"total_sessions":     total,
		"active_sessions":    active,
		"completed_sessions": completed,
		"results":            results,
		"timestamp":          time.Now().Unix(),
	}
}

// CleanupOldSessions removes sessions older than maxAge
func (m *Manager) CleanupOldSessions(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, session := range m.sessions {
		if session.Completed && session.CompletedAt.Before(cutoff) {
			delete(m.sessions, id)
			removed++
		}
	}

	return removed
}
