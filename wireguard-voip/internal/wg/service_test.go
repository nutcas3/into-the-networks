package wg

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Name: "wg0", Logger: logger})
	require.NotNil(t, svc)
	assert.Equal(t, "wg0", svc.name)
	assert.False(t, svc.IsConfigured())
}

func TestNewServiceDefaultName(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	require.NotNil(t, svc)
	assert.Equal(t, "wg0", svc.name)
}

func TestGenerateKeyPair(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	require.NoError(t, err)
	assert.NotEmpty(t, priv)
	assert.NotEmpty(t, pub)
	assert.NotEqual(t, priv, pub)

	// Keys should be base64 encoded (roughly 44 chars for 32 bytes)
	assert.Greater(t, len(priv), 40)
	assert.Greater(t, len(pub), 40)
}

func TestConfigure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	cfg := &InterfaceConfig{
		PrivateKey: "private-key",
		ListenPort: 51820,
		Address:    "10.200.0.1/24",
		MTU:        1420,
		Peers:      []PeerConfig{},
	}

	err := svc.Configure(cfg)
	require.NoError(t, err)
	assert.True(t, svc.IsConfigured())

	retrieved := svc.GetConfig()
	assert.Equal(t, 51820, retrieved.ListenPort)
	assert.Equal(t, "10.200.0.1/24", retrieved.Address)
}

func TestAddPeer(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	svc.Configure(&InterfaceConfig{PrivateKey: "priv", ListenPort: 51820})

	peer := PeerConfig{
		PublicKey:           "pubkey123",
		AllowedIPs:          []string{"10.200.0.2/32"},
		Endpoint:            "192.168.1.100:51820",
		PersistentKeepalive: 25,
	}

	err := svc.AddPeer(peer)
	require.NoError(t, err)
	assert.Equal(t, 1, svc.GetPeerCount())
}

func TestAddPeerNotConfigured(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	err := svc.AddPeer(PeerConfig{PublicKey: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestRemovePeer(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	svc.Configure(&InterfaceConfig{PrivateKey: "priv"})

	svc.AddPeer(PeerConfig{PublicKey: "peer1", AllowedIPs: []string{"10.0.0.1/32"}})
	svc.AddPeer(PeerConfig{PublicKey: "peer2", AllowedIPs: []string{"10.0.0.2/32"}})
	assert.Equal(t, 2, svc.GetPeerCount())

	err := svc.RemovePeer("peer1")
	require.NoError(t, err)
	assert.Equal(t, 1, svc.GetPeerCount())
}

func TestRemovePeerNotConfigured(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	err := svc.RemovePeer("test")
	assert.Error(t, err)
}

func TestUpDown(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	assert.NoError(t, svc.Up())
	assert.NoError(t, svc.Down())
}

func TestParseAllowedIPs(t *testing.T) {
	nets, err := ParseAllowedIPs([]string{"10.0.0.0/24", "192.168.1.0/24"})
	require.NoError(t, err)
	assert.Len(t, nets, 2)

	_, err = ParseAllowedIPs([]string{"invalid"})
	assert.Error(t, err)
}

func TestAllocateIP(t *testing.T) {
	used := make(map[string]bool)

	ip1, err := AllocateIP("10.200.0.0/24", used)
	require.NoError(t, err)
	assert.Equal(t, "10.200.0.2/32", ip1)
	used[ip1] = true

	ip2, err := AllocateIP("10.200.0.0/24", used)
	require.NoError(t, err)
	assert.Equal(t, "10.200.0.3/32", ip2)
}

func TestAllocateIPInvalidSubnet(t *testing.T) {
	_, err := AllocateIP("invalid", nil)
	assert.Error(t, err)
}

func TestAllocateIPExhausted(t *testing.T) {
	used := make(map[string]bool)
	// Fill almost all IPs (leave one so first allocation succeeds then fills)
	for i := 2; i < 253; i++ {
		used[fmt.Sprintf("10.200.0.%d/32", i)] = true
	}
	// Take the last one
	ip, err := AllocateIP("10.200.0.0/24", used)
	require.NoError(t, err)
	used[ip] = true

	// Now should be exhausted
	_, err = AllocateIP("10.200.0.0/24", used)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no available IPs")
}
