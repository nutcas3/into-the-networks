package cdr

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nutcas3/esl-resilience/internal/tenant"
	"github.com/sirupsen/logrus"
)

type TenantRepository struct {
	db          *sql.DB
	tenantMgr   *tenant.Manager
	logger      *logrus.Logger
}

func NewTenantRepository(db *sql.DB, tenantMgr *tenant.Manager) *TenantRepository {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &TenantRepository{
		db:        db,
		tenantMgr: tenantMgr,
		logger:    logger,
	}
}

// Initialize tenant-specific CDR tables
func (r *TenantRepository) Initialize(ctx context.Context) error {
	// Create tenant-specific CDR tables with tenant_id segregation
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tenant_cdr (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			tenant_id UUID NOT NULL REFERENCES tenants(id),
			account_code VARCHAR(255),
			caller_id_name VARCHAR(255),
			caller_id_number VARCHAR(255),
			destination_number VARCHAR(255),
			start_timestamp TIMESTAMP NOT NULL,
			answer_timestamp TIMESTAMP,
			end_timestamp TIMESTAMP,
			duration_seconds INTEGER,
			billsec_seconds INTEGER,
			hangup_cause VARCHAR(64),
			channel_uuid UUID NOT NULL,
			destination_channel_uuid UUID,
			context VARCHAR(64),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(channel_uuid)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_cdr_tenant_id ON tenant_cdr(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_cdr_start_timestamp ON tenant_cdr(start_timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_cdr_caller_id_number ON tenant_cdr(caller_id_number)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_cdr_destination_number ON tenant_cdr(destination_number)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_cdr_channel_uuid ON tenant_cdr(channel_uuid)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_cdr_tenant_start ON tenant_cdr(tenant_id, start_timestamp)`,
		`CREATE TABLE IF NOT EXISTS tenant_call_metadata (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			tenant_id UUID NOT NULL REFERENCES tenants(id),
			cdr_id UUID NOT NULL REFERENCES tenant_cdr(id) ON DELETE CASCADE,
			key VARCHAR(128),
			value TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(cdr_id, key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_call_metadata_tenant_id ON tenant_call_metadata(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_call_metadata_cdr_id ON tenant_call_metadata(cdr_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_call_metadata_key ON tenant_call_metadata(key)`,
	}

	for _, query := range queries {
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			r.logger.WithError(err).Error("Failed to initialize tenant CDR tables")
			return fmt.Errorf("failed to initialize tenant CDR tables: %w", err)
		}
	}

	r.logger.Info("Tenant CDR tables initialized successfully")
	return nil
}

type TenantCallDetailRecord struct {
	ID                    uuid.UUID      `db:"id"`
	TenantID              uuid.UUID      `db:"tenant_id"`
	AccountCode           string         `db:"account_code"`
	CallerIDName          string         `db:"caller_id_name"`
	CallerIDNumber        string         `db:"caller_id_number"`
	DestinationNumber     string         `db:"destination_number"`
	StartTimestamp        time.Time      `db:"start_timestamp"`
	AnswerTimestamp       sql.NullTime   `db:"answer_timestamp"`
	EndTimestamp          sql.NullTime   `db:"end_timestamp"`
	DurationSeconds       sql.NullInt64  `db:"duration_seconds"`
	BillsecSeconds        sql.NullInt64  `db:"billsec_seconds"`
	HangupCause           string         `db:"hangup_cause"`
	ChannelUUID           uuid.UUID      `db:"channel_uuid"`
	DestinationChannelUUID sql.NullString `db:"destination_channel_uuid"`
	Context               string         `db:"context"`
	CreatedAt             time.Time      `db:"created_at"`
}

func (r *TenantRepository) CreateCDR(ctx context.Context, tenantID uuid.UUID, cdr *TenantCallDetailRecord) error {
	// Validate tenant exists and is active
	if err := r.tenantMgr.ValidateTenantAccess(ctx, tenantID, ""); err != nil {
		return fmt.Errorf("tenant access validation failed: %w", err)
	}

	cdr.TenantID = tenantID

	query := `
		INSERT INTO tenant_cdr (
			id, tenant_id, account_code, caller_id_name, caller_id_number, destination_number,
			start_timestamp, answer_timestamp, end_timestamp, duration_seconds,
			billsec_seconds, hangup_cause, channel_uuid, destination_channel_uuid,
			context, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		) ON CONFLICT (channel_uuid) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			answer_timestamp = EXCLUDED.answer_timestamp,
			end_timestamp = EXCLUDED.end_timestamp,
			duration_seconds = EXCLUDED.duration_seconds,
			billsec_seconds = EXCLUDED.billsec_seconds,
			hangup_cause = EXCLUDED.hangup_cause,
			destination_channel_uuid = EXCLUDED.destination_channel_uuid
	`

	_, err := r.db.ExecContext(ctx, query,
		cdr.ID, cdr.TenantID, cdr.AccountCode, cdr.CallerIDName, cdr.CallerIDNumber, cdr.DestinationNumber,
		cdr.StartTimestamp, cdr.AnswerTimestamp, cdr.EndTimestamp, cdr.DurationSeconds,
		cdr.BillsecSeconds, cdr.HangupCause, cdr.ChannelUUID, cdr.DestinationChannelUUID,
		cdr.Context, cdr.CreatedAt,
	)

	if err != nil {
		r.logger.WithError(err).Error("Failed to create tenant CDR")
		return fmt.Errorf("failed to create tenant CDR: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id":    cdr.TenantID,
		"cdr_id":       cdr.ID,
		"caller":       cdr.CallerIDNumber,
		"destination":  cdr.DestinationNumber,
		"channel_uuid": cdr.ChannelUUID,
	}).Info("Tenant CDR created/updated")

	return nil
}

func (r *TenantRepository) UpdateCDR(ctx context.Context, tenantID, channelUUID uuid.UUID, updates map[string]any) error {
	// Validate tenant exists and is active
	if err := r.tenantMgr.ValidateTenantAccess(ctx, tenantID, ""); err != nil {
		return fmt.Errorf("tenant access validation failed: %w", err)
	}

	setClause := ""
	args := []any{}
	argIndex := 1

	for key, value := range updates {
		if setClause != "" {
			setClause += ", "
		}
		setClause += fmt.Sprintf("%s = $%d", key, argIndex)
		args = append(args, value)
		argIndex++
	}

	args = append(args, tenantID, channelUUID)

	query := fmt.Sprintf(`
		UPDATE tenant_cdr 
		SET %s 
		WHERE tenant_id = $%d AND channel_uuid = $%d
	`, setClause, argIndex, argIndex+1)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update tenant CDR")
		return fmt.Errorf("failed to update tenant CDR: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id":    tenantID,
		"channel_uuid": channelUUID,
	}).Info("Tenant CDR updated")

	return nil
}

func (r *TenantRepository) GetCDRByChannel(ctx context.Context, tenantID, channelUUID uuid.UUID) (*TenantCallDetailRecord, error) {
	// Validate tenant exists and is active
	if err := r.tenantMgr.ValidateTenantAccess(ctx, tenantID, ""); err != nil {
		return nil, fmt.Errorf("tenant access validation failed: %w", err)
	}

	query := `
		SELECT id, tenant_id, account_code, caller_id_name, caller_id_number, destination_number,
			   start_timestamp, answer_timestamp, end_timestamp, duration_seconds,
			   billsec_seconds, hangup_cause, channel_uuid, destination_channel_uuid,
			   context, created_at
		FROM tenant_cdr
		WHERE tenant_id = $1 AND channel_uuid = $2
	`

	var cdr TenantCallDetailRecord
	err := r.db.QueryRowContext(ctx, query, tenantID, channelUUID).Scan(
		&cdr.ID, &cdr.TenantID, &cdr.AccountCode, &cdr.CallerIDName, &cdr.CallerIDNumber, &cdr.DestinationNumber,
		&cdr.StartTimestamp, &cdr.AnswerTimestamp, &cdr.EndTimestamp, &cdr.DurationSeconds,
		&cdr.BillsecSeconds, &cdr.HangupCause, &cdr.ChannelUUID, &cdr.DestinationChannelUUID,
		&cdr.Context, &cdr.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.WithError(err).Error("Failed to get tenant CDR by channel")
		return nil, fmt.Errorf("failed to get tenant CDR by channel: %w", err)
	}

	return &cdr, nil
}

func (r *TenantRepository) GetActiveCalls(ctx context.Context, tenantID uuid.UUID) ([]*TenantCallDetailRecord, error) {
	// Validate tenant exists and is active
	if err := r.tenantMgr.ValidateTenantAccess(ctx, tenantID, ""); err != nil {
		return nil, fmt.Errorf("tenant access validation failed: %w", err)
	}

	query := `
		SELECT id, tenant_id, account_code, caller_id_name, caller_id_number, destination_number,
			   start_timestamp, answer_timestamp, end_timestamp, duration_seconds,
			   billsec_seconds, hangup_cause, channel_uuid, destination_channel_uuid,
			   context, created_at
		FROM tenant_cdr
		WHERE tenant_id = $1 AND end_timestamp IS NULL
		ORDER BY start_timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get tenant active calls")
		return nil, fmt.Errorf("failed to get tenant active calls: %w", err)
	}
	defer rows.Close()

	var calls []*TenantCallDetailRecord
	for rows.Next() {
		var cdr TenantCallDetailRecord
		err := rows.Scan(
			&cdr.ID, &cdr.TenantID, &cdr.AccountCode, &cdr.CallerIDName, &cdr.CallerIDNumber, &cdr.DestinationNumber,
			&cdr.StartTimestamp, &cdr.AnswerTimestamp, &cdr.EndTimestamp, &cdr.DurationSeconds,
			&cdr.BillsecSeconds, &cdr.HangupCause, &cdr.ChannelUUID, &cdr.DestinationChannelUUID,
			&cdr.Context, &cdr.CreatedAt,
		)
		if err != nil {
			r.logger.WithError(err).Error("Failed to scan tenant CDR row")
			continue
		}
		calls = append(calls, &cdr)
	}

	return calls, nil
}

func (r *TenantRepository) GetTenantCallStatistics(ctx context.Context, tenantID uuid.UUID) (map[string]any, error) {
	// Validate tenant exists and is active
	if err := r.tenantMgr.ValidateTenantAccess(ctx, tenantID, ""); err != nil {
		return nil, fmt.Errorf("tenant access validation failed: %w", err)
	}

	stats := make(map[string]any)

	// Total calls for tenant
	var totalCalls int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenant_cdr WHERE tenant_id = $1", tenantID).Scan(&totalCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant total calls: %w", err)
	}
	stats["total_calls"] = totalCalls

	// Active calls for tenant
	var activeCalls int64
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenant_cdr WHERE tenant_id = $1 AND end_timestamp IS NULL", tenantID).Scan(&activeCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant active calls: %w", err)
	}
	stats["active_calls"] = activeCalls

	// Average duration for tenant
	var avgDuration sql.NullFloat64
	err = r.db.QueryRowContext(ctx, "SELECT AVG(duration_seconds) FROM tenant_cdr WHERE tenant_id = $1 AND duration_seconds IS NOT NULL", tenantID).Scan(&avgDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant average duration: %w", err)
	}
	if avgDuration.Valid {
		stats["average_duration_seconds"] = avgDuration.Float64
	}

	// Calls by hangup cause for tenant
	hangupQuery := `
		SELECT hangup_cause, COUNT(*) as count
		FROM tenant_cdr
		WHERE tenant_id = $1 AND hangup_cause IS NOT NULL
		GROUP BY hangup_cause
		ORDER BY count DESC
		LIMIT 10
	`
	rows, err := r.db.QueryContext(ctx, hangupQuery, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant hangup causes: %w", err)
	}
	defer rows.Close()

	hangupCauses := make(map[string]int64)
	for rows.Next() {
		var cause string
		var count int64
		if err := rows.Scan(&cause, &count); err != nil {
			continue
		}
		hangupCauses[cause] = count
	}
	stats["hangup_causes"] = hangupCauses

	// Calls today for tenant
	var callsToday int64
	today := time.Now().Format("2006-01-02")
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenant_cdr WHERE tenant_id = $1 AND DATE(start_timestamp) = $2", tenantID, today).Scan(&callsToday)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant calls today: %w", err)
	}
	stats["calls_today"] = callsToday

	return stats, nil
}

