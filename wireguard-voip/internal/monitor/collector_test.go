package monitor

import (
	"testing"
	"time"

	"github.com/nutcas3/wireguard-voip/internal/peer"
	"github.com/nutcas3/wireguard-voip/internal/tunnel"
	"github.com/nutcas3/wireguard-voip/internal/wg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollector(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	c := NewCollector(Config{
		Logger:   logger,
		Interval: 100 * time.Millisecond,
	})
	require.NotNil(t, c)
	defer c.Close()

	// Wait for first collection
	time.Sleep(150 * time.Millisecond)
	metrics := c.GetMetrics()
	assert.NotNil(t, metrics)
	assert.NotZero(t, metrics.Timestamp)
}

func TestCollectorWithServices(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	wgSvc := wg.NewService(wg.Config{Logger: logger})
	wgSvc.Configure(&wg.InterfaceConfig{PrivateKey: "priv", ListenPort: 51820})

	peerMgr, _ := peer.NewManager(peer.Config{Subnet: "10.200.0.0/24", Logger: logger})
	tunnelSvc := tunnel.NewService(tunnel.Config{Logger: logger})
	tunnelSvc.Up()

	// Provision some peers
	p1, _ := peerMgr.Provision("alice", "iPhone")
	p2, _ := peerMgr.Provision("bob", "Android")

	// Mark one connected
	peerMgr.UpdateStats(p1.PublicKey, 1024, 2048, time.Now())
	peerMgr.UpdateStats(p2.PublicKey, 512, 512, time.Now().Add(-10*time.Minute))

	c := NewCollector(Config{
		WGService: wgSvc,
		PeerMgr:   peerMgr,
		TunnelSvc: tunnelSvc,
		Logger:    logger,
		Interval:  100 * time.Millisecond,
	})
	defer c.Close()

	// Wait for collection
	time.Sleep(150 * time.Millisecond)

	metrics := c.GetMetrics()
	assert.True(t, metrics.TunnelUp)
	assert.Equal(t, 2, metrics.PeerCount)
	assert.Equal(t, 1, metrics.ConnectedPeers)
	assert.Equal(t, uint64(1536), metrics.TotalRx)
	assert.Equal(t, uint64(2560), metrics.TotalTx)
	assert.Len(t, metrics.PeerDetails, 2)
}

func TestCheckHealth(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tunnelSvc := tunnel.NewService(tunnel.Config{Logger: logger})
	tunnelSvc.Up()

	c := NewCollector(Config{
		TunnelSvc: tunnelSvc,
		Logger:    logger,
		Interval:  1 * time.Hour, // Don't run auto-collection
	})
	defer c.Close()

	// Manually collect
	c.collect()

	health := c.CheckHealth()
	assert.Equal(t, "healthy", health["status"])
	assert.True(t, health["tunnel_up"].(bool))
	assert.NotZero(t, health["timestamp"])
}

func TestCheckHealthDegraded(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	c := NewCollector(Config{
		Logger:    logger,
		Interval:  1 * time.Hour,
	})
	defer c.Close()

	c.collect()

	health := c.CheckHealth()
	assert.Equal(t, "degraded", health["status"])
	assert.False(t, health["tunnel_up"].(bool))
}
