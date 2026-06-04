package cdr

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type CallDetailRecord struct {
	ID                     uuid.UUID      `db:"id"`
	AccountCode            string         `db:"account_code"`
	CallerIDName           string         `db:"caller_id_name"`
	CallerIDNumber         string         `db:"caller_id_number"`
	DestinationNumber      string         `db:"destination_number"`
	StartTimestamp         time.Time      `db:"start_timestamp"`
	AnswerTimestamp        sql.NullTime   `db:"answer_timestamp"`
	EndTimestamp           sql.NullTime   `db:"end_timestamp"`
	DurationSeconds        sql.NullInt64  `db:"duration_seconds"`
	BillsecSeconds         sql.NullInt64  `db:"billsec_seconds"`
	HangupCause            string         `db:"hangup_cause"`
	ChannelUUID            uuid.UUID      `db:"channel_uuid"`
	DestinationChannelUUID sql.NullString `db:"destination_channel_uuid"`
	Context                string         `db:"context"`
	CreatedAt              time.Time      `db:"created_at"`
}

type CallMetadata struct {
	ID    uuid.UUID `db:"id"`
	CDRID uuid.UUID `db:"cdr_id"`
	Key   string    `db:"key"`
	Value string    `db:"value"`
}

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

func (r *Repository) CreateCDR(ctx context.Context, cdr *CallDetailRecord) error {
	query := `
		INSERT INTO cdr (
			id, account_code, caller_id_name, caller_id_number, destination_number,
			start_timestamp, answer_timestamp, end_timestamp, duration_seconds,
			billsec_seconds, hangup_cause, channel_uuid, destination_channel_uuid,
			context, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		) ON CONFLICT (id) DO UPDATE SET
			answer_timestamp = EXCLUDED.answer_timestamp,
			end_timestamp = EXCLUDED.end_timestamp,
			duration_seconds = EXCLUDED.duration_seconds,
			billsec_seconds = EXCLUDED.billsec_seconds,
			hangup_cause = EXCLUDED.hangup_cause,
			destination_channel_uuid = EXCLUDED.destination_channel_uuid
	`

	_, err := r.db.ExecContext(ctx, query,
		cdr.ID, cdr.AccountCode, cdr.CallerIDName, cdr.CallerIDNumber, cdr.DestinationNumber,
		cdr.StartTimestamp, cdr.AnswerTimestamp, cdr.EndTimestamp, cdr.DurationSeconds,
		cdr.BillsecSeconds, cdr.HangupCause, cdr.ChannelUUID, cdr.DestinationChannelUUID,
		cdr.Context, cdr.CreatedAt,
	)

	if err != nil {
		r.logger.WithError(err).Error("Failed to create CDR")
		return fmt.Errorf("failed to create CDR: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"cdr_id":      cdr.ID,
		"caller":      cdr.CallerIDNumber,
		"destination": cdr.DestinationNumber,
		"channel":     cdr.ChannelUUID,
	}).Info("CDR created/updated")

	return nil
}

func (r *Repository) UpdateCDR(ctx context.Context, channelUUID uuid.UUID, updates map[string]any) error {
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

	args = append(args, channelUUID)

	query := fmt.Sprintf("UPDATE cdr SET %s WHERE channel_uuid = $%d", setClause, argIndex)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update CDR")
		return fmt.Errorf("failed to update CDR: %w", err)
	}

	r.logger.WithField("channel_uuid", channelUUID).Info("CDR updated")
	return nil
}

func (r *Repository) GetCDRByChannel(ctx context.Context, channelUUID uuid.UUID) (*CallDetailRecord, error) {
	query := `
		SELECT id, account_code, caller_id_name, caller_id_number, destination_number,
			   start_timestamp, answer_timestamp, end_timestamp, duration_seconds,
			   billsec_seconds, hangup_cause, channel_uuid, destination_channel_uuid,
			   context, created_at
		FROM cdr
		WHERE channel_uuid = $1
	`

	var cdr CallDetailRecord
	err := r.db.QueryRowContext(ctx, query, channelUUID).Scan(
		&cdr.ID, &cdr.AccountCode, &cdr.CallerIDName, &cdr.CallerIDNumber, &cdr.DestinationNumber,
		&cdr.StartTimestamp, &cdr.AnswerTimestamp, &cdr.EndTimestamp, &cdr.DurationSeconds,
		&cdr.BillsecSeconds, &cdr.HangupCause, &cdr.ChannelUUID, &cdr.DestinationChannelUUID,
		&cdr.Context, &cdr.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.WithError(err).Error("Failed to get CDR by channel")
		return nil, fmt.Errorf("failed to get CDR by channel: %w", err)
	}

	return &cdr, nil
}

