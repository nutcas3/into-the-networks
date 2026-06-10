package apns

import (
	"fmt"
	"sync"
	"time"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"github.com/sirupsen/logrus"
)

// Service handles Apple Push Notification Service for VoIP
type Service struct {
	client   *apns2.Client
	bundleID string
	mu       sync.RWMutex
	logger   *logrus.Logger
	useJWT   bool
	keyID    string
	teamID   string
	certPath string
}

// Config holds APNS service configuration
type Config struct {
	BundleID     string
	CertPath     string
	CertPassword string
	KeyID        string
	TeamID       string
	AuthKeyPath  string
	UseJWT       bool
	Logger       *logrus.Logger
}

// NewService creates a new APNS service
func NewService(cfg Config) (*Service, error) {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	s := &Service{
		bundleID: cfg.BundleID,
		logger:   cfg.Logger,
		useJWT:   cfg.UseJWT,
		keyID:    cfg.KeyID,
		teamID:   cfg.TeamID,
		certPath: cfg.CertPath,
	}

	if cfg.UseJWT {
		if cfg.AuthKeyPath == "" {
			return nil, fmt.Errorf("auth key path required for JWT authentication")
		}
		authKey, err := token.AuthKeyFromFile(cfg.AuthKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load auth key: %w", err)
		}
		t := &token.Token{
			AuthKey: authKey,
			KeyID:   cfg.KeyID,
			TeamID:  cfg.TeamID,
		}
		s.client = apns2.NewTokenClient(t)
	} else {
		if cfg.CertPath == "" {
			return nil, fmt.Errorf("certificate path required for certificate authentication")
		}
		cert, err := certificate.FromP12File(cfg.CertPath, cfg.CertPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}
		s.client = apns2.NewClient(cert).Production()
	}

	return s, nil
}

// SendVoIPPush sends a VoIP push notification to an iOS device
func (s *Service) SendVoIPPush(deviceToken, callerID, callerName, sessionID string) error {
	if s.client == nil {
		return fmt.Errorf("APNS client not initialized")
	}

	p := payload.NewPayload().
		AlertTitle("Incoming Call").
		AlertBody(fmt.Sprintf("%s is calling", callerName)).
		Sound("ringtone.caf").
		ContentAvailable().
		Custom("caller_id", callerID).
		Custom("caller_name", callerName).
		Custom("session_id", sessionID).
		Custom("type", "voip_incoming_call").
		Custom("timestamp", time.Now().Unix())

	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       fmt.Sprintf("%s.voip", s.bundleID),
		Payload:     p,
		Expiration:  time.Now().Add(30 * time.Second),
	}

	resp, err := s.client.Push(notification)
	if err != nil {
		s.logger.WithError(err).WithField("token", truncateToken(deviceToken)).Error("Failed to send VoIP push")
		return fmt.Errorf("failed to send VoIP push: %w", err)
	}

	if !resp.Sent() {
		s.logger.WithFields(logrus.Fields{
			"reason": resp.Reason,
			"status": resp.StatusCode,
		}).Error("VoIP push rejected")
		return fmt.Errorf("VoIP push rejected: %s (status %d)", resp.Reason, resp.StatusCode)
	}

	s.logger.WithFields(logrus.Fields{
		"apns_id": resp.ApnsID,
		"token":   truncateToken(deviceToken),
	}).Info("VoIP push sent successfully")

	return nil
}

// SendSilentPush sends a silent push notification for background updates
func (s *Service) SendSilentPush(deviceToken string, data map[string]string) error {
	if s.client == nil {
		return fmt.Errorf("APNS client not initialized")
	}

	p := payload.NewPayload().ContentAvailable()
	for k, v := range data {
		p.Custom(k, v)
	}

	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       s.bundleID,
		Payload:     p,
		Expiration:  time.Now().Add(24 * time.Hour),
	}

	resp, err := s.client.Push(notification)
	if err != nil {
		return fmt.Errorf("failed to send silent push: %w", err)
	}

	if !resp.Sent() {
		return fmt.Errorf("silent push rejected: %s", resp.Reason)
	}

	return nil
}

// ValidateToken checks if a device token is valid
func (s *Service) ValidateToken(deviceToken string) bool {
	p := payload.NewPayload().
		ContentAvailable().
		Custom("type", "ping")

	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       s.bundleID,
		Payload:     p,
		Expiration:  time.Now().Add(10 * time.Second),
	}

	resp, err := s.client.Push(notification)
	if err != nil {
		return false
	}

	if resp.StatusCode == 410 {
		// Token is no longer valid
		return false
	}

	return resp.Sent()
}

func truncateToken(token string) string {
	if len(token) <= 20 {
		return token
	}
	return token[:20] + "..."
}