func (r *TenantRepository) AddMetadata(ctx context.Context, tenantID, cdrID uuid.UUID, key, value string) error {
	// Validate tenant exists and is active
	if err := r.tenantMgr.ValidateTenantAccess(ctx, tenantID, ""); err != nil {
		return fmt.Errorf("tenant access validation failed: %w", err)
	}

	query := `
		INSERT INTO tenant_call_metadata (id, tenant_id, cdr_id, key, value, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (cdr_id, key) DO UPDATE SET
			value = EXCLUDED.value
	`

	metadataID := uuid.New()
	_, err := r.db.ExecContext(ctx, query, metadataID, tenantID, cdrID, key, value, time.Now())
	if err != nil {
		r.logger.WithError(err).Error("Failed to add tenant metadata")
		return fmt.Errorf("failed to add tenant metadata: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"cdr_id":    cdrID,
		"key":       key,
	}).Debug("Tenant metadata added")

	return nil
}

func (r *TenantRepository) GetMetadata(ctx context.Context, tenantID, cdrID uuid.UUID) (map[string]string, error) {
	// Validate tenant exists and is active
	if err := r.tenantMgr.ValidateTenantAccess(ctx, tenantID, ""); err != nil {
		return nil, fmt.Errorf("tenant access validation failed: %w", err)
	}

	query := `
		SELECT key, value
		FROM tenant_call_metadata
		WHERE tenant_id = $1 AND cdr_id = $2
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID, cdrID)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get tenant metadata")
		return nil, fmt.Errorf("failed to get tenant metadata: %w", err)
	}
	defer rows.Close()

	metadata := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		metadata[key] = value
	}

	return metadata, nil
}

func (r *TenantRepository) GetGlobalStats(ctx context.Context) (map[string]any, error) {
	stats := make(map[string]any)

	// Total CDRs across all tenants
	var totalCDRs int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenant_cdr").Scan(&totalCDRs)
	if err != nil {
		return nil, fmt.Errorf("failed to get total CDRs: %w", err)
	}
	stats["total_cdrs"] = totalCDRs

	// Active calls across all tenants
	var totalActiveCalls int64
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenant_cdr WHERE end_timestamp IS NULL").Scan(&totalActiveCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to get total active calls: %w", err)
	}
	stats["total_active_calls"] = totalActiveCalls

	// CDRs by tenant
	tenantQuery := `
		SELECT t.name, COUNT(c.id) as call_count
		FROM tenants t
		LEFT JOIN tenant_cdr c ON t.id = c.tenant_id
		WHERE t.status = 'active'
		GROUP BY t.id, t.name
		ORDER BY call_count DESC
	`
	rows, err := r.db.QueryContext(ctx, tenantQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get CDRs by tenant: %w", err)
	}
	defer rows.Close()

	cdrsByTenant := make(map[string]int64)
	for rows.Next() {
		var tenantName string
		var callCount int64
		if err := rows.Scan(&tenantName, &callCount); err != nil {
			continue
		}
		cdrsByTenant[tenantName] = callCount
	}
	stats["cdrs_by_tenant"] = cdrsByTenant

	return stats, nil
}
