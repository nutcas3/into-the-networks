package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTManager_GenerateToken(t *testing.T) {
	jwtMgr := NewJWTManager("test-secret", 24*time.Hour)
	
	userID := uuid.New()
	tenantID := uuid.New()
	role := "admin"
	
	token, err := jwtMgr.GenerateToken(userID, tenantID, role)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	
	if token == "" {
		t.Error("Expected non-empty token")
	}
}

func TestJWTManager_ValidateToken(t *testing.T) {
	jwtMgr := NewJWTManager("test-secret", 24*time.Hour)
	
	userID := uuid.New()
	tenantID := uuid.New()
	role := "admin"
	
	token, err := jwtMgr.GenerateToken(userID, tenantID, role)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	
	claims, err := jwtMgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	
	if claims.UserID != userID {
		t.Errorf("Expected UserID %v, got %v", userID, claims.UserID)
	}
	
	if claims.TenantID != tenantID {
		t.Errorf("Expected TenantID %v, got %v", tenantID, claims.TenantID)
	}
	
	if claims.Role != role {
		t.Errorf("Expected Role %s, got %s", role, claims.Role)
	}
}

func TestJWTManager_InvalidToken(t *testing.T) {
	jwtMgr := NewJWTManager("test-secret", 24*time.Hour)
	
	_, err := jwtMgr.ValidateToken("invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

func TestJWTManager_RefreshToken(t *testing.T) {
	jwtMgr := NewJWTManager("test-secret", 1*time.Hour)
	
	userID := uuid.New()
	tenantID := uuid.New()
	role := "admin"
	
	token, err := jwtMgr.GenerateToken(userID, tenantID, role)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	
	// Wait for token to be close to expiration
	time.Sleep(35 * time.Minute)
	
	newToken, err := jwtMgr.RefreshToken(token)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	
	if newToken == "" {
		t.Error("Expected non-empty refreshed token")
	}
}
