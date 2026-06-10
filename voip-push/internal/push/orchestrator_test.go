package push

import (
	"fmt"
	"testing"

	"github.com/nutcas3/voip-push/internal/device"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOrchestrator(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	deviceMgr := device.NewManager(logger)

	orch := NewOrchestrator(Config{
		DeviceMgr:   deviceMgr,
		RetryCount:  3,
		RetryDelay:  0,
		PushTimeout: 1,
		Logger:      logger,
	})

	require.NotNil(t, orch)
	assert.NotNil(t, orch.deviceMgr)
	assert.Equal(t, 3, orch.retryCount)
}

func TestOrchestratorSendIncomingCallPushNoDevice(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	deviceMgr := device.NewManager(logger)
	orch := NewOrchestrator(Config{
		DeviceMgr: deviceMgr,
		Logger:    logger,
	})

	_, err := orch.SendIncomingCallPush("nobody", "caller", "Caller", "session-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no device registered")
}

func TestOrchestratorSendIncomingCallPushDisabled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	deviceMgr := device.NewManager(logger)
	orch := NewOrchestrator(Config{
		DeviceMgr: deviceMgr,
		Logger:    logger,
	})

	// Register but then disable
	dev := &device.Device{
		ID:        "device-1",
		UserID:    "alice",
		Platform:  device.PlatformIOS,
		PushToken: "token",
	}
	deviceMgr.Register(dev)
	dev.Enabled = false

	_, err := orch.SendIncomingCallPush("alice", "caller", "Caller", "session-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestOrchestratorSendSilentPushNoDevice(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	deviceMgr := device.NewManager(logger)
	orch := NewOrchestrator(Config{
		DeviceMgr: deviceMgr,
		Logger:    logger,
	})

	err := orch.SendSilentPush("nobody", map[string]string{"type": "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no device registered")
}

func TestOrchestratorValidateDeviceTokenNoDevice(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	deviceMgr := device.NewManager(logger)
	orch := NewOrchestrator(Config{
		DeviceMgr: deviceMgr,
		Logger:    logger,
	})

	valid := orch.ValidateDeviceToken("nobody")
	assert.False(t, valid)
}

func TestOrchestratorGetStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	deviceMgr := device.NewManager(logger)
	orch := NewOrchestrator(Config{
		DeviceMgr: deviceMgr,
		Logger:    logger,
	})

	// Register some devices
	deviceMgr.Register(&device.Device{UserID: "alice", Platform: device.PlatformIOS, PushToken: "t1"})
	deviceMgr.Register(&device.Device{UserID: "bob", Platform: device.PlatformAndroid, PushToken: "t2"})

	stats := orch.GetStats()
	assert.Equal(t, 2, stats["total_devices"])

	counts, ok := stats["devices_by_platform"].(map[device.Platform]int)
	require.True(t, ok)
	assert.Equal(t, 1, counts[device.PlatformIOS])
	assert.Equal(t, 1, counts[device.PlatformAndroid])
}

func TestContains(t *testing.T) {
	assert.True(t, contains("hello world", "world"))
	assert.True(t, contains("test string", "test"))
	assert.False(t, contains("hello", "world"))
	assert.True(t, contains("same", "same"))
	assert.False(t, contains("a", "abc"))
}

func TestIsInvalidTokenError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	orch := NewOrchestrator(Config{Logger: logger})

	// Invalid token indicators
	assert.True(t, orch.isInvalidTokenError(fmt.Errorf("BadDeviceToken")))
	assert.True(t, orch.isInvalidTokenError(fmt.Errorf("some error with Unregistered token")))
	assert.False(t, orch.isInvalidTokenError(fmt.Errorf("network timeout")))
	assert.False(t, orch.isInvalidTokenError(nil))
}
