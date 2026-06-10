package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/nutcas3/wireguard-voip/internal/peer"
	"github.com/nutcas3/wireguard-voip/internal/tunnel"
	"github.com/nutcas3/wireguard-voip/internal/wg"
	"github.com/sirupsen/logrus"
)

// Collector gathers and reports tunnel health metrics
type Collector struct {
	wgService *wg.Service
	peerMgr   *peer.Manager
	tunnelSvc *tunnel.Service
	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	interval  time.Duration
	mu        sync.RWMutex
	metrics   *Metrics
}

// Metrics holds collected health metrics
type Metrics struct {
	Timestamp      int64        `json:"timestamp"`
	TunnelUp       bool         `json:"tunnel_up"`
	PeerCount      int          `json:"peer_count"`
	ConnectedPeers int          `json:"connected_peers"`
	TotalRx        uint64       `json:"total_rx"`
	TotalTx        uint64       `json:"total_tx"`
	PeerDetails    []PeerMetric `json:"peer_details,omitempty"`
}

// PeerMetric holds per-peer metrics
type PeerMetric struct {
	PublicKey     string `json:"public_key"`
	UserID        string `json:"user_id"`
	IP            string `json:"ip"`
	Connected     bool   `json:"connected"`
	RxBytes       uint64 `json:"rx_bytes"`
	TxBytes       uint64 `json:"tx_bytes"`
	LastHandshake int64  `json:"last_handshake"`
}

// Config holds collector configuration
type Config struct {
	WGService *wg.Service
	PeerMgr   *peer.Manager
	TunnelSvc *tunnel.Service
	Interval  time.Duration
	Logger    *logrus.Logger
}

// NewCollector creates a new metrics collector
func NewCollector(cfg Config) *Collector {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Collector{
		wgService: cfg.WGService,
		peerMgr:   cfg.PeerMgr,
		tunnelSvc: cfg.TunnelSvc,
		logger:    cfg.Logger,
		ctx:       ctx,
		cancel:    cancel,
		interval:  cfg.Interval,
		metrics:   &Metrics{},
	}

	go c.run()

	return c
}

// Close stops the collector
func (c *Collector) Close() {
	c.cancel()
}

// GetMetrics returns current metrics
func (c *Collector) GetMetrics() *Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := *c.metrics
	return &m
}

func (c *Collector) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Collector) collect() {
	metrics := &Metrics{
		Timestamp: time.Now().Unix(),
	}

	// Tunnel status
	if c.tunnelSvc != nil {
		metrics.TunnelUp = c.tunnelSvc.IsUp()
	}

	// Peer metrics
	if c.peerMgr != nil {
		metrics.PeerCount = c.peerMgr.Count()
		metrics.ConnectedPeers = c.peerMgr.CountConnected()

		peers := c.peerMgr.GetAll()
		metrics.PeerDetails = make([]PeerMetric, 0, len(peers))
		for _, p := range peers {
			metrics.TotalRx += p.RxBytes
			metrics.TotalTx += p.TxBytes

			pm := PeerMetric{
				PublicKey: p.PublicKey[:16] + "...",
				UserID:    p.UserID,
				IP:        p.IP,
				Connected: p.Connected,
				RxBytes:   p.RxBytes,
				TxBytes:   p.TxBytes,
			}
			if !p.LastHandshake.IsZero() {
				pm.LastHandshake = p.LastHandshake.Unix()
			}
			metrics.PeerDetails = append(metrics.PeerDetails, pm)
		}
	}

	c.mu.Lock()
	c.metrics = metrics
	c.mu.Unlock()

	// Log health status
	if metrics.TunnelUp {
		c.logger.WithFields(logrus.Fields{
			"peers":     metrics.PeerCount,
			"connected": metrics.ConnectedPeers,
			"rx":        metrics.TotalRx,
			"tx":        metrics.TotalTx,
		}).Debug("Tunnel health check")
	} else {
		c.logger.Warn("Tunnel is down")
	}
}

// CheckHealth performs a quick health check
func (c *Collector) CheckHealth() map[string]interface{} {
	metrics := c.GetMetrics()

	status := "healthy"
	if !metrics.TunnelUp {
		status = "degraded"
	}

	return map[string]interface{}{
		"status":          status,
		"tunnel_up":       metrics.TunnelUp,
		"peer_count":      metrics.PeerCount,
		"connected_peers": metrics.ConnectedPeers,
		"timestamp":       metrics.Timestamp,
	}
}
