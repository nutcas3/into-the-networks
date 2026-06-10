package fcm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Service handles Firebase Cloud Messaging push notifications
type Service struct {
	serverKey   string
	projectID   string
	httpClient  *http.Client
	mu          sync.RWMutex
	logger      *logrus.Logger
}

// Message represents an FCM push notification message
type Message struct {
	To       string                 `json:"to,omitempty"`
	Token    string                 `json:"token,omitempty"`
	Topic    string                 `json:"topic,omitempty"`
	Data     map[string]string      `json:"data,omitempty"`
	Notification *Notification      `json:"notification,omitempty"`
	Android  *AndroidConfig         `json:"android,omitempty"`
	APNS     *APNSConfig            `json:"apns,omitempty"`
	Priority string                 `json:"priority,omitempty"`
}

// Notification represents the display notification
type Notification struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

// AndroidConfig represents Android-specific options
type AndroidConfig struct {
	Priority    string            `json:"priority,omitempty"`
	TTL         string            `json:"ttl,omitempty"`
	Data        map[string]string `json:"data,omitempty"`
	Notification *AndroidNotification `json:"notification,omitempty"`
}

// AndroidNotification represents Android display notification
type AndroidNotification struct {
	Title        string `json:"title,omitempty"`
	Body         string `json:"body,omitempty"`
	ChannelID    string `json:"channel_id,omitempty"`
	Sound        string `json:"sound,omitempty"`
	Priority     string `json:"priority,omitempty"`
	Visibility   string `json:"visibility,omitempty"`
}

// APNSConfig represents APNS-specific options
type APNSConfig struct {
	Headers map[string]string `json:"headers,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// Response represents FCM API response
type Response struct {
	Name    string `json:"name,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// Config holds FCM service configuration
type Config struct {
	ServerKey string
	ProjectID string
	Logger    *logrus.Logger
}

// NewService creates a new FCM service
func NewService(cfg Config) *Service {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	return &Service{
		serverKey:  cfg.ServerKey,
		projectID:  cfg.ProjectID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     cfg.Logger,
	}
}

// SendPush sends a push notification to a device
func (s *Service) SendPush(token string, data map[string]string) error {
	msg := Message{
		To:       token,
		Data:     data,
		Priority: "high",
		Android: &AndroidConfig{
			Priority: "high",
			TTL:      "0s",
			Data:     data,
			Notification: &AndroidNotification{
				Title:      "Incoming Call",
				Body:       "You have an incoming call",
				ChannelID:  "incoming_calls",
				Sound:      "ringtone",
				Priority:   "high",
				Visibility: "public",
			},
		},
	}

	return s.send(msg)
}

// SendVoIPPush sends a high-priority VoIP push notification
func (s *Service) SendVoIPPush(token string, callerID, callerName, sessionID string) error {
	data := map[string]string{
		"type":        "voip_incoming_call",
		"caller_id":   callerID,
		"caller_name": callerName,
		"session_id":  sessionID,
		"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
	}

	msg := Message{
		To:       token,
		Data:     data,
		Priority: "high",
		Android: &AndroidConfig{
			Priority: "high",
			TTL:      "0s",
			Data:     data,
			Notification: &AndroidNotification{
				Title:      "Incoming Call",
				Body:       fmt.Sprintf("%s is calling", callerName),
				ChannelID:  "voip_calls",
				Sound:      "ringtone",
				Priority:   "high",
				Visibility: "public",
			},
		},
	}

	return s.send(msg)
}

// send delivers the message to FCM
func (s *Service) send(msg Message) error {
	if s.serverKey == "" {
		return fmt.Errorf("FCM server key not configured")
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("key=%s", s.serverKey))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send push: %w", err)
	}
	defer resp.Body.Close()

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FCM returned status %d: %s", resp.StatusCode, result.Error)
	}

	s.logger.WithFields(logrus.Fields{
		"to":     msg.To,
		"status": resp.StatusCode,
	}).Info("FCM push sent")

	return nil
}

// ValidateToken checks if a token is valid by sending a silent ping
func (s *Service) ValidateToken(token string) bool {
	msg := Message{
		To:       token,
		Priority: "high",
		Data: map[string]string{
			"type": "ping",
		},
	}

	err := s.send(msg)
	if err != nil {
		s.logger.WithError(err).WithField("token", token[:min(len(token), 20)]+"...").Warn("Token validation failed")
		return false
	}
	return true
}
