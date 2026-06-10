package peer

import (
	"fmt"
	"sync"
	"time"

	"github.com/nutcas3/wireguard-voip/internal/wg"
	"github.com/sirupsen/logrus"
)

// Peer represents a WireGuard peer with metadata
type Peer struct {
	ID            string
	PublicKey     string
	PrivateKey    string
	AllowedIPs    []string
	Endpoint      string
	IP            string
	UserID        string
	DeviceInfo    string
	Connected     bool
	LastHandshake time.Time
	RxBytes       uint64
	TxBytes       uint64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Manager handles peer lifecycle and provisioning
type Manager struct {
	peers     map[string]*Peer  // key: publicKey
	userPeers map[string]string // key: userID, value: publicKey
	ipPool    map[string]bool   // tracks allocated IPs
	subnet    string
	mu        sync.RWMutex
	logger    *logrus.Logger
	wgService *wg.Service
}

// Config holds peer manager configuration
type Config struct {
	Subnet    string
	Logger    *logrus.Logger
	WGService *wg.Service
}

// NewManager creates a new peer manager
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	if cfg.Subnet == "" {
		cfg.Subnet = "10.200.0.0/24"
	}

	return &Manager{
		peers:     make(map[string]*Peer),
		userPeers: make(map[string]string),
		ipPool:    make(map[string]bool),
		subnet:    cfg.Subnet,
		logger:    cfg.Logger,
		wgService: cfg.WGService,
	}, nil
}

// Provision creates a new peer for a user
func (m *Manager) Provision(userID, deviceInfo string) (*Peer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if user already has a peer
	if existingKey, exists := m.userPeers[userID]; exists {
		if peer, ok := m.peers[existingKey]; ok {
			m.logger.WithField("user_id", userID).Info("User already has a peer, returning existing")
			peer.UpdatedAt = time.Now()
			return peer, nil
		}
	}

	// Generate key pair
	privKey, pubKey, err := wg.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	// Allocate IP
	ip, err := wg.AllocateIP(m.subnet, m.ipPool)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}
	m.ipPool[ip] = true

	peer := &Peer{
		ID:         fmt.Sprintf("peer-%d", time.Now().UnixNano()),
		PublicKey:  pubKey,
		PrivateKey: privKey,
		AllowedIPs: []string{ip},
		IP:         ip,
		UserID:     userID,
		DeviceInfo: deviceInfo,
		Connected:  false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.peers[pubKey] = peer
	m.userPeers[userID] = pubKey

	// Add to WireGuard interface
	if m.wgService != nil && m.wgService.IsConfigured() {
		wgPeer := wg.PeerConfig{
			PublicKey:           pubKey,
			AllowedIPs:          []string{ip},
			PersistentKeepalive: 25,
		}
		if err := m.wgService.AddPeer(wgPeer); err != nil {
			m.logger.WithError(err).Warn("Failed to add peer to WireGuard")
		}
	}

	m.logger.WithFields(logrus.Fields{
		"user_id":    userID,
		"peer_id":    peer.ID,
		"ip":         ip,
		"public_key": truncateKey(pubKey),
	}).Info("Provisioned new peer")

	return peer, nil
}

// Revoke removes a peer permanently
func (m *Manager) Revoke(publicKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	peer, exists := m.peers[publicKey]
	if !exists {
		return fmt.Errorf("peer not found: %s", truncateKey(publicKey))
	}

	// Release IP
	delete(m.ipPool, peer.IP)
	delete(m.userPeers, peer.UserID)
	delete(m.peers, publicKey)

	// Remove from WireGuard
	if m.wgService != nil {
		if err := m.wgService.RemovePeer(publicKey); err != nil {
			m.logger.WithError(err).Warn("Failed to remove peer from WireGuard")
		}
	}

	m.logger.WithFields(logrus.Fields{
		"user_id": peer.UserID,
		"peer_id": peer.ID,
	}).Info("Revoked peer")

	return nil
}

// GetByPublicKey retrieves a peer by public key
func (m *Manager) GetByPublicKey(publicKey string) (*Peer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer, exists := m.peers[publicKey]
	return peer, exists
}

// GetByUserID retrieves a peer by user ID
func (m *Manager) GetByUserID(userID string) (*Peer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pubKey, exists := m.userPeers[userID]
	if !exists {
		return nil, false
	}

	peer, exists := m.peers[pubKey]
	return peer, exists
}

// GetAll returns all peers
func (m *Manager) GetAll() []*Peer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]*Peer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	return peers
}

// UpdateStats updates peer connection statistics
func (m *Manager) UpdateStats(publicKey string, rx, tx uint64, handshake time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	peer, exists := m.peers[publicKey]
	if !exists {
		return fmt.Errorf("peer not found")
	}

	peer.RxBytes = rx
	peer.TxBytes = tx
	peer.LastHandshake = handshake
	peer.Connected = !handshake.IsZero() && time.Since(handshake) < 3*time.Minute
	peer.UpdatedAt = time.Now()

	return nil
}

// Count returns the number of peers
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.peers)
}

// CountConnected returns the number of connected peers
func (m *Manager) CountConnected() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, p := range m.peers {
		if p.Connected {
			count++
		}
	}
	return count
}

func truncateKey(key string) string {
	if len(key) <= 16 {
		return key
	}
	return key[:16] + "..."
}
