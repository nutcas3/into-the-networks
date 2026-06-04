package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	_ "github.com/lib/pq"
)

type Repository struct {
	db     *sql.DB
	logger *logrus.Logger
}

func NewRepository(db *sql.DB) *Repository {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Repository{
		db:     db,
		logger: logger,
	}
}

func (r *Repository) Initialize(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255) UNIQUE NOT NULL,
			domain VARCHAR(255) UNIQUE NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			config JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tenants_name ON tenants(name)`,
		`CREATE INDEX IF NOT EXISTS idx_tenants_domain ON tenants(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tenants_last_active ON tenants(last_active)`,
		`CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ language 'plpgsql'`,
		`CREATE TRIGGER update_tenants_updated_at 
			BEFORE UPDATE ON tenants 
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
	}

	for _, query := range queries {
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			r.logger.WithError(err).Error("Failed to initialize tenant tables")
			return fmt.Errorf("failed to initialize tenant tables: %w", err)
		}
	}

	r.logger.Info("Tenant tables initialized successfully")
	return nil
}

func (r *Repository) Create(ctx context.Context, tenant *Tenant) error {
	configJSON, err := json.Marshal(tenant.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal tenant config: %w", err)
	}

	query := `
		INSERT INTO tenants (id, name, domain, status, config, created_at, updated_at, last_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (name) DO NOTHING
		RETURNING id
	`

	err = r.db.QueryRowContext(ctx, query,
		tenant.ID, tenant.Name, tenant.Domain, tenant.Status,
		configJSON, tenant.CreatedAt, tenant.UpdatedAt, tenant.LastActive,
	).Scan(&tenant.ID)

	if err != nil {
		r.logger.WithError(err).Error("Failed to create tenant")
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id":   tenant.ID,
		"tenant_name": tenant.Name,
	}).Info("Tenant created in database")

	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	query := `
		SELECT id, name, domain, status, config, created_at, updated_at, last_active
		FROM tenants
		WHERE id = $1
	`

	var tenant Tenant
	var configJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tenant.ID, &tenant.Name, &tenant.Domain, &tenant.Status,
		&configJSON, &tenant.CreatedAt, &tenant.UpdatedAt, &tenant.LastActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant with ID '%s' not found", id)
		}
		r.logger.WithError(err).Error("Failed to get tenant by ID")
		return nil, fmt.Errorf("failed to get tenant by ID: %w", err)
	}

	if err := json.Unmarshal(configJSON, &tenant.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tenant config: %w", err)
	}

	return &tenant, nil
}

func (r *Repository) GetByName(ctx context.Context, name string) (*Tenant, error) {
	query := `
		SELECT id, name, domain, status, config, created_at, updated_at, last_active
		FROM tenants
		WHERE name = $1
	`

	var tenant Tenant
	var configJSON []byte

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&tenant.ID, &tenant.Name, &tenant.Domain, &tenant.Status,
		&configJSON, &tenant.CreatedAt, &tenant.UpdatedAt, &tenant.LastActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant with name '%s' not found", name)
		}
		r.logger.WithError(err).Error("Failed to get tenant by name")
		return nil, fmt.Errorf("failed to get tenant by name: %w", err)
	}

	if err := json.Unmarshal(configJSON, &tenant.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tenant config: %w", err)
	}

	return &tenant, nil
}

