package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nutcas3/esl-resilience/internal/tenant"
)

type Authenticator struct {
	tenantMgr  TenantManager
	apiKeys    map[string]*APIKey
	sessionMgr *SessionManager
	logger     Logger
}

type APIKey struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	KeyHash   string    `json:"key_hash"`
	Name      string    `json:"name"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastUsed  time.Time `json:"last_used"`
	Active    bool      `json:"active"`
}

type Session struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastUsed  time.Time `json:"last_used"`
	Active    bool      `json:"active"`
}

type SessionManager struct {
	sessions map[string]*Session
	tokens   map[uuid.UUID]string // session ID -> token
	mu       RWMutex
	logger   Logger
}

type Claims struct {
	TenantID  uuid.UUID `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	Scopes    []string  `json:"scopes"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ContextKey string

const (
	TenantIDKey  ContextKey = "tenant_id"
	UserIDKey    ContextKey = "user_id"
	ScopesKey    ContextKey = "scopes"
	SessionIDKey ContextKey = "session_id"
)

// Interfaces for dependency injection
type TenantManager interface {
	ValidateTenantAccess(ctx context.Context, tenantID uuid.UUID, feature string) error
	GetTenant(ctx context.Context, tenantID uuid.UUID) (*tenant.Tenant, error)
}

type Logger interface {
	Info(args ...any)
	Infof(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Debug(args ...any)
	Debugf(format string, args ...any)
	WithField(key string, value any) Logger
	WithFields(fields map[string]any) Logger
}

// RWMutex interface for testing
type RWMutex interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}
