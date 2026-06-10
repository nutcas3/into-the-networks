package media

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	// This test requires RTPengine to be running
	// Skip if not available
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	_, err := NewManager(Config{
		RTPEngineAddress: "127.0.0.1:2223",
		Logger:           logger,
	})

	// Will fail if RTPengine is not running, which is expected in test environment
	if err != nil {
		t.Skipf("RTPengine not available: %v", err)
	}
}

func TestNewManagerInvalidAddress(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	_, err := NewManager(Config{
		RTPEngineAddress: "invalid-address",
		Logger:           logger,
	})

	assert.Error(t, err)
}

func TestNGClientFormatMessage(t *testing.T) {
	cookie := "test-cookie"
	req := Request{
		Command: "ping",
		CallID:  "test-call",
	}

	msg := formatMessage(cookie, req)
	assert.Contains(t, msg, cookie)
	assert.Contains(t, msg, "ping")
	assert.Contains(t, msg, "test-call")
}

func TestNGClientGenerateCookie(t *testing.T) {
	c1 := generateCookie()
	c2 := generateCookie()

	assert.NotEmpty(t, c1)
	assert.NotEmpty(t, c2)
	assert.NotEqual(t, c1, c2)
	assert.Equal(t, 16, len(c1)) // 8 bytes = 16 hex chars
}

func TestSplitN(t *testing.T) {
	// Test basic split
	parts := splitN([]byte("hello world test"), ' ', 3)
	require.Len(t, parts, 3)
	assert.Equal(t, []byte("hello"), parts[0])
	assert.Equal(t, []byte("world"), parts[1])
	assert.Equal(t, []byte("test"), parts[2])

	// Test with fewer parts than n
	parts = splitN([]byte("hello world"), ' ', 5)
	require.Len(t, parts, 2)
	assert.Equal(t, []byte("hello"), parts[0])
	assert.Equal(t, []byte("world"), parts[1])

	// Test no separator
	parts = splitN([]byte("hello"), ' ', 2)
	require.Len(t, parts, 1)
	assert.Equal(t, []byte("hello"), parts[0])
}
