package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nutcas3/esl-resilience/internal/tenant"
	"github.com/sirupsen/logrus"
)

func NewAuthenticator(tenantMgr *tenant.Manager) *Authenticator {
	logger := NewLogrusLogger()
	logger.SetLevel("info")

	return &Authenticator{
		tenantMgr:  tenantMgr,
		apiKeys:    make(map[string]*APIKey),
		sessionMgr: NewSessionManager(),
		logger:     logger,
	}
}

// API Key Authentication
func (a *Authenticator) GenerateAPIKey(tenantID uuid.UUID, name string, scopes []string, expiresIn time.Duration) (*APIKey, error) {
	// Validate tenant exists and is active
	if err := a.tenantMgr.ValidateTenantAccess(context.Background(), tenantID, ""); err != nil {
		return nil, fmt.Errorf("tenant access validation failed: %w", err)
	}

	// Generate API key
	key := uuid.New().String() + uuid.New().String()
	hash := a.hashAPIKey(key)

	apiKey := &APIKey{
		ID:        uuid.New(),
		TenantID:  tenantID,
		KeyHash:   hash,
		Name:      name,
		Scopes:    scopes,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(expiresIn),
		Active:    true,
	}

	// Store the hash (not the actual key)
	a.apiKeys[hash] = apiKey

	a.logger.WithFields(logrus.Fields{
		"api_key_id": apiKey.ID,
		"tenant_id":  tenantID,
		"name":       name,
	}).Info("API key generated")

	// Return the actual key (only time it's shown)
	return &APIKey{
		ID:        apiKey.ID,
		TenantID:  tenantID,
		KeyHash:   key, // Return actual key for user
		Name:      name,
		Scopes:    scopes,
		CreatedAt: apiKey.CreatedAt,
		ExpiresAt: apiKey.ExpiresAt,
		LastUsed:  apiKey.LastUsed,
		Active:    true,
	}, nil
}

func (a *Authenticator) hashAPIKey(key string) string {
	h := hmac.New(sha256.New, []byte("esl-resilience-api-key-salt"))
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

func (a *Authenticator) ValidateAPIKey(key string) (*APIKey, error) {
	hash := a.hashAPIKey(key)

	apiKey, exists := a.apiKeys[hash]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}

	if !apiKey.Active {
		return nil, fmt.Errorf("API key is deactivated")
	}

	if time.Now().After(apiKey.ExpiresAt) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Validate tenant is still active
	if err := a.tenantMgr.ValidateTenantAccess(context.Background(), apiKey.TenantID, ""); err != nil {
		return nil, fmt.Errorf("tenant access validation failed: %w", err)
	}

	// Update last used time
	apiKey.LastUsed = time.Now()

	return apiKey, nil
}

func (a *Authenticator) RevokeAPIKey(keyHash string) error {
	apiKey, exists := a.apiKeys[keyHash]
	if !exists {
		return fmt.Errorf("API key not found")
	}

	apiKey.Active = false

	a.logger.WithFields(logrus.Fields{
		"api_key_id": apiKey.ID,
		"tenant_id":  apiKey.TenantID,
	}).Info("API key revoked")

	return nil
}

// HTTP Middleware
func (a *Authenticator) RequireAuth(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try API key authentication first
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				if err := a.authenticateWithAPIKey(r, apiKey, scopes); err != nil {
					http.Error(w, err.Error(), http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Try session authentication (Bearer token)
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if err := a.authenticateWithSession(r, token, scopes); err != nil {
					http.Error(w, err.Error(), http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// No authentication provided
			http.Error(w, "Authentication required", http.StatusUnauthorized)
		})
	}
}

func (a *Authenticator) authenticateWithAPIKey(r *http.Request, key string, requiredScopes []string) error {
	apiKey, err := a.ValidateAPIKey(key)
	if err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}

	// Check scopes
	if !a.hasRequiredScopes(apiKey.Scopes, requiredScopes) {
		return fmt.Errorf("insufficient scopes")
	}

	// Add tenant info to context
	ctx := context.WithValue(r.Context(), TenantIDKey, apiKey.TenantID)
	ctx = context.WithValue(ctx, UserIDKey, "api-key")
	ctx = context.WithValue(ctx, ScopesKey, apiKey.Scopes)

	*r = *r.WithContext(ctx)
	return nil
}

func (a *Authenticator) authenticateWithSession(r *http.Request, token string, requiredScopes []string) error {
	session, err := a.sessionMgr.ValidateSession(token)
	if err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	// Validate tenant is still active
	if err := a.tenantMgr.ValidateTenantAccess(context.Background(), session.TenantID, ""); err != nil {
		return fmt.Errorf("tenant access validation failed: %w", err)
	}

	// For sessions, we'd typically get scopes from user roles
	// For now, we'll use default scopes
	scopes := []string{"read", "write"}

	// Check scopes
	if !a.hasRequiredScopes(scopes, requiredScopes) {
		return fmt.Errorf("insufficient scopes")
	}

	// Add tenant info to context
	ctx := context.WithValue(r.Context(), TenantIDKey, session.TenantID)
	ctx = context.WithValue(ctx, UserIDKey, session.UserID)
	ctx = context.WithValue(ctx, ScopesKey, scopes)
	ctx = context.WithValue(ctx, SessionIDKey, session.ID)

	*r = *r.WithContext(ctx)
	return nil
}

func (a *Authenticator) hasRequiredScopes(userScopes, requiredScopes []string) bool {
	if len(requiredScopes) == 0 {
		return true
	}

	scopeSet := make(map[string]bool)
	for _, scope := range userScopes {
		scopeSet[scope] = true
	}

	for _, required := range requiredScopes {
		if !scopeSet[required] {
			return false
		}
	}

	return true
}

// Helper functions for extracting context values
func GetTenantID(ctx context.Context) (uuid.UUID, bool) {
	tenantID, ok := ctx.Value(TenantIDKey).(uuid.UUID)
	return tenantID, ok
}

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

func GetScopes(ctx context.Context) ([]string, bool) {
	scopes, ok := ctx.Value(ScopesKey).([]string)
	return scopes, ok
}

func GetSessionID(ctx context.Context) (uuid.UUID, bool) {
	sessionID, ok := ctx.Value(SessionIDKey).(uuid.UUID)
	return sessionID, ok
}

// Tenant validation middleware
func (a *Authenticator) RequireTenantAccess(feature string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, ok := GetTenantID(r.Context())
			if !ok {
				http.Error(w, "Tenant ID not found in context", http.StatusUnauthorized)
				return
			}

			if err := a.tenantMgr.ValidateTenantAccess(r.Context(), tenantID, feature); err != nil {
				http.Error(w, fmt.Sprintf("Tenant access denied: %v", err), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Rate limiting middleware
func (a *Authenticator) RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := GetTenantID(r.Context())
			if !ok {
				next.ServeHTTP(w, r) // Skip rate limiting for unauthenticated requests (they'll be caught by auth middleware)
				return
			}

			// Simple in-memory rate limiting (in production, use Redis or similar)
			// TODO: Implement tenant-specific rate limiting using tenantID
			// This is a placeholder implementation
			next.ServeHTTP(w, r)
		})
	}
}
