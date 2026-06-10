package zero

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Rater manages zero-rated traffic policies
type Rater struct {
	policies   map[string]*Policy
	carrierAPI string
	mu         sync.RWMutex
	logger     *logrus.Logger
	enabled    bool
}

// Policy defines a zero-rating policy
type Policy struct {
	Name        string
	Description string
	Subnets     []string
	Ports       []int
	Protocols   []string
	Enabled     bool
	CreatedAt   time.Time
}

// Config holds zero-rater configuration
type Config struct {
	CarrierAPI string
	Enabled    bool
	Logger     *logrus.Logger
}

// NewRater creates a new zero-rater
func NewRater(cfg Config) *Rater {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	r := &Rater{
		policies:   make(map[string]*Policy),
		carrierAPI: cfg.CarrierAPI,
		logger:     cfg.Logger,
		enabled:    cfg.Enabled,
	}

	// Add default VoIP zero-rating policy
	r.AddPolicy(&Policy{
		Name:        "voip-default",
		Description: "Zero-rated VoIP traffic",
		Subnets:     []string{"0.0.0.0/0"},
		Ports:       []int{5060, 5061, 10000, 20000},
		Protocols:   []string{"udp", "tcp"},
		Enabled:     true,
		CreatedAt:   time.Now(),
	})

	return r
}

// AddPolicy adds a new zero-rating policy
func (r *Rater) AddPolicy(policy *Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}

	r.policies[policy.Name] = policy

	r.logger.WithFields(logrus.Fields{
		"policy":  policy.Name,
		"subnets": len(policy.Subnets),
	}).Info("Added zero-rating policy")

	return nil
}

// RemovePolicy removes a policy by name
func (r *Rater) RemovePolicy(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.policies[name]; !exists {
		return fmt.Errorf("policy not found: %s", name)
	}

	delete(r.policies, name)
	r.logger.WithField("policy", name).Info("Removed zero-rating policy")

	return nil
}

// IsZeroRated checks if traffic to a given destination is zero-rated
func (r *Rater) IsZeroRated(dstIP string, port int, protocol string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.enabled {
		return false
	}

	ip := net.ParseIP(dstIP)
	if ip == nil {
		return false
	}

	for _, policy := range r.policies {
		if !policy.Enabled {
			continue
		}

		// Check subnet match
		subnetMatch := false
		for _, subnet := range policy.Subnets {
			if subnet == "0.0.0.0/0" {
				subnetMatch = true
				break
			}
			_, ipnet, err := net.ParseCIDR(subnet)
			if err != nil {
				continue
			}
			if ipnet.Contains(ip) {
				subnetMatch = true
				break
			}
		}
		if !subnetMatch {
			continue
		}

		// Check port match
		portMatch := false
		if len(policy.Ports) == 0 {
			portMatch = true
		} else {
			for _, p := range policy.Ports {
				if p == port {
					portMatch = true
					break
				}
			}
		}
		if !portMatch {
			continue
		}

		// Check protocol match
		protocolMatch := false
		if len(policy.Protocols) == 0 {
			protocolMatch = true
		} else {
			for _, proto := range policy.Protocols {
				if proto == protocol || proto == "any" {
					protocolMatch = true
					break
				}
			}
		}
		if !protocolMatch {
			continue
		}

		return true
	}

	return false
}

// GetPolicies returns all policies
func (r *Rater) GetPolicies() []*Policy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	policies := make([]*Policy, 0, len(r.policies))
	for _, p := range r.policies {
		policies = append(policies, p)
	}
	return policies
}

// Enable enables zero-rating
func (r *Rater) Enable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = true
	r.logger.Info("Zero-rating enabled")
}

// Disable disables zero-rating
func (r *Rater) Disable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = false
	r.logger.Info("Zero-rating disabled")
}

// IsEnabled returns zero-rating status
func (r *Rater) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled
}

// NotifyCarrier notifies the carrier API about zero-rated session start/end
func (r *Rater) NotifyCarrier(userID, sessionID string, start bool) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.carrierAPI == "" {
		return nil // No carrier integration configured
	}

	event := "session_end"
	if start {
		event = "session_start"
	}

	r.logger.WithFields(logrus.Fields{
		"user_id":    userID,
		"session_id": sessionID,
		"event":      event,
	}).Info("Notified carrier about zero-rated session")

	// In production, this would make an HTTP call to the carrier API
	// e.g., POST carrierAPI/zero-rating {user_id, session_id, event}

	return nil
}
