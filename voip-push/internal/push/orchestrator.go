package push

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nutcas3/voip-push/internal/apns"
	"github.com/nutcas3/voip-push/internal/device"
	"github.com/nutcas3/voip-push/internal/fcm"
	"github.com/sirupsen/logrus"
)

// Orchestrator coordinates push notifications across platforms
type Orchestrator struct {
	fcmService   *fcm.Service
	apnsService  *apns.Service
	deviceMgr    *device.Manager
	mu           sync.RWMutex
	logger       *logrus.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	retryCount   int
	retryDelay   time.Duration
	pushTimeout  time.Duration
}

// Config holds orchestrator configuration
type Config struct {
	FCMService   *fcm.Service
	APNSService  *apns.Service
	DeviceMgr    *device.Manager
	RetryCount   int
	RetryDelay   time.Duration
	PushTimeout  time.Duration
	Logger       *logrus.Logger
}

// PushResult represents the result of a push notification attempt
type PushResult struct {
	Success     bool
	Platform    device.Platform
	Token       string
	Error       error
	Timestamp   time.Time
	RetryCount  int
}

// NewOrchestrator creates a new push orchestrator
func NewOrchestrator(cfg Config) *Orchestrator {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	if cfg.RetryCount <= 0 {
		cfg.RetryCount = 3
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 2 * time.Second
	}
	if cfg.PushTimeout <= 0 {
		cfg.PushTimeout = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Orchestrator{
		fcmService:  cfg.FCMService,
		apnsService: cfg.APNSService,
		deviceMgr:   cfg.DeviceMgr,
		logger:      cfg.Logger,
		ctx:         ctx,
		cancel:      cancel,
		retryCount:  cfg.RetryCount,
		retryDelay:  cfg.RetryDelay,
		pushTimeout: cfg.PushTimeout,
	}
}

// Close shuts down the orchestrator
func (o *Orchestrator) Close() {
	o.cancel()
}

// SendIncomingCallPush sends a push notification for an incoming call
func (o *Orchestrator) SendIncomingCallPush(userID, callerID, callerName, sessionID string) (*PushResult, error) {
	dev, exists := o.deviceMgr.GetDevice(userID)
	if !exists {
		return nil, fmt.Errorf("no device registered for user: %s", userID)
	}

	if !dev.Enabled {
		return nil, fmt.Errorf("device is disabled for user: %s", userID)
	}

	result := &PushResult{
		Platform:  dev.Platform,
		Token:     dev.PushToken,
		Timestamp: time.Now(),
	}

	switch dev.Platform {
	case device.PlatformIOS:
		result = o.sendIOSPush(dev, callerID, callerName, sessionID)
	case device.PlatformAndroid:
		result = o.sendAndroidPush(dev, callerID, callerName, sessionID)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", dev.Platform)
	}

	return result, nil
}

// sendIOSPush sends a VoIP push to iOS device
func (o *Orchestrator) sendIOSPush(dev *device.Device, callerID, callerName, sessionID string) *PushResult {
	result := &PushResult{
		Platform:  device.PlatformIOS,
		Token:     dev.VoIPToken,
		Timestamp: time.Now(),
	}

	if o.apnsService == nil {
		result.Error = fmt.Errorf("APNS service not configured")
		return result
	}

	if dev.VoIPToken == "" {
		result.Error = fmt.Errorf("no VoIP token for device")
		return result
	}

	var lastErr error
	for i := 0; i <= o.retryCount; i++ {
		ctx, cancel := context.WithTimeout(o.ctx, o.pushTimeout)
		
		errChan := make(chan error, 1)
		go func() {
			errChan <- o.apnsService.SendVoIPPush(dev.VoIPToken, callerID, callerName, sessionID)
		}()

		select {
		case err := <-errChan:
			if err == nil {
				result.Success = true
				result.RetryCount = i
				cancel()
				return result
			}
			lastErr = err
			o.logger.WithError(err).WithField("attempt", i+1).Warn("VoIP push failed, retrying")
		case <-ctx.Done():
			lastErr = fmt.Errorf("push timeout")
			o.logger.WithField("attempt", i+1).Warn("VoIP push timeout")
		}
		cancel()

		if i < o.retryCount {
			time.Sleep(o.retryDelay)
		}
	}

	result.Error = lastErr
	result.RetryCount = o.retryCount

	// If token is invalid, disable the device
	if lastErr != nil {
		if o.isInvalidTokenError(lastErr) {
			dev.Enabled = false
			o.logger.WithField("user_id", dev.UserID).Warn("Device disabled due to invalid token")
		}
	}

	return result
}

// sendAndroidPush sends a VoIP push to Android device
func (o *Orchestrator) sendAndroidPush(dev *device.Device, callerID, callerName, sessionID string) *PushResult {
	result := &PushResult{
		Platform:  device.PlatformAndroid,
		Token:     dev.PushToken,
		Timestamp: time.Now(),
	}

	if o.fcmService == nil {
		result.Error = fmt.Errorf("FCM service not configured")
		return result
	}

	var lastErr error
	for i := 0; i <= o.retryCount; i++ {
		ctx, cancel := context.WithTimeout(o.ctx, o.pushTimeout)
		
		errChan := make(chan error, 1)
		go func() {
			errChan <- o.fcmService.SendVoIPPush(dev.PushToken, callerID, callerName, sessionID)
		}()

		select {
		case err := <-errChan:
			if err == nil {
				result.Success = true
				result.RetryCount = i
				cancel()
				return result
			}
			lastErr = err
			o.logger.WithError(err).WithField("attempt", i+1).Warn("FCM push failed, retrying")
		case <-ctx.Done():
			lastErr = fmt.Errorf("push timeout")
			o.logger.WithField("attempt", i+1).Warn("FCM push timeout")
		}
		cancel()

		if i < o.retryCount {
			time.Sleep(o.retryDelay)
		}
	}

	result.Error = lastErr
	result.RetryCount = o.retryCount
	return result
}

// SendSilentPush sends a silent/background push notification
func (o *Orchestrator) SendSilentPush(userID string, data map[string]string) error {
	dev, exists := o.deviceMgr.GetDevice(userID)
	if !exists {
		return fmt.Errorf("no device registered for user: %s", userID)
	}

	if !dev.Enabled {
		return fmt.Errorf("device is disabled for user: %s", userID)
	}

	switch dev.Platform {
	case device.PlatformIOS:
		if o.apnsService == nil {
			return fmt.Errorf("APNS service not configured")
		}
		return o.apnsService.SendSilentPush(dev.PushToken, data)
	case device.PlatformAndroid:
		if o.fcmService == nil {
			return fmt.Errorf("FCM service not configured")
		}
		return o.fcmService.SendPush(dev.PushToken, data)
	default:
		return fmt.Errorf("unsupported platform: %s", dev.Platform)
	}
}

// ValidateDeviceToken validates a device's push token
func (o *Orchestrator) ValidateDeviceToken(userID string) bool {
	dev, exists := o.deviceMgr.GetDevice(userID)
	if !exists {
		return false
	}

	switch dev.Platform {
	case device.PlatformIOS:
		if o.apnsService == nil {
			return false
		}
		return o.apnsService.ValidateToken(dev.PushToken)
	case device.PlatformAndroid:
		if o.fcmService == nil {
			return false
		}
		return o.fcmService.ValidateToken(dev.PushToken)
	default:
		return false
	}
}

// isInvalidTokenError checks if an error indicates an invalid token
func (o *Orchestrator) isInvalidTokenError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	invalidTokenIndicators := []string{
		"BadDeviceToken",
		"Unregistered",
		"invalid token",
		"NotRegistered",
	}

	for _, indicator := range invalidTokenIndicators {
		if contains(errStr, indicator) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetStats returns push notification statistics
func (o *Orchestrator) GetStats() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return map[string]interface{}{
		"total_devices":       o.deviceMgr.Count(),
		"devices_by_platform": o.deviceMgr.CountByPlatform(),
	}
}
