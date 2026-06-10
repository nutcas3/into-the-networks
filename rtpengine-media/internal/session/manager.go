package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nutcas3/rtpengine-media/internal/ng"
	"github.com/sirupsen/logrus"
)

// Session represents an active media session managed by RTPengine
type Session struct {
	CallID     string            `json:"call_id"`
	FromTag    string            `json:"from_tag"`
	ToTag      string            `json:"to_tag,omitempty"`
	LocalSDP   string            `json:"local_sdp,omitempty"`
	RemoteSDP  string            `json:"remote_sdp,omitempty"`
	LocalAddr  string            `json:"local_addr,omitempty"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
	Status     string            `json:"status"`
	Codec      string            `json:"codec,omitempty"`
	Recordings []string          `json:"recordings,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	ExpiresAt  time.Time         `json:"expires_at"`
	Direction  string            `json:"direction,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	mu         sync.RWMutex
}

// IsActive returns true if the session is in an active state
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status == "active" || s.Status == "offered"
}

// UpdateToTag sets the to-tag for the session
func (s *Session) UpdateToTag(toTag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToTag = toTag
	s.UpdatedAt = time.Now()
}

// UpdateSDP updates the SDP information
func (s *Session) UpdateSDP(local, remote string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LocalSDP = local
	s.RemoteSDP = remote
	s.UpdatedAt = time.Now()
}

// UpdateStatus changes the session status
func (s *Session) UpdateStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.UpdatedAt = time.Now()
}

// AddRecording adds a recording path to the session
func (s *Session) AddRecording(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Recordings = append(s.Recordings, path)
}

// Manager handles the lifecycle of media sessions
type Manager struct {
	ngClient        *ng.Client
	sessions        map[string]*Session
	mu              sync.RWMutex
	logger          *logrus.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	timeout         time.Duration
	cleanupInterval time.Duration
}

// Config holds configuration for the session manager
type Config struct {
	NGAddress       string
	SessionTimeout  time.Duration
	CleanupInterval time.Duration
	Logger          *logrus.Logger
}

// NewManager creates a new session manager
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	if cfg.SessionTimeout == 0 {
		cfg.SessionTimeout = 1 * time.Hour
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	client, err := ng.NewClient(cfg.NGAddress, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create ng client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		ngClient:        client,
		sessions:        make(map[string]*Session),
		logger:          cfg.Logger,
		ctx:             ctx,
		cancel:          cancel,
		timeout:         cfg.SessionTimeout,
		cleanupInterval: cfg.CleanupInterval,
	}

	// Verify RTPengine connectivity
	if err := client.Ping(); err != nil {
		client.Close()
		return nil, fmt.Errorf("rtpengine not responding: %w", err)
	}

	m.logger.Info("Connected to RTPengine")

	// Start cleanup goroutine
	m.wg.Add(1)
	go m.cleanupLoop()

	return m, nil
}

// Close shuts down the session manager
func (m *Manager) Close() error {
	m.cancel()

	// Delete all active sessions
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()

	for _, s := range sessions {
		if err := m.Delete(s.CallID, s.FromTag, s.ToTag); err != nil {
			m.logger.WithError(err).WithField("callid", s.CallID).Warn("Failed to delete session during shutdown")
		}
	}

	m.wg.Wait()
	return m.ngClient.Close()
}

// Offer creates a new media session with an SDP offer
func (m *Manager) Offer(callID, fromTag string, sdp string, options map[string]interface{}) (*Session, string, error) {
	resp, err := m.ngClient.Offer(callID, fromTag, sdp, options)
	if err != nil {
		return nil, "", fmt.Errorf("offer failed: %w", err)
	}

	if resp.Result != "ok" {
		return nil, "", fmt.Errorf("offer rejected: %s", resp.Error)
	}

	var resultData struct {
		SDP string `json:"sdp"`
	}
	if err := json.Unmarshal(resp.Data, &resultData); err != nil {
		m.logger.WithError(err).Warn("Failed to parse offer response")
	}

	session := &Session{
		CallID:    callID,
		FromTag:   fromTag,
		LocalSDP:  resultData.SDP,
		RemoteSDP: sdp,
		Status:    "offered",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.timeout),
	}

	if v, ok := options["direction"].([]string); ok && len(v) > 0 {
		session.Direction = v[0]
	}

	m.mu.Lock()
	m.sessions[callID] = session
	m.mu.Unlock()

	m.logger.WithFields(logrus.Fields{
		"callid":  callID,
		"fromtag": fromTag,
	}).Info("Created media session offer")

	return session, resultData.SDP, nil
}

// Answer updates a session with an SDP answer
func (m *Manager) Answer(callID, fromTag, toTag string, sdp string, options map[string]interface{}) (*Session, string, error) {
	resp, err := m.ngClient.Answer(callID, fromTag, toTag, sdp, options)
	if err != nil {
		return nil, "", fmt.Errorf("answer failed: %w", err)
	}

	if resp.Result != "ok" {
		return nil, "", fmt.Errorf("answer rejected: %s", resp.Error)
	}

	var resultData struct {
		SDP string `json:"sdp"`
	}
	if err := json.Unmarshal(resp.Data, &resultData); err != nil {
		m.logger.WithError(err).Warn("Failed to parse answer response")
	}

	m.mu.Lock()
	session, exists := m.sessions[callID]
	if !exists {
		m.mu.Unlock()
		return nil, "", fmt.Errorf("session not found: %s", callID)
	}
	session.UpdateToTag(toTag)
	session.UpdateSDP(resultData.SDP, sdp)
	session.UpdateStatus("active")
	session.ExpiresAt = time.Now().Add(m.timeout)
	m.mu.Unlock()

	m.logger.WithFields(logrus.Fields{
		"callid": callID,
		"totag":  toTag,
	}).Info("Session established")

	return session, resultData.SDP, nil
}

// Delete removes a media session
func (m *Manager) Delete(callID, fromTag, toTag string) error {
	resp, err := m.ngClient.Delete(callID, fromTag, toTag)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	if resp.Result != "ok" && resp.Result != "deleted" {
		m.logger.WithFields(logrus.Fields{
			"callid": callID,
			"result": resp.Result,
		}).Warn("Unexpected delete response")
	}

	m.mu.Lock()
	delete(m.sessions, callID)
	m.mu.Unlock()

	m.logger.WithField("callid", callID).Info("Deleted media session")
	return nil
}

// Query retrieves session information from RTPengine
func (m *Manager) Query(callID, fromTag, toTag string) (map[string]interface{}, error) {
	resp, err := m.ngClient.Query(callID, fromTag, toTag)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if resp.Result != "ok" {
		return nil, fmt.Errorf("query rejected: %s", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse query response: %w", err)
	}

	return result, nil
}

// List returns all active sessions
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// Get returns a specific session by call ID
func (m *Manager) Get(callID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, exists := m.sessions[callID]
	return session, exists
}

// StartRecording begins recording a session
func (m *Manager) StartRecording(callID, fromTag, toTag string) error {
	resp, err := m.ngClient.StartRecording(callID, fromTag, toTag)
	if err != nil {
		return fmt.Errorf("start recording failed: %w", err)
	}

	if resp.Result != "ok" {
		return fmt.Errorf("start recording rejected: %s", resp.Error)
	}

	m.mu.Lock()
	if session, exists := m.sessions[callID]; exists {
		session.AddRecording(fmt.Sprintf("/recordings/%s", callID))
	}
	m.mu.Unlock()

	m.logger.WithField("callid", callID).Info("Started recording")
	return nil
}

// StopRecording stops recording a session
func (m *Manager) StopRecording(callID, fromTag, toTag string) error {
	resp, err := m.ngClient.StopRecording(callID, fromTag, toTag)
	if err != nil {
		return fmt.Errorf("stop recording failed: %w", err)
	}

	if resp.Result != "ok" {
		return fmt.Errorf("stop recording rejected: %s", resp.Error)
	}

	m.logger.WithField("callid", callID).Info("Stopped recording")
	return nil
}

// cleanupLoop periodically cleans up expired sessions
func (m *Manager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.ctx.Done():
			return
		}
	}
}

// cleanup removes expired sessions
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for callID, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			m.logger.WithField("callid", callID).Debug("Session expired, deleting")
			// Delete from RTPengine asynchronously
			go func(cid, fromTag, toTag string) {
				if _, err := m.ngClient.Delete(cid, fromTag, toTag); err != nil {
					m.logger.WithError(err).WithField("callid", cid).Warn("Failed to delete expired session")
				}
			}(callID, session.FromTag, session.ToTag)
			delete(m.sessions, callID)
		}
	}
}
