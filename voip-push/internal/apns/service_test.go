package apns

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewServiceNoConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Without bundle ID, APNS won't be initialized
	_, err := NewService(Config{
		Logger: logger,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "certificate path required")
}

func TestNewServiceWithJWT(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// JWT auth without auth key path
	_, err := NewService(Config{
		BundleID: "com.example.app",
		UseJWT:   true,
		KeyID:    "ABC123",
		TeamID:   "TEAM123",
		Logger:   logger,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auth key path")
}

func TestTruncateToken(t *testing.T) {
	short := "short"
	assert.Equal(t, "short", truncateToken(short))

	long := "this-is-a-very-long-device-token-string"
	result := truncateToken(long)
	assert.Equal(t, "this-is-a-very-long-...", result)
	assert.Equal(t, 23, len(result))
}
