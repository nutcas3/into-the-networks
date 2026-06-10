package tunnel

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Service manages the VPN tunnel for VoIP traffic
type Service struct {
	name        string
	localAddr   string
	remoteAddr  string
	mtu         int
	isUp        bool
	mu          sync.RWMutex
	logger      *logrus.Logger
	routes      []*net.IPNet
	voipSubnets []string
}

// Config holds tunnel configuration
type Config struct {
	Name        string
	LocalAddr   string
	RemoteAddr  string
	MTU         int
	VoIPSubnets []string
	Logger      *logrus.Logger
}

// NewService creates a new tunnel service
func NewService(cfg Config) *Service {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	if cfg.Name == "" {
		cfg.Name = "wg0"
	}
	if cfg.MTU == 0 {
		cfg.MTU = 1420
	}

	return &Service{
		name:        cfg.Name,
		localAddr:   cfg.LocalAddr,
		remoteAddr:  cfg.RemoteAddr,
		mtu:         cfg.MTU,
		logger:      cfg.Logger,
		routes:      make([]*net.IPNet, 0),
		voipSubnets: cfg.VoIPSubnets,
	}
}

// Up brings the tunnel up
func (s *Service) Up() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isUp {
		return nil
	}

	// Add VoIP-specific routes for zero-rating
	for _, subnet := range s.voipSubnets {
		_, ipnet, err := net.ParseCIDR(subnet)
		if err != nil {
			s.logger.WithError(err).WithField("subnet", subnet).Warn("Invalid VoIP subnet")
			continue
		}
		s.routes = append(s.routes, ipnet)
		s.logger.WithFields(logrus.Fields{
			"subnet": subnet,
			"interface": s.name,
		}).Info("Added VoIP route for zero-rating")
	}

	s.isUp = true
	s.logger.WithFields(logrus.Fields{
		"interface": s.name,
		"local":     s.localAddr,
		"routes":    len(s.routes),
	}).Info("Tunnel is up")

	return nil
}

// Down brings the tunnel down
func (s *Service) Down() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isUp {
		return nil
	}

	s.routes = s.routes[:0]
	s.isUp = false

	s.logger.WithField("interface", s.name).Info("Tunnel is down")

	return nil
}

// IsUp returns the tunnel status
func (s *Service) IsUp() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isUp
}

// GetRoutes returns configured routes
func (s *Service) GetRoutes() []*net.IPNet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routes := make([]*net.IPNet, len(s.routes))
	copy(routes, s.routes)
	return routes
}

// AddRoute adds a route to the tunnel
func (s *Service) AddRoute(subnet string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return fmt.Errorf("invalid subnet %s: %w", subnet, err)
	}

	s.routes = append(s.routes, ipnet)

	s.logger.WithFields(logrus.Fields{
		"subnet":    subnet,
		"interface": s.name,
	}).Info("Added route to tunnel")

	return nil
}

// GetStats returns tunnel statistics
func (s *Service) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"name":        s.name,
		"up":          s.isUp,
		"local_addr":  s.localAddr,
		"remote_addr": s.remoteAddr,
		"mtu":         s.mtu,
		"routes":      len(s.routes),
		"timestamp":   time.Now().Unix(),
	}
}

// defaultVoIPSubnets returns common VoIP-related subnets
func DefaultVoIPSubnets() []string {
	return []string{
		"0.0.0.0/0", // Route all traffic through VPN for full zero-rating
	}
}
