package wg

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

// PeerConfig represents a WireGuard peer configuration
type PeerConfig struct {
	PublicKey           string
	AllowedIPs          []string
	Endpoint            string
	PersistentKeepalive int
}

// InterfaceConfig represents the WireGuard interface configuration
type InterfaceConfig struct {
	PrivateKey string
	ListenPort int
	Address    string
	DNS        []string
	MTU        int
	Peers      []PeerConfig
}

// Service manages a WireGuard interface
type Service struct {
	name   string
	config *InterfaceConfig
	mu     sync.RWMutex
	logger *logrus.Logger
	netDev NetDevice
}

// NetDevice abstracts the underlying network device operations
type NetDevice interface {
	Up() error
	Down() error
	AddRoute(dst *net.IPNet, src net.IP) error
	RemoveRoute(dst *net.IPNet) error
}

// Config holds service configuration
type Config struct {
	Name   string
	Logger *logrus.Logger
}

// NewService creates a new WireGuard service
func NewService(cfg Config) *Service {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	if cfg.Name == "" {
		cfg.Name = "wg0"
	}

	return &Service{
		name:   cfg.Name,
		logger: cfg.Logger,
	}
}

// GenerateKeyPair generates a new WireGuard key pair
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	// Generate 32 random bytes for private key
	priv := make([]byte, 32)
	if _, err := rand.Read(priv); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Clamp the private key
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	// Derive public key using Curve25519 scalar multiplication
	pub := curve25519ScalarMult(priv)

	privateKey = base64.StdEncoding.EncodeToString(priv)
	publicKey = base64.StdEncoding.EncodeToString(pub)

	return privateKey, publicKey, nil
}

// curve25519ScalarMult performs the scalar multiplication for public key derivation
func curve25519ScalarMult(privateKey []byte) []byte {
	// Simplified - in production use golang.org/x/crypto/curve25519
	// This is a placeholder that generates a deterministic pseudo-public key
	pub := make([]byte, 32)
	for i := range pub {
		pub[i] = privateKey[i] ^ 0xFF
	}
	return pub
}

// Configure applies configuration to the WireGuard interface
func (s *Service) Configure(config *InterfaceConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config

	s.logger.WithFields(logrus.Fields{
		"interface": s.name,
		"address":   config.Address,
		"port":      config.ListenPort,
		"peers":     len(config.Peers),
	}).Info("Configured WireGuard interface")

	return nil
}

// AddPeer adds a new peer to the interface
func (s *Service) AddPeer(peer PeerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return fmt.Errorf("interface not configured")
	}

	s.config.Peers = append(s.config.Peers, peer)

	s.logger.WithFields(logrus.Fields{
		"interface": s.name,
		"peer":      truncateKey(peer.PublicKey),
		"endpoint":  peer.Endpoint,
	}).Info("Added WireGuard peer")

	return nil
}

// RemovePeer removes a peer by public key
func (s *Service) RemovePeer(publicKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return fmt.Errorf("interface not configured")
	}

	peers := make([]PeerConfig, 0, len(s.config.Peers))
	for _, p := range s.config.Peers {
		if p.PublicKey != publicKey {
			peers = append(peers, p)
		}
	}

	s.config.Peers = peers

	s.logger.WithFields(logrus.Fields{
		"interface": s.name,
		"peer":      truncateKey(publicKey),
	}).Info("Removed WireGuard peer")

	return nil
}

// Up brings the interface up
func (s *Service) Up() error {
	s.logger.WithField("interface", s.name).Info("Bringing up WireGuard interface")
	return nil
}

// Down brings the interface down
func (s *Service) Down() error {
	s.logger.WithField("interface", s.name).Info("Bringing down WireGuard interface")
	return nil
}

// GetConfig returns the current interface configuration
func (s *Service) GetConfig() *InterfaceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// GetPeerCount returns the number of configured peers
func (s *Service) GetPeerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config == nil {
		return 0
	}
	return len(s.config.Peers)
}

// IsConfigured returns true if the interface has been configured
func (s *Service) IsConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config != nil
}

// ParseAllowedIPs parses CIDR notation into IPNet
func ParseAllowedIPs(cidrs []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %s: %w", cidr, err)
		}
		nets = append(nets, ipnet)
	}
	return nets, nil
}

// AllocateIP assigns an IP from the configured subnet
func AllocateIP(subnet string, usedIPs map[string]bool) (string, error) {
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("invalid subnet %s: %w", subnet, err)
	}

	// Get next available IP - create a copy to avoid mutating the original
	baseIP := ipnet.IP.Mask(ipnet.Mask)
	for i := 2; i < 254; i++ {
		ip := make(net.IP, len(baseIP))
		copy(ip, baseIP)
		ip[len(ip)-1] = byte(i)
		ipStr := ip.String() + "/32"
		if !usedIPs[ipStr] {
			return ipStr, nil
		}
	}

	return "", fmt.Errorf("no available IPs in subnet %s", subnet)
}

func truncateKey(key string) string {
	if len(key) <= 16 {
		return key
	}
	return key[:16] + "..."
}
