package device

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Platform represents the mobile OS platform
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// Device represents a registered mobile device
type Device struct {
	ID           string
	UserID       string
	Platform     Platform
	PushToken    string
	VoIPToken    string     // iOS specific VoIP token
	AppVersion   string
	OSVersion    string
	DeviceModel  string
	RegisteredAt time.Time
	LastActive   time.Time
	Enabled      bool
}

// Manager handles device registration and token management
type Manager struct {
	devices map[string]*Device // key: userID
	tokens  map[string]string  // key: pushToken, value: userID
	mu      sync.RWMutex
	logger  *logrus.Logger
}

// NewManager creates a new device manager
func NewManager(logger *logrus.Logger) *Manager {
	if logger == nil {
		logger = logrus.New()
	}

	return &Manager{
		devices: make(map[string]*Device),
		tokens:  make(map[string]string),
		logger:  logger,
	}
}

// Register registers a new device or updates an existing one
func (m *Manager) Register(device *Device) error {
	if device.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if device.PushToken == "" {
		return fmt.Errorf("push token is required")
	}
	if device.Platform != PlatformIOS && device.Platform != PlatformAndroid {
		return fmt.Errorf("invalid platform: %s", device.Platform)
	}

	device.RegisteredAt = time.Now()
	device.LastActive = time.Now()
	device.Enabled = true

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove old token mapping if device already exists
	if existing, exists := m.devices[device.UserID]; exists {
		delete(m.tokens, existing.PushToken)
		if existing.VoIPToken != "" {
			delete(m.tokens, existing.VoIPToken)
		}
	}

	m.devices[device.UserID] = device
	m.tokens[device.PushToken] = device.UserID
	if device.VoIPToken != "" {
		m.tokens[device.VoIPToken] = device.UserID
	}

	m.logger.WithFields(logrus.Fields{
		"user_id":   device.UserID,
		"platform":  device.Platform,
		"device_id": device.ID,
	}).Info("Device registered")

	return nil
}

// Unregister removes a device registration
func (m *Manager) Unregister(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[userID]
	if !exists {
		return fmt.Errorf("device not found for user: %s", userID)
	}

	delete(m.tokens, device.PushToken)
	if device.VoIPToken != "" {
		delete(m.tokens, device.VoIPToken)
	}
	delete(m.devices, userID)

	m.logger.WithField("user_id", userID).Info("Device unregistered")

	return nil
}

// GetDevice retrieves a device by user ID
func (m *Manager) GetDevice(userID string) (*Device, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[userID]
	return device, exists
}

// GetDeviceByToken retrieves a device by push token
func (m *Manager) GetDeviceByToken(token string) (*Device, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userID, exists := m.tokens[token]
	if !exists {
		return nil, false
	}

	device, exists := m.devices[userID]
	return device, exists
}

// UpdateToken updates the push token for a device
func (m *Manager) UpdateToken(userID, newToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[userID]
	if !exists {
		return fmt.Errorf("device not found for user: %s", userID)
	}

	// Remove old token mapping
	delete(m.tokens, device.PushToken)

	device.PushToken = newToken
	device.LastActive = time.Now()
	m.tokens[newToken] = userID

	m.logger.WithField("user_id", userID).Info("Push token updated")

	return nil
}

// UpdateVoIPToken updates the VoIP token for an iOS device
func (m *Manager) UpdateVoIPToken(userID, voipToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, exists := m.devices[userID]
	if !exists {
		return fmt.Errorf("device not found for user: %s", userID)
	}

	if device.VoIPToken != "" {
		delete(m.tokens, device.VoIPToken)
	}

	device.VoIPToken = voipToken
	device.LastActive = time.Now()
	m.tokens[voipToken] = userID

	m.logger.WithField("user_id", userID).Info("VoIP token updated")

	return nil
}

// GetAllDevices returns all registered devices
func (m *Manager) GetAllDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}

	return devices
}

// GetDevicesByPlatform returns all devices for a specific platform
func (m *Manager) GetDevicesByPlatform(platform Platform) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, device := range m.devices {
		if device.Platform == platform && device.Enabled {
			devices = append(devices, device)
		}
	}

	return devices
}

// CleanupInactive removes devices that haven't been active for the specified duration
func (m *Manager) CleanupInactive(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for userID, device := range m.devices {
		if device.LastActive.Before(cutoff) {
			delete(m.tokens, device.PushToken)
			if device.VoIPToken != "" {
				delete(m.tokens, device.VoIPToken)
			}
			delete(m.devices, userID)
			removed++
		}
	}

	m.logger.WithField("removed", removed).Info("Cleaned up inactive devices")

	return removed
}

// Count returns the total number of registered devices
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.devices)
}

// CountByPlatform returns the number of devices per platform
func (m *Manager) CountByPlatform() map[Platform]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[Platform]int)
	for _, device := range m.devices {
		if device.Enabled {
			counts[device.Platform]++
		}
	}

	return counts
}
