package peer

import (
	"testing"
	"time"

	"github.com/nutcas3/wireguard-voip/internal/wg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{
		Subnet: "10.200.0.0/24",
		Logger: logger,
	})
	require.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.Equal(t, 0, mgr.Count())
}

func TestNewManagerDefaultSubnet(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{Logger: logger})
	require.NoError(t, err)
	assert.NotNil(t, mgr)
}

func TestProvision(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{
		Subnet: "10.200.0.0/24",
		Logger: logger,
	})
	require.NoError(t, err)

	peer, err := mgr.Provision("alice", "iPhone 14")
	require.NoError(t, err)
	assert.NotEmpty(t, peer.ID)
	assert.NotEmpty(t, peer.PublicKey)
	assert.NotEmpty(t, peer.PrivateKey)
	assert.Equal(t, "alice", peer.UserID)
	assert.Equal(t, "iPhone 14", peer.DeviceInfo)
	assert.False(t, peer.Connected)
	assert.NotEmpty(t, peer.IP)
}

func TestProvisionDuplicateUser(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{
		Subnet: "10.200.0.0/24",
		Logger: logger,
	})
	require.NoError(t, err)

	p1, err := mgr.Provision("alice", "device1")
	require.NoError(t, err)

	p2, err := mgr.Provision("alice", "device2")
	require.NoError(t, err)

	// Should return existing peer
	assert.Equal(t, p1.ID, p2.ID)
}

func TestRevoke(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{
		Subnet: "10.200.0.0/24",
		Logger: logger,
	})
	require.NoError(t, err)

	peer, _ := mgr.Provision("alice", "device")
	assert.Equal(t, 1, mgr.Count())

	err = mgr.Revoke(peer.PublicKey)
	require.NoError(t, err)
	assert.Equal(t, 0, mgr.Count())

	// Should be gone
	_, exists := mgr.GetByPublicKey(peer.PublicKey)
	assert.False(t, exists)
}

func TestRevokeNotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{
		Subnet: "10.200.0.0/24",
		Logger: logger,
	})
	require.NoError(t, err)

	err = mgr.Revoke("invalid-key")
	assert.Error(t, err)
}

func TestGetByUserID(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{
		Subnet: "10.200.0.0/24",
		Logger: logger,
	})
	require.NoError(t, err)

	mgr.Provision("alice", "device")

	peer, exists := mgr.GetByUserID("alice")
	assert.True(t, exists)
	assert.Equal(t, "alice", peer.UserID)

	_, exists = mgr.GetByUserID("bob")
	assert.False(t, exists)
}

func TestUpdateStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{
		Subnet: "10.200.0.0/24",
		Logger: logger,
	})
	require.NoError(t, err)

	peer, _ := mgr.Provision("alice", "device")

	handshake := time.Now().Add(-1 * time.Minute)
	err = mgr.UpdateStats(peer.PublicKey, 1000, 2000, handshake)
	require.NoError(t, err)

	updated, _ := mgr.GetByPublicKey(peer.PublicKey)
	assert.Equal(t, uint64(1000), updated.RxBytes)
	assert.Equal(t, uint64(2000), updated.TxBytes)
	assert.True(t, updated.Connected)
}

func TestCountConnected(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mgr, err := NewManager(Config{
		Subnet: "10.200.0.0/24",
		Logger: logger,
	})
	require.NoError(t, err)

	p1, _ := mgr.Provision("alice", "device1")
	_, _ = mgr.Provision("bob", "device2")

	// Mark one as connected
	mgr.UpdateStats(p1.PublicKey, 0, 0, time.Now())

	assert.Equal(t, 2, mgr.Count())
	assert.Equal(t, 1, mgr.CountConnected())
}

func TestProvisionWithWGService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	wgSvc := wg.NewService(wg.Config{Name: "wg0", Logger: logger})
	wgSvc.Configure(&wg.InterfaceConfig{
		PrivateKey: "priv",
		ListenPort: 51820,
	})

	mgr, err := NewManager(Config{
		Subnet:    "10.200.0.0/24",
		Logger:    logger,
		WGService: wgSvc,
	})
	require.NoError(t, err)

	peer, err := mgr.Provision("alice", "device")
	require.NoError(t, err)
	assert.NotNil(t, peer)
}
