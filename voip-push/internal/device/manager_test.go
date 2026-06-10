package device

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)
	require.NotNil(t, mgr)
	assert.NotNil(t, mgr.devices)
	assert.NotNil(t, mgr.tokens)
	assert.Equal(t, 0, mgr.Count())
}

func TestManagerRegister(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	dev := &Device{
		ID:        "device-1",
		UserID:    "alice",
		Platform:  PlatformIOS,
		PushToken: "apns-token-123",
		VoIPToken: "voip-token-456",
	}

	err := mgr.Register(dev)
	require.NoError(t, err)
	assert.True(t, dev.Enabled)
	assert.False(t, dev.RegisteredAt.IsZero())
	assert.False(t, dev.LastActive.IsZero())

	// Verify device is stored
	stored, exists := mgr.GetDevice("alice")
	require.True(t, exists)
	assert.Equal(t, "device-1", stored.ID)
	assert.Equal(t, "apns-token-123", stored.PushToken)

	// Verify token mapping
	byToken, exists := mgr.GetDeviceByToken("apns-token-123")
	require.True(t, exists)
	assert.Equal(t, "alice", byToken.UserID)

	byVoIP, exists := mgr.GetDeviceByToken("voip-token-456")
	require.True(t, exists)
	assert.Equal(t, "alice", byVoIP.UserID)
}

func TestManagerRegisterInvalid(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	// Missing user ID
	err := mgr.Register(&Device{PushToken: "token", Platform: PlatformIOS})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user ID")

	// Missing push token
	err = mgr.Register(&Device{UserID: "alice", Platform: PlatformIOS})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "push token")

	// Invalid platform
	err = mgr.Register(&Device{UserID: "alice", PushToken: "token", Platform: "windows"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "platform")
}

func TestManagerUpdateToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	dev := &Device{
		ID:        "device-1",
		UserID:    "alice",
		Platform:  PlatformIOS,
		PushToken: "old-token",
	}
	mgr.Register(dev)

	// Update token
	err := mgr.UpdateToken("alice", "new-token")
	require.NoError(t, err)

	// Verify new token mapping
	byNew, exists := mgr.GetDeviceByToken("new-token")
	require.True(t, exists)
	assert.Equal(t, "alice", byNew.UserID)

	// Old token should be gone
	_, exists = mgr.GetDeviceByToken("old-token")
	assert.False(t, exists)
}

func TestManagerUpdateVoIPToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	dev := &Device{
		ID:        "device-1",
		UserID:    "alice",
		Platform:  PlatformIOS,
		PushToken: "push-token",
		VoIPToken: "old-voip",
	}
	mgr.Register(dev)

	// Update VoIP token
	err := mgr.UpdateVoIPToken("alice", "new-voip")
	require.NoError(t, err)

	// Verify new VoIP mapping
	byNew, exists := mgr.GetDeviceByToken("new-voip")
	require.True(t, exists)
	assert.Equal(t, "alice", byNew.UserID)

	// Old VoIP token should be gone
	_, exists = mgr.GetDeviceByToken("old-voip")
	assert.False(t, exists)
}

func TestManagerUnregister(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	dev := &Device{
		ID:        "device-1",
		UserID:    "alice",
		Platform:  PlatformIOS,
		PushToken: "token",
	}
	mgr.Register(dev)
	assert.Equal(t, 1, mgr.Count())

	// Unregister
	err := mgr.Unregister("alice")
	require.NoError(t, err)
	assert.Equal(t, 0, mgr.Count())

	// Device should be gone
	_, exists := mgr.GetDevice("alice")
	assert.False(t, exists)

	// Token mapping should be gone
	_, exists = mgr.GetDeviceByToken("token")
	assert.False(t, exists)
}

func TestManagerUnregisterNotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	err := mgr.Unregister("nobody")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManagerGetDevicesByPlatform(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	mgr.Register(&Device{UserID: "alice", Platform: PlatformIOS, PushToken: "t1"})
	mgr.Register(&Device{UserID: "bob", Platform: PlatformAndroid, PushToken: "t2"})
	mgr.Register(&Device{UserID: "charlie", Platform: PlatformIOS, PushToken: "t3"})

	iosDevices := mgr.GetDevicesByPlatform(PlatformIOS)
	assert.Len(t, iosDevices, 2)

	androidDevices := mgr.GetDevicesByPlatform(PlatformAndroid)
	assert.Len(t, androidDevices, 1)
}

func TestManagerCountByPlatform(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	mgr.Register(&Device{UserID: "alice", Platform: PlatformIOS, PushToken: "t1"})
	mgr.Register(&Device{UserID: "bob", Platform: PlatformAndroid, PushToken: "t2"})
	mgr.Register(&Device{UserID: "charlie", Platform: PlatformIOS, PushToken: "t3"})

	counts := mgr.CountByPlatform()
	assert.Equal(t, 2, counts[PlatformIOS])
	assert.Equal(t, 1, counts[PlatformAndroid])
}

func TestManagerCleanupInactive(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	// Register active device
	mgr.Register(&Device{UserID: "alice", Platform: PlatformIOS, PushToken: "t1"})

	// Register inactive device by manually setting last active
	mgr.Register(&Device{UserID: "bob", Platform: PlatformAndroid, PushToken: "t2"})
	dev, _ := mgr.GetDevice("bob")
	dev.LastActive = time.Now().Add(-100 * 24 * time.Hour) // 100 days ago

	assert.Equal(t, 2, mgr.Count())

	removed := mgr.CleanupInactive(30 * 24 * time.Hour) // 30 days
	assert.Equal(t, 1, removed)
	assert.Equal(t, 1, mgr.Count())

	// Bob should be gone, Alice should remain
	_, exists := mgr.GetDevice("bob")
	assert.False(t, exists)
	_, exists = mgr.GetDevice("alice")
	assert.True(t, exists)
}

func TestManagerReRegisterUpdates(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr := NewManager(logger)

	// First registration
	mgr.Register(&Device{
		ID:        "device-1",
		UserID:    "alice",
		Platform:  PlatformIOS,
		PushToken: "token-1",
	})

	// Re-register with new token
	mgr.Register(&Device{
		ID:        "device-2",
		UserID:    "alice",
		Platform:  PlatformIOS,
		PushToken: "token-2",
	})

	// Should have latest token
	dev, _ := mgr.GetDevice("alice")
	assert.Equal(t, "token-2", dev.PushToken)
	assert.Equal(t, "device-2", dev.ID)

	// Old token should be gone
	_, exists := mgr.GetDeviceByToken("token-1")
	assert.False(t, exists)

	// New token should exist
	_, exists = mgr.GetDeviceByToken("token-2")
	assert.True(t, exists)
}
