package enrichment

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Pipeline struct {
	db            *sql.DB
	geoResolver   *GeoResolver
	carrierDB     *CarrierDatabase
	fraudDetector *FraudDetector
	logger        *logrus.Logger
}

type EnrichedCDR struct {
	CDRID        uuid.UUID
	TenantID     uuid.UUID
	CallerNumber string
	CalleeNumber string
	StartTime    time.Time
	EndTime      time.Time
	Duration     int
	Status       string
	HangupCause  string

	// Enriched fields
	CallerCountry   string
	CallerRegion    string
	CallerCity      string
	CalleeCountry   string
	CalleeRegion    string
	CalleeCity      string
	CallerCarrier   string
	CalleeCarrier   string
	CallCategory    string
	FraudScore      float64
	FraudIndicators []string
	IsInternational bool
	IsPremium       bool
	IsHighRisk      bool
}

type GeoLocation struct {
	Country string
	Region  string
	City    string
	Lat     float64
	Lon     float64
}

type CarrierInfo struct {
	Name      string
	Type      string
	Country   string
	IsPremium bool
}

func NewPipeline(db *sql.DB) *Pipeline {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Pipeline{
		db:            db,
		geoResolver:   NewGeoResolver(),
		carrierDB:     NewCarrierDatabase(),
		fraudDetector: NewFraudDetector(),
		logger:        logger,
	}
}

func (p *Pipeline) EnrichCDR(ctx context.Context, cdrID uuid.UUID) (*EnrichedCDR, error) {
	// Get base CDR
	cdr, err := p.getBaseCDR(ctx, cdrID)
	if err != nil {
		return nil, fmt.Errorf("failed to get base CDR: %w", err)
	}

	enriched := &EnrichedCDR{
		CDRID:        cdrID,
		TenantID:     cdr.TenantID,
		CallerNumber: cdr.CallerNumber,
		CalleeNumber: cdr.CalleeNumber,
		StartTime:    cdr.StartTime,
		EndTime:      cdr.EndTime,
		Duration:     cdr.Duration,
		Status:       cdr.Status,
		HangupCause:  cdr.HangupCause,
	}

	// Enrich with geographic data
	callerGeo, err := p.geoResolver.Resolve(cdr.CallerNumber)
	if err == nil {
		enriched.CallerCountry = callerGeo.Country
		enriched.CallerRegion = callerGeo.Region
		enriched.CallerCity = callerGeo.City
	}

	calleeGeo, err := p.geoResolver.Resolve(cdr.CalleeNumber)
	if err == nil {
		enriched.CalleeCountry = calleeGeo.Country
		enriched.CalleeRegion = calleeGeo.Region
		enriched.CalleeCity = calleeGeo.City
	}

	// Determine if international
	enriched.IsInternational = (enriched.CallerCountry != "" && enriched.CalleeCountry != "" &&
		enriched.CallerCountry != enriched.CalleeCountry)

	// Enrich with carrier data
	callerCarrier, err := p.carrierDB.Lookup(cdr.CallerNumber)
	if err == nil {
		enriched.CallerCarrier = callerCarrier.Name
		enriched.IsPremium = enriched.IsPremium || callerCarrier.IsPremium
	}

	calleeCarrier, err := p.carrierDB.Lookup(cdr.CalleeNumber)
	if err == nil {
		enriched.CalleeCarrier = calleeCarrier.Name
		enriched.IsPremium = enriched.IsPremium || calleeCarrier.IsPremium
	}

	// Categorize call
	enriched.CallCategory = p.categorizeCall(enriched)

	// Detect fraud
	fraudResult := p.fraudDetector.Analyze(enriched)
	enriched.FraudScore = fraudResult.Score
	enriched.FraudIndicators = fraudResult.Indicators
	enriched.IsHighRisk = fraudResult.Score > 0.7

	// Store enriched data
	if err := p.storeEnrichedCDR(ctx, enriched); err != nil {
		p.logger.WithError(err).Error("Failed to store enriched CDR")
	}

	p.logger.WithFields(logrus.Fields{
		"cdr_id":      cdrID,
		"tenant_id":   enriched.TenantID,
		"category":    enriched.CallCategory,
		"fraud_score": enriched.FraudScore,
	}).Info("CDR enriched successfully")

	return enriched, nil
}

func (p *Pipeline) BatchEnrich(ctx context.Context, cdrIDs []uuid.UUID) ([]*EnrichedCDR, error) {
	results := make([]*EnrichedCDR, 0, len(cdrIDs))

	for _, id := range cdrIDs {
		enriched, err := p.EnrichCDR(ctx, id)
		if err != nil {
			p.logger.WithError(err).WithField("cdr_id", id).Error("Failed to enrich CDR")
			continue
		}
		results = append(results, enriched)
	}

	return results, nil
}

type baseCDR struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	CallerNumber string
	CalleeNumber string
	StartTime    time.Time
	EndTime      time.Time
	Duration     int
	Status       string
	HangupCause  string
}

func (p *Pipeline) getBaseCDR(ctx context.Context, cdrID uuid.UUID) (*baseCDR, error) {
	query := `
		SELECT id, tenant_id, caller_number, callee_number, 
		       start_time, end_time, duration, status, hangup_cause
		FROM cdrs
		WHERE id = $1
	`

	var cdr baseCDR
	err := p.db.QueryRowContext(ctx, query, cdrID).Scan(
		&cdr.ID, &cdr.TenantID, &cdr.CallerNumber, &cdr.CalleeNumber,
		&cdr.StartTime, &cdr.EndTime, &cdr.Duration, &cdr.Status, &cdr.HangupCause,
	)

	return &cdr, err
}

func (p *Pipeline) categorizeCall(enriched *EnrichedCDR) string {
	// Simple categorization logic
	if enriched.IsInternational {
		return "international"
	}

	if enriched.IsPremium {
		return "premium"
	}

	if enriched.Duration > 300 { // > 5 minutes
		return "long_duration"
	}

	if enriched.Status == "failed" {
		return "failed"
	}

	return "standard"
}

func (p *Pipeline) storeEnrichedCDR(ctx context.Context, enriched *EnrichedCDR) error {
	query := `
		INSERT INTO enriched_cdrs (
			cdr_id, tenant_id, caller_country, caller_region, caller_city,
			callee_country, callee_region, callee_city,
			caller_carrier, callee_carrier, call_category,
			fraud_score, fraud_indicators, is_international,
			is_premium, is_high_risk, enriched_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (cdr_id) DO UPDATE SET
			caller_country = EXCLUDED.caller_country,
			callee_country = EXCLUDED.callee_country,
			call_category = EXCLUDED.call_category,
			fraud_score = EXCLUDED.fraud_score,
			is_high_risk = EXCLUDED.is_high_risk,
			enriched_at = EXCLUDED.enriched_at
	`

	indicators := strings.Join(enriched.FraudIndicators, ",")

	_, err := p.db.ExecContext(ctx, query,
		enriched.CDRID, enriched.TenantID,
		enriched.CallerCountry, enriched.CallerRegion, enriched.CallerCity,
		enriched.CalleeCountry, enriched.CalleeRegion, enriched.CalleeCity,
		enriched.CallerCarrier, enriched.CalleeCarrier, enriched.CallCategory,
		enriched.FraudScore, indicators, enriched.IsInternational,
		enriched.IsPremium, enriched.IsHighRisk, time.Now(),
	)

	return err
}