func (r *Repository) GetActiveCalls(ctx context.Context) ([]*CallDetailRecord, error) {
	query := `
		SELECT id, account_code, caller_id_name, caller_id_number, destination_number,
			   start_timestamp, answer_timestamp, end_timestamp, duration_seconds,
			   billsec_seconds, hangup_cause, channel_uuid, destination_channel_uuid,
			   context, created_at
		FROM cdr
		WHERE end_timestamp IS NULL
		ORDER BY start_timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get active calls")
		return nil, fmt.Errorf("failed to get active calls: %w", err)
	}
	defer rows.Close()

	var calls []*CallDetailRecord
	for rows.Next() {
		var cdr CallDetailRecord
		err := rows.Scan(
			&cdr.ID, &cdr.AccountCode, &cdr.CallerIDName, &cdr.CallerIDNumber, &cdr.DestinationNumber,
			&cdr.StartTimestamp, &cdr.AnswerTimestamp, &cdr.EndTimestamp, &cdr.DurationSeconds,
			&cdr.BillsecSeconds, &cdr.HangupCause, &cdr.ChannelUUID, &cdr.DestinationChannelUUID,
			&cdr.Context, &cdr.CreatedAt,
		)
		if err != nil {
			r.logger.WithError(err).Error("Failed to scan CDR row")
			continue
		}
		calls = append(calls, &cdr)
	}

	return calls, nil
}

func (r *Repository) GetCallStatistics(ctx context.Context) (map[string]any, error) {
	stats := make(map[string]any)

	// Total calls
	var totalCalls int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cdr").Scan(&totalCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to get total calls: %w", err)
	}
	stats["total_calls"] = totalCalls

	// Active calls
	var activeCalls int64
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cdr WHERE end_timestamp IS NULL").Scan(&activeCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to get active calls: %w", err)
	}
	stats["active_calls"] = activeCalls

	// Average duration
	var avgDuration sql.NullFloat64
	err = r.db.QueryRowContext(ctx, "SELECT AVG(duration_seconds) FROM cdr WHERE duration_seconds IS NOT NULL").Scan(&avgDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to get average duration: %w", err)
	}
	if avgDuration.Valid {
		stats["average_duration_seconds"] = avgDuration.Float64
	}

	// Calls by hangup cause
	hangupQuery := `
		SELECT hangup_cause, COUNT(*) as count
		FROM cdr
		WHERE hangup_cause IS NOT NULL
		GROUP BY hangup_cause
		ORDER BY count DESC
		LIMIT 10
	`
	rows, err := r.db.QueryContext(ctx, hangupQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get hangup causes: %w", err)
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

	return stats, nil
}

func (r *Repository) AddMetadata(ctx context.Context, cdrID uuid.UUID, key, value string) error {
	query := `
		INSERT INTO call_metadata (id, cdr_id, key, value, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cdr_id, key) DO UPDATE SET
			value = EXCLUDED.value
	`

	metadataID := uuid.New()
	_, err := r.db.ExecContext(ctx, query, metadataID, cdrID, key, value, time.Now())
	if err != nil {
		r.logger.WithError(err).Error("Failed to add metadata")
		return fmt.Errorf("failed to add metadata: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"cdr_id": cdrID,
		"key":    key,
	}).Debug("Metadata added")

	return nil
}

func (r *Repository) GetMetadata(ctx context.Context, cdrID uuid.UUID) (map[string]string, error) {
	query := `
		SELECT key, value
		FROM call_metadata
		WHERE cdr_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, cdrID)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get metadata")
		return nil, fmt.Errorf("failed to get metadata: %w", err)
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