func (r *Repository) GetByDomain(ctx context.Context, domain string) (*Tenant, error) {
	query := `
		SELECT id, name, domain, status, config, created_at, updated_at, last_active
		FROM tenants
		WHERE domain = $1
	`

	var tenant Tenant
	var configJSON []byte

	err := r.db.QueryRowContext(ctx, query, domain).Scan(
		&tenant.ID, &tenant.Name, &tenant.Domain, &tenant.Status,
		&configJSON, &tenant.CreatedAt, &tenant.UpdatedAt, &tenant.LastActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant with domain '%s' not found", domain)
		}
		r.logger.WithError(err).Error("Failed to get tenant by domain")
		return nil, fmt.Errorf("failed to get tenant by domain: %w", err)
	}

	if err := json.Unmarshal(configJSON, &tenant.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tenant config: %w", err)
	}

	return &tenant, nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	setClause := ""
	args := []any{}
	argIndex := 1

	for key, value := range updates {
		if setClause != "" {
			setClause += ", "
		}

		switch key {
		case "name":
			setClause += fmt.Sprintf("name = $%d", argIndex)
			args = append(args, value)
			argIndex++
		case "domain":
			setClause += fmt.Sprintf("domain = $%d", argIndex)
			args = append(args, value)
			argIndex++
		case "status":
			setClause += fmt.Sprintf("status = $%d", argIndex)
			args = append(args, value)
			argIndex++
		case "config":
			setClause += fmt.Sprintf("config = $%d", argIndex)
			configJSON, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}
			args = append(args, configJSON)
			argIndex++
		case "last_active":
			setClause += fmt.Sprintf("last_active = $%d", argIndex)
			args = append(args, value)
			argIndex++
		}
	}

	if setClause == "" {
		return fmt.Errorf("no valid updates provided")
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE tenants SET %s WHERE id = $%d", setClause, argIndex)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update tenant")
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id": id,
		"updates":   len(updates),
	}).Info("Tenant updated in database")

	return nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tenants WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.WithError(err).Error("Failed to delete tenant")
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tenant with ID '%s' not found", id)
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id": id,
	}).Info("Tenant deleted from database")

	return nil
}

func (r *Repository) List(ctx context.Context, status string, limit, offset int) ([]*Tenant, error) {
	query := `
		SELECT id, name, domain, status, config, created_at, updated_at, last_active
		FROM tenants
	`
	args := []any{}

	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.WithError(err).Error("Failed to list tenants")
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*Tenant
	for rows.Next() {
		var tenant Tenant
		var configJSON []byte

		err := rows.Scan(
			&tenant.ID, &tenant.Name, &tenant.Domain, &tenant.Status,
			&configJSON, &tenant.CreatedAt, &tenant.UpdatedAt, &tenant.LastActive,
		)
		if err != nil {
			r.logger.WithError(err).Error("Failed to scan tenant row")
			continue
		}

		if err := json.Unmarshal(configJSON, &tenant.Config); err != nil {
			r.logger.WithError(err).Error("Failed to unmarshal tenant config")
			continue
		}

		tenants = append(tenants, &tenant)
	}

	return tenants, nil
}

func (r *Repository) GetStats(ctx context.Context) (map[string]any, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN status = 'active' THEN 1 END) as active,
			COUNT(CASE WHEN status = 'suspended' THEN 1 END) as suspended,
			COUNT(CASE WHEN status = 'deleted' THEN 1 END) as deleted,
			COUNT(CASE WHEN last_active > NOW() - INTERVAL '24 hours' THEN 1 END) as active_last_24h
		FROM tenants
	`

	var total, active, suspended, deleted, activeLast24h int64
	err := r.db.QueryRowContext(ctx, query).Scan(&total, &active, &suspended, &deleted, &activeLast24h)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get tenant stats")
		return nil, fmt.Errorf("failed to get tenant stats: %w", err)
	}

	stats := map[string]any{
		"total_tenants":       total,
		"active_tenants":      active,
		"suspended_tenants":   suspended,
		"deleted_tenants":     deleted,
		"active_last_24h":     activeLast24h,
		"active_percentage":   float64(active) / float64(total) * 100,
		"suspended_percentage": float64(suspended) / float64(total) * 100,
	}

	return stats, nil
}

func (r *Repository) UpdateLastActive(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tenants SET last_active = CURRENT_TIMESTAMP WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update last active")
		return fmt.Errorf("failed to update last active: %w", err)
	}

	return nil
}

func (r *Repository) CleanupInactive(ctx context.Context, inactiveDuration time.Duration) error {
	query := `
		UPDATE tenants 
		SET status = 'suspended', updated_at = CURRENT_TIMESTAMP
		WHERE status = 'active' 
		AND last_active < $1
	`

	cutoff := time.Now().Add(-inactiveDuration)
	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		r.logger.WithError(err).Error("Failed to cleanup inactive tenants")
		return fmt.Errorf("failed to cleanup inactive tenants: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"cleaned_tenants": rowsAffected,
		"cutoff_time":     cutoff,
	}).Info("Inactive tenants cleaned up")

	return nil
}
