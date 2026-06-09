package router

import (
	"fmt"
	"sync"
	"time"

	"github.com/nutcas3/amd-system/internal/call"
	"github.com/sirupsen/logrus"
)

// Service routes calls based on classification results
type Service struct {
	callMgr       *call.Manager
	defaultRoute  string
	amRoute       string
	faxRoute      string
	unknownRoute  string
	maxRetries    int
	mu            sync.RWMutex
	logger        *logrus.Logger
}

// Config holds router configuration
type Config struct {
	CallMgr      *call.Manager
	DefaultRoute string
	AMRoute      string
	FaxRoute     string
	UnknownRoute string
	MaxRetries   int
	Logger       *logrus.Logger
}

// NewService creates a new call router
func NewService(cfg Config) *Service {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	if cfg.DefaultRoute == "" {
		cfg.DefaultRoute = "agent_queue"
	}
	if cfg.AMRoute == "" {
		cfg.AMRoute = "voicemail_drop"
	}
	if cfg.FaxRoute == "" {
		cfg.FaxRoute = "fax_handler"
	}
	if cfg.UnknownRoute == "" {
		cfg.UnknownRoute = "retry_queue"
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}

	return &Service{
		callMgr:      cfg.CallMgr,
		defaultRoute: cfg.DefaultRoute,
		amRoute:      cfg.AMRoute,
		faxRoute:     cfg.FaxRoute,
		unknownRoute: cfg.UnknownRoute,
		maxRetries:   cfg.MaxRetries,
		logger:       cfg.Logger,
	}
}

// Route processes a classification result and routes the call accordingly
func (s *Service) Route(sessionID string, result call.Result, confidence float64) error {
	var destination string
	var action string

	switch result {
	case call.ResultHuman:
		destination = s.defaultRoute
		action = "connect_to_agent"
	case call.ResultAnsweringMachine:
		destination = s.amRoute
		action = "leave_voicemail"
	case call.ResultBeep:
		destination = s.amRoute
		action = "play_after_beep"
	case call.ResultFax:
		destination = s.faxRoute
		action = "handle_fax"
	case call.ResultSilence:
		destination = s.unknownRoute
		action = "retry_silence"
	case call.ResultUnknown:
		destination = s.unknownRoute
		action = "retry_classification"
	}

	if err := s.callMgr.RouteSession(sessionID, destination); err != nil {
		return fmt.Errorf("failed to route session: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"session_id":  sessionID,
		"result":      result,
		"confidence":  confidence,
		"destination": destination,
		"action":      action,
	}).Info("Call routed")

	return nil
}

// ShouldRetry determines if a call should be retried
func (s *Service) ShouldRetry(sessionID string) bool {
	session, exists := s.callMgr.GetSession(sessionID)
	if !exists {
		return false
	}

	// Retry if unknown/silence and max retries not reached
	if (session.Result == call.ResultUnknown || session.Result == call.ResultSilence) &&
		session.RetryCount < s.maxRetries {
		return true
	}

	return false
}

// RecordRetry increments the retry count for a session
func (s *Service) RecordRetry(sessionID string) error {
	session, exists := s.callMgr.GetSession(sessionID)
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.RetryCount++

	s.logger.WithFields(logrus.Fields{
		"session_id":  sessionID,
		"retry_count": session.RetryCount,
	}).Info("Call retry recorded")

	return nil
}

// GetRoutingStats returns routing statistics
func (s *Service) GetRoutingStats() map[string]interface{} {
	stats := s.callMgr.GetStats()

	results, _ := stats["results"].(map[call.Result]int)

	return map[string]interface{}{
		"total_sessions":  stats["total_sessions"],
		"routed_to_agent": results[call.ResultHuman],
		"routed_to_voicemail": results[call.ResultAnsweringMachine] + results[call.ResultBeep],
		"routed_to_fax": results[call.ResultFax],
		"retried":         results[call.ResultUnknown] + results[call.ResultSilence],
		"timestamp":       time.Now().Unix(),
	}
}
