package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

func NewSessionManager() *SessionManager {
	logger := NewLogrusLogger()
	logger.SetLevel("info")

	return &SessionManager{
		sessions: make(map[string]*Session),
		tokens:   make(map[uuid.UUID]string),
		logger:   logger,
	}
}

func (sm *SessionManager) CreateSession(tenantID uuid.UUID, userID string, expiresIn time.Duration) (*Session, error) {
	sessionID := uuid.New()
	token := generateSecureToken()

	session := &Session{
		ID:        sessionID,
		TenantID:  tenantID,
		UserID:    userID,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(expiresIn),
		LastUsed:  time.Now(),
		Active:    true,
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessions[token] = session
	sm.tokens[sessionID] = token

	sm.logger.Infof("Session created: %s for tenant: %s", sessionID, tenantID)

	return session, nil
}

func (sm *SessionManager) ValidateSession(token string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[token]
	if !exists {
		return nil, ErrSessionNotFound
	}

	if !session.Active {
		return nil, ErrSessionInactive
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	// Update last used time
	session.LastUsed = time.Now()

	return session, nil
}

func (sm *SessionManager) InvalidateSession(token string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[token]
	if !exists {
		return ErrSessionNotFound
	}

	session.Active = false
	delete(sm.tokens, session.ID)

	sm.logger.Infof("Session invalidated: %s", session.ID)

	return nil
}

func (sm *SessionManager) InvalidateUserSessions(tenantID uuid.UUID, userID string) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	count := 0
	for token, session := range sm.sessions {
		if session.TenantID == tenantID && session.UserID == userID {
			session.Active = false
			delete(sm.tokens, session.ID)
			delete(sm.sessions, token)
			count++
		}
	}

	sm.logger.Infof("Invalidated %d sessions for user: %s in tenant: %s", count, userID, tenantID)

	return count
}

func (sm *SessionManager) CleanupExpiredSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	count := 0

	for token, session := range sm.sessions {
		if now.After(session.ExpiresAt) || !session.Active {
			delete(sm.sessions, token)
			delete(sm.tokens, session.ID)
			count++
		}
	}

	if count > 0 {
		sm.logger.Infof("Cleaned up %d expired sessions", count)
	}

	return count
}

func (sm *SessionManager) GetActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	now := time.Now()

	for _, session := range sm.sessions {
		if session.Active && now.Before(session.ExpiresAt) {
			count++
		}
	}

	return count
}

func (sm *SessionManager) GetSessionStats() map[string]any {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	now := time.Now()
	activeCount := 0
	expiredCount := 0
	inactiveCount := 0

	for _, session := range sm.sessions {
		if !session.Active {
			inactiveCount++
		} else if now.After(session.ExpiresAt) {
			expiredCount++
		} else {
			activeCount++
		}
	}

	return map[string]any{
		"total_sessions":    len(sm.sessions),
		"active_sessions":   activeCount,
		"expired_sessions":  expiredCount,
		"inactive_sessions": inactiveCount,
		"tokens":            len(sm.tokens),
	}
}

func generateSecureToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to less secure method if crypto/rand fails
		return uuid.New().String()
	}
	return hex.EncodeToString(bytes)
}

// Error definitions
var (
	ErrSessionNotFound    = &AuthError{Code: "SESSION_NOT_FOUND", Message: "Session not found"}
	ErrSessionInactive    = &AuthError{Code: "SESSION_INACTIVE", Message: "Session is inactive"}
	ErrSessionExpired     = &AuthError{Code: "SESSION_EXPIRED", Message: "Session has expired"}
	ErrInvalidToken       = &AuthError{Code: "INVALID_TOKEN", Message: "Invalid authentication token"}
	ErrInsufficientScopes = &AuthError{Code: "INSUFFICIENT_SCOPES", Message: "Insufficient scopes for this operation"}
)

type AuthError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AuthError) Error() string {
	return e.Message
}
