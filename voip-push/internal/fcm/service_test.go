package fcm

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := NewService(Config{
		ServerKey: "test-key",
		ProjectID: "test-project",
		Logger:    logger,
	})

	require.NotNil(t, service)
	assert.Equal(t, "test-key", service.serverKey)
	assert.Equal(t, "test-project", service.projectID)
	assert.NotNil(t, service.httpClient)
}

func TestServiceSendPushNotConfigured(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := NewService(Config{
		Logger: logger,
	})

	// Should fail because server key is not set
	err := service.SendPush("token", map[string]string{"type": "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestServiceValidateToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := NewService(Config{
		Logger: logger,
	})

	// Should fail because server key is not set
	valid := service.ValidateToken("some-token")
	assert.False(t, valid)
}

func TestMessageStruct(t *testing.T) {
	msg := Message{
		To:    "device-token",
		Data:  map[string]string{"type": "test"},
		Topic: "test-topic",
	}

	assert.Equal(t, "device-token", msg.To)
	assert.Equal(t, "test-topic", msg.Topic)
	assert.Equal(t, "test", msg.Data["type"])
}

func TestAndroidConfig(t *testing.T) {
	cfg := AndroidConfig{
		Priority: "high",
		TTL:      "0s",
		Data:     map[string]string{"key": "value"},
		Notification: &AndroidNotification{
			Title:     "Test",
			Body:      "Test body",
			ChannelID: "test-channel",
			Sound:     "default",
			Priority:  "high",
		},
	}

	assert.Equal(t, "high", cfg.Priority)
	assert.Equal(t, "0s", cfg.TTL)
	assert.Equal(t, "Test", cfg.Notification.Title)
}
