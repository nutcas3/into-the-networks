package tenant

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Tenant struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	Domain     string    `json:"domain" db:"domain"`
	Status     string    `json:"status" db:"status"` // active, suspended, deleted
	Config     Config    `json:"config" db:"config"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	LastActive time.Time `json:"last_active" db:"last_active"`
}

type Config struct {
	MaxConcurrentCalls int           `json:"max_concurrent_calls"`
	CallTimeout        time.Duration `json:"call_timeout"`
	AllowedCodecs      []string      `json:"allowed_codecs"`
	RecordingEnabled   bool          `json:"recording_enabled"`
	CDRRetention       time.Duration `json:"cdr_retention"`
	RateLimit          int           `json:"rate_limit"` // calls per minute
	Features           []string      `json:"features"`   // voicemail, conference, etc.
}

type Manager struct {
	tenants  map[uuid.UUID]*Tenant
	byName   map[string]*Tenant
	byDomain map[string]*Tenant
	mu       sync.RWMutex
	logger   *logrus.Logger
}

func NewManager() *Manager {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Manager{
		tenants:  make(map[uuid.UUID]*Tenant),
		byName:   make(map[string]*Tenant),
		byDomain: make(map[string]*Tenant),
		logger:   logger,
	}
}

func (m *Manager) CreateTenant(ctx context.Context, name, domain string, config Config) (*Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if tenant already exists
	if _, exists := m.byName[name]; exists {
		return nil, fmt.Errorf("tenant with name '%s' already exists", name)
	}
	if _, exists := m.byDomain[domain]; exists {
		return nil, fmt.Errorf("tenant with domain '%s' already exists", domain)
	}

	// Set default config if not provided
	if config.MaxConcurrentCalls == 0 {
		config.MaxConcurrentCalls = 100
	}
	if config.CallTimeout == 0 {
		config.CallTimeout = 5 * time.Minute
	}
	if config.AllowedCodecs == nil {
		config.AllowedCodecs = []string{"PCMU", "PCMA", "G729"}
	}
	if config.CDRRetention == 0 {
		config.CDRRetention = 90 * 24 * time.Hour // 90 days
	}
	if config.RateLimit == 0 {
		config.RateLimit = 1000
	}

	tenant := &Tenant{
		ID:         uuid.New(),
		Name:       name,
		Domain:     domain,
		Status:     "active",
		Config:     config,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	// Store in all lookup maps
	m.tenants[tenant.ID] = tenant
	m.byName[name] = tenant
	m.byDomain[domain] = tenant

	m.logger.WithFields(logrus.Fields{
		"tenant_id":   tenant.ID,
		"tenant_name": name,
		"domain":      domain,
	}).Info("Tenant created")

	return tenant, nil
}

func (m *Manager) GetTenant(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant, exists := m.tenants[id]
	if !exists {
		return nil, fmt.Errorf("tenant with ID '%s' not found", id)
	}

	if tenant.Status == "deleted" {
		return nil, fmt.Errorf("tenant '%s' is deleted", tenant.Name)
	}

	// Update last active time
	tenant.LastActive = time.Now()
	tenant.UpdatedAt = time.Now()

	return tenant, nil
}

func (m *Manager) GetTenantByName(ctx context.Context, name string) (*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant, exists := m.byName[name]
	if !exists {
		return nil, fmt.Errorf("tenant with name '%s' not found", name)
	}

	if tenant.Status == "deleted" {
		return nil, fmt.Errorf("tenant '%s' is deleted", tenant.Name)
	}

	// Update last active time
	tenant.LastActive = time.Now()
	tenant.UpdatedAt = time.Now()

	return tenant, nil
}

func (m *Manager) GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant, exists := m.byDomain[domain]
	if !exists {
		return nil, fmt.Errorf("tenant with domain '%s' not found", domain)
	}

	if tenant.Status == "deleted" {
		return nil, fmt.Errorf("tenant '%s' is deleted", tenant.Name)
	}

	// Update last active time
	tenant.LastActive = time.Now()
	tenant.UpdatedAt = time.Now()

	return tenant, nil
}

func (m *Manager) UpdateTenant(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, exists := m.tenants[id]
	if !exists {
		return fmt.Errorf("tenant with ID '%s' not found", id)
	}

	if tenant.Status == "deleted" {
		return fmt.Errorf("cannot update deleted tenant '%s'", tenant.Name)
	}

	// Apply updates
	for key, value := range updates {
		switch key {
		case "name":
			if newName, ok := value.(string); ok {
				// Remove from name lookup
				delete(m.byName, tenant.Name)
				tenant.Name = newName
				m.byName[newName] = tenant
			}
		case "domain":
			if newDomain, ok := value.(string); ok {
				// Remove from domain lookup
				delete(m.byDomain, tenant.Domain)
				tenant.Domain = newDomain
				m.byDomain[newDomain] = tenant
			}
		case "status":
			if status, ok := value.(string); ok {
				tenant.Status = status
			}
		case "config":
			if config, ok := value.(Config); ok {
				tenant.Config = config
			}
		}
	}

	tenant.UpdatedAt = time.Now()

	m.logger.WithFields(logrus.Fields{
		"tenant_id":   tenant.ID,
		"tenant_name": tenant.Name,
		"updates":     len(updates),
	}).Info("Tenant updated")

	return nil
}

func (m *Manager) SuspendTenant(ctx context.Context, id uuid.UUID) error {
	return m.UpdateTenant(ctx, id, map[string]any{"status": "suspended"})
}

func (m *Manager) DeleteTenant(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, exists := m.tenants[id]
	if !exists {
		return fmt.Errorf("tenant with ID '%s' not found", id)
	}

	// Soft delete - mark as deleted
	tenant.Status = "deleted"
	tenant.UpdatedAt = time.Now()

	m.logger.WithFields(logrus.Fields{
		"tenant_id":   tenant.ID,
		"tenant_name": tenant.Name,
	}).Info("Tenant deleted")

	return nil
}

func (m *Manager) ListTenants(ctx context.Context, status string) ([]*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tenants []*Tenant
	for _, tenant := range m.tenants {
		if status == "" || tenant.Status == status {
			tenants = append(tenants, tenant)
		}
	}

	return tenants, nil
}

func (m *Manager) GetActiveTenants(ctx context.Context) ([]*Tenant, error) {
	return m.ListTenants(ctx, "active")
}

func (m *Manager) ValidateTenantAccess(ctx context.Context, tenantID uuid.UUID, feature string) error {
	tenant, err := m.GetTenant(ctx, tenantID)
	if err != nil {
		return err
	}

	if tenant.Status != "active" {
		return fmt.Errorf("tenant '%s' is not active", tenant.Name)
	}

	// Check if feature is enabled for tenant
	if feature != "" {
		if slices.Contains(tenant.Config.Features, feature) {
			return nil
		}
		return fmt.Errorf("feature '%s' not enabled for tenant '%s'", feature, tenant.Name)
	}

	return nil
}

func (m *Manager) GetTenantStats(ctx context.Context) (map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]any)
	statusCounts := make(map[string]int)

	for _, tenant := range m.tenants {
		statusCounts[tenant.Status]++
	}

	stats["total_tenants"] = len(m.tenants)
	stats["status_counts"] = statusCounts
	stats["active_tenants"] = statusCounts["active"]
	stats["suspended_tenants"] = statusCounts["suspended"]
	stats["deleted_tenants"] = statusCounts["deleted"]

	return stats, nil
}

func (m *Manager) CleanupInactiveTenants(ctx context.Context, inactiveDuration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-inactiveDuration)
	cleaned := 0

	for _, tenant := range m.tenants {
		if tenant.Status == "active" && tenant.LastActive.Before(cutoff) {
			tenant.Status = "suspended"
			tenant.UpdatedAt = time.Now()
			cleaned++
		}
	}

	m.logger.WithField("cleaned_tenants", cleaned).Info("Inactive tenants cleaned up")
	return nil
}
