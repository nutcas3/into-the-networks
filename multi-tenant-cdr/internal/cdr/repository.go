package cdr

import (
	"github.com/nutcas3/multi-tenant-cdr/internal/db"
)

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(cdr *CDR) error {
	query := `
		INSERT INTO cdr (call_uuid, caller, destination, call_start_time, call_end_time, duration_seconds, disposition, hangup_cause)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (call_uuid) DO NOTHING
	`

	_, err := r.db.Exec(query,
		cdr.CallUUID,
		cdr.Caller,
		cdr.Destination,
		cdr.CallStartTime,
		cdr.CallEndTime,
		cdr.DurationSeconds,
		cdr.Disposition,
		cdr.HangupCause,
	)

	return err
}
