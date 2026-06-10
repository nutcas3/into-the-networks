package webrtc

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPeerConnectionManager(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := Config{
		STUNServers: []string{"stun:stun.l.google.com:19302"},
		Logger:      logger,
	}

	manager, err := NewPeerConnectionManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.NotNil(t, manager.connections)
	assert.NotNil(t, manager.config)
}

func TestNewPeerConnectionManagerWithTURN(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := Config{
		STUNServers: []string{"stun:stun.l.google.com:19302"},
		TURNServers: []TURNConfig{
			{
				URL:      "turn:turn.example.com:3478",
				Username: "user",
				Password: "pass",
			},
		},
		Logger: logger,
	}

	manager, err := NewPeerConnectionManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.NotNil(t, manager.config)
}

func TestPeerConnectionManagerGetPeerConnection(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := Config{
		STUNServers: []string{"stun:stun.l.google.com:19302"},
		Logger:      logger,
	}

	manager, err := NewPeerConnectionManager(cfg)
	require.NoError(t, err)

	// Non-existent peer
	peer, exists := manager.GetPeerConnection("non-existent")
	assert.False(t, exists)
	assert.Nil(t, peer)
}

func TestPeerConnectionManagerClosePeerConnection(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := Config{
		STUNServers: []string{"stun:stun.l.google.com:19302"},
		Logger:      logger,
	}

	manager, err := NewPeerConnectionManager(cfg)
	require.NoError(t, err)

	// Create a peer connection
	peer, err := manager.CreatePeerConnection("user1", "session1")
	require.NoError(t, err)
	assert.NotNil(t, peer)

	// Verify it exists
	found, exists := manager.GetPeerConnection(peer.ID)
	assert.True(t, exists)
	assert.NotNil(t, found)

	// Close it
	err = manager.ClosePeerConnection(peer.ID)
	require.NoError(t, err)

	// Verify it's gone
	_, exists = manager.GetPeerConnection(peer.ID)
	assert.False(t, exists)
}

func TestPeerConnectionManagerCloseNonExistent(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := Config{
		STUNServers: []string{"stun:stun.l.google.com:19302"},
		Logger:      logger,
	}

	manager, err := NewPeerConnectionManager(cfg)
	require.NoError(t, err)

	err = manager.ClosePeerConnection("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
