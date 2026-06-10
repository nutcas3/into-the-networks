package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Engine struct {
	db     *sql.DB
	cache  Cache
	logger *logrus.Logger
}

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

func NewEngine(db *sql.DB, cache Cache) *Engine {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Engine{
		db:     db,
		cache:  cache,
		logger: logger,
	}
}

type DailyAnalytics struct {
	TenantID           uuid.UUID `json:"tenant_id"`
	Date               time.Time `json:"date"`
	TotalCalls         int64     `json:"total_calls"`
	SuccessfulCalls    int64     `json:"successful_calls"`
	FailedCalls        int64     `json:"failed_calls"`
	AvgDurationSeconds float64   `json:"avg_duration_seconds"`
	TotalDuration      int64     `json:"total_duration_seconds"`
	TotalCost          float64   `json:"total_cost"`
	PeakHour           int       `json:"peak_hour"`
	PeakHourCalls      int64     `json:"peak_hour_calls"`
	UniqueCallers      int64     `json:"unique_callers"`
	UniqueDestinations int64     `json:"unique_destinations"`
}

type HourlyAnalytics struct {
	TenantID        uuid.UUID `json:"tenant_id"`
	Date            time.Time `json:"date"`
	Hour            int       `json:"hour"`
	TotalCalls      int64     `json:"total_calls"`
	SuccessfulCalls int64     `json:"successful_calls"`
	FailedCalls     int64     `json:"failed_calls"`
	AvgDuration     float64   `json:"avg_duration_seconds"`
	TotalCost       float64   `json:"total_cost"`
}

type CallPattern struct {
	HourOfDay     int     `json:"hour_of_day"`
	DayOfWeek     int     `json:"day_of_week"`
	CallCount     int64   `json:"call_count"`
	AvgDuration   float64 `json:"avg_duration"`
	SuccessRate   float64 `json:"success_rate"`
}

type QualityMetrics struct {
	TenantID         uuid.UUID `json:"tenant_id"`
	Period           string   `json:"period"`
	AvgQualityScore  float64   `json:"avg_quality_score"`
	PacketLossRate   float64   `json:"packet_loss_rate"`
	Jitter           float64   `json:"jitter"`
	Latency          float64   `json:"latency"`
	CallsWithQuality int64    `json:"calls_with_quality"`
}

type TrendAnalysis struct {
	TenantID          uuid.UUID `json:"tenant_id"`
	Period            string   `json:"period"`
	CurrentCalls      int64    `json:"current_calls"`
	PreviousCalls     int64    `json:"previous_calls"`
	GrowthRate        float64  `json:"growth_rate"`
	PeakVolume        int64    `json:"peak_volume"`
	PeakVolumeTime    time.Time `json:"peak_volume_time"`
	AvgCallDuration   float64  `json:"avg_call_duration"`
	TrendDirection    string   `json:"trend_direction"` // up, down, stable
}

func (e *Engine) GetDailyAnalytics(ctx context.Context, tenantID uuid.UUID, startDate, endDate time.Time) ([]*DailyAnalytics, error) {
	cacheKey := fmt.Sprintf("analytics:daily:%s:%s:%s", tenantID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	
	// Try cache first
	if e.cache != nil {
		if cached, err := e.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			e.logger.WithField("tenant_id", tenantID).Debug("Analytics cache hit")
			// Deserialize and return (simplified)
		}
	}

	query := `
		SELECT 
			tenant_id, date, total_calls, successful_calls, failed_calls,
			avg_duration_seconds, total_duration_seconds, total_cost,
			peak_hour, peak_hour_calls, unique_callers, unique_destinations
		FROM cdr_analytics_daily
		WHERE tenant_id = $1 AND date >= $2 AND date <= $3
		ORDER BY date ASC
	`

	rows, err := e.db.QueryContext(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily analytics: %w", err)
	}
	defer rows.Close()

	var analytics []*DailyAnalytics
	for rows.Next() {
		var a DailyAnalytics
		var date time.Time
		var avgDuration sql.NullFloat64
		var totalCost sql.NullFloat64

		err := rows.Scan(
			&a.TenantID, &date, &a.TotalCalls, &a.SuccessfulCalls, &a.FailedCalls,
			&avgDuration, &a.TotalDuration, &totalCost,
			&a.PeakHour, &a.PeakHourCalls, &a.UniqueCallers, &a.UniqueDestinations,
		)
		if err != nil {
			e.logger.WithError(err).Error("Failed to scan analytics row")
			continue
		}

		a.Date = date
		if avgDuration.Valid {
			a.AvgDurationSeconds = avgDuration.Float64
		}
		if totalCost.Valid {
			a.TotalCost = totalCost.Float64
		}

		analytics = append(analytics, &a)
	}

	// Cache the result
	if e.cache != nil && len(analytics) > 0 {
		// Serialize and cache (simplified)
		e.cache.Set(ctx, cacheKey, []byte("cached"), 5*time.Minute)
	}

	return analytics, nil
}

func (e *Engine) GetHourlyAnalytics(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]*HourlyAnalytics, error) {
	query := `
		SELECT 
			tenant_id, date, hour, total_calls, successful_calls, failed_calls,
			avg_duration_seconds, total_cost
		FROM cdr_analytics_hourly
		WHERE tenant_id = $1 AND date = $2
		ORDER BY hour ASC
	`

	rows, err := e.db.QueryContext(ctx, query, tenantID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query hourly analytics: %w", err)
	}
	defer rows.Close()

	var analytics []*HourlyAnalytics
	for rows.Next() {
		var a HourlyAnalytics
		var date time.Time
		var avgDuration sql.NullFloat64
		var totalCost sql.NullFloat64

		err := rows.Scan(
			&a.TenantID, &date, &a.Hour, &a.TotalCalls, &a.SuccessfulCalls, &a.FailedCalls,
			&avgDuration, &totalCost,
		)
		if err != nil {
			e.logger.WithError(err).Error("Failed to scan hourly analytics row")
			continue
		}

		a.Date = date
		if avgDuration.Valid {
			a.AvgDuration = avgDuration.Float64
		}
		if totalCost.Valid {
			a.TotalCost = totalCost.Float64
		}

		analytics = append(analytics, &a)
	}

	return analytics, nil
}

func (e *Engine) GetCallPatterns(ctx context.Context, tenantID uuid.UUID, startDate, endDate time.Time) ([]*CallPattern, error) {
	query := `
		SELECT 
			EXTRACT(HOUR FROM start_timestamp) as hour_of_day,
			EXTRACT(DOW FROM start_timestamp) as day_of_week,
			COUNT(*) as call_count,
			AVG(duration_seconds) as avg_duration,
			CASE 
				WHEN hangup_cause = 'NORMAL_CLEARING' THEN 1.0 
				ELSE 0.0 
			END as success_rate
		FROM cdr
		WHERE tenant_id = $1 AND start_timestamp >= $2 AND start_timestamp <= $3
		GROUP BY hour_of_day, day_of_week
		ORDER BY day_of_week, hour_of_day
	`

	rows, err := e.db.QueryContext(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query call patterns: %w", err)
	}
	defer rows.Close()

	var patterns []*CallPattern
	for rows.Next() {
		var p CallPattern
		var avgDuration sql.NullFloat64

		err := rows.Scan(
			&p.HourOfDay, &p.DayOfWeek, &p.CallCount, &avgDuration, &p.SuccessRate,
		)
		if err != nil {
			e.logger.WithError(err).Error("Failed to scan call pattern row")
			continue
		}

		if avgDuration.Valid {
			p.AvgDuration = avgDuration.Float64
		}

		patterns = append(patterns, &p)
	}

	return patterns, nil
}

func (e *Engine) GetQualityMetrics(ctx context.Context, tenantID uuid.UUID, period string) (*QualityMetrics, error) {
	var startDate time.Time
	now := time.Now()

	switch period {
	case "day":
		startDate = now.AddDate(0, 0, -1)
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	default:
		startDate = now.AddDate(0, 0, -7)
	}

	query := `
		SELECT 
			COUNT(*) as calls_with_quality,
			AVG(quality_score) as avg_quality_score
		FROM cdr
		WHERE tenant_id = $1 AND start_timestamp >= $2 AND quality_score IS NOT NULL
	`

	var metrics QualityMetrics
	var callsWithQuality int64
	var avgQualityScore sql.NullFloat64

	err := e.db.QueryRowContext(ctx, query, tenantID, startDate).Scan(
		&callsWithQuality, &avgQualityScore,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query quality metrics: %w", err)
	}

	metrics.TenantID = tenantID
	metrics.Period = period
	metrics.CallsWithQuality = callsWithQuality
	if avgQualityScore.Valid {
		metrics.AvgQualityScore = avgQualityScore.Float64
	}

	// Get enriched data for packet loss, jitter, latency
	enrichedQuery := `
		SELECT 
			AVG((enriched_data->>'packet_loss')::NUMERIC) as packet_loss,
			AVG((enriched_data->>'jitter')::NUMERIC) as jitter,
			AVG((enriched_data->>'latency')::NUMERIC) as latency
		FROM cdr
		WHERE tenant_id = $1 AND start_timestamp >= $2 AND enriched_data IS NOT NULL
	`

	var packetLoss, jitter, latency sql.NullFloat64
	err = e.db.QueryRowContext(ctx, enrichedQuery, tenantID, startDate).Scan(
		&packetLoss, &jitter, &latency,
	)
	if err == nil {
		if packetLoss.Valid {
			metrics.PacketLossRate = packetLoss.Float64
		}
		if jitter.Valid {
			metrics.Jitter = jitter.Float64
		}
		if latency.Valid {
			metrics.Latency = latency.Float64
		}
	}

	return &metrics, nil
}

func (e *Engine) GetTrendAnalysis(ctx context.Context, tenantID uuid.UUID, period string) (*TrendAnalysis, error) {
	var currentStart, previousStart time.Time
	now := time.Now()

	switch period {
	case "day":
		currentStart = now.AddDate(0, 0, -1)
		previousStart = now.AddDate(0, 0, -2)
	case "week":
		currentStart = now.AddDate(0, 0, -7)
		previousStart = now.AddDate(0, 0, -14)
	case "month":
		currentStart = now.AddDate(0, -1, 0)
		previousStart = now.AddDate(0, -2, 0)
	default:
		currentStart = now.AddDate(0, 0, -7)
		previousStart = now.AddDate(0, 0, -14)
	}

	// Get current period stats
	currentQuery := `
		SELECT 
			COUNT(*) as total_calls,
			AVG(duration_seconds) as avg_duration,
			MAX(start_timestamp) as peak_time
		FROM cdr
		WHERE tenant_id = $1 AND start_timestamp >= $2 AND start_timestamp <= $3
	`

	var currentCalls, previousCalls int64
	var currentAvgDuration, previousAvgDuration sql.NullFloat64
	var currentPeakTime, previousPeakTime sql.NullTime

	err := e.db.QueryRowContext(ctx, currentQuery, tenantID, currentStart, now).Scan(
		&currentCalls, &currentAvgDuration, &currentPeakTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query current period stats: %w", err)
	}

	// Get previous period stats
	previousQuery := `
		SELECT 
			COUNT(*) as total_calls,
			AVG(duration_seconds) as avg_duration,
			MAX(start_timestamp) as peak_time
		FROM cdr
		WHERE tenant_id = $1 AND start_timestamp >= $2 AND start_timestamp < $3
	`

	err = e.db.QueryRowContext(ctx, previousQuery, tenantID, previousStart, currentStart).Scan(
		&previousCalls, &previousAvgDuration, &previousPeakTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query previous period stats: %w", err)
	}

	var analysis TrendAnalysis
	analysis.TenantID = tenantID
	analysis.Period = period
	analysis.CurrentCalls = currentCalls
	analysis.PreviousCalls = previousCalls

	if previousCalls > 0 {
		analysis.GrowthRate = float64(currentCalls-previousCalls) / float64(previousCalls) * 100
	}

	analysis.PeakVolume = currentCalls
	if currentPeakTime.Valid {
		analysis.PeakVolumeTime = currentPeakTime.Time
	}

	if currentAvgDuration.Valid {
		analysis.AvgCallDuration = currentAvgDuration.Float64
	}

	// Determine trend direction
	if analysis.GrowthRate > 5 {
		analysis.TrendDirection = "up"
	} else if analysis.GrowthRate < -5 {
		analysis.TrendDirection = "down"
	} else {
		analysis.TrendDirection = "stable"
	}

	return &analysis, nil
}

func (e *Engine) RefreshAnalytics(ctx context.Context, tenantID uuid.UUID, date time.Time) error {
	// Recalculate daily analytics for a specific date
	query := `
		INSERT INTO cdr_analytics_daily (
			tenant_id, date, total_calls, successful_calls, failed_calls,
			total_duration_seconds, total_cost, unique_callers, unique_destinations
		)
		SELECT 
			tenant_id,
			DATE(start_timestamp) as date,
			COUNT(*) as total_calls,
			COUNT(*) FILTER (WHERE hangup_cause = 'NORMAL_CLEARING') as successful_calls,
			COUNT(*) FILTER (WHERE hangup_cause != 'NORMAL_CLEARING') as failed_calls,
			COALESCE(SUM(duration_seconds), 0) as total_duration_seconds,
			COALESCE(SUM(cost), 0) as total_cost,
			COUNT(DISTINCT caller_id_number) as unique_callers,
			COUNT(DISTINCT destination_number) as unique_destinations
		FROM cdr
		WHERE tenant_id = $1 AND DATE(start_timestamp) = $2
		GROUP BY tenant_id, DATE(start_timestamp)
		ON CONFLICT (tenant_id, date) DO UPDATE SET
			total_calls = EXCLUDED.total_calls,
			successful_calls = EXCLUDED.successful_calls,
			failed_calls = EXCLUDED.failed_calls,
			total_duration_seconds = EXCLUDED.total_duration_seconds,
			total_cost = EXCLUDED.total_cost,
			unique_callers = EXCLUDED.unique_callers,
			unique_destinations = EXCLUDED.unique_destinations,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := e.db.ExecContext(ctx, query, tenantID, date)
	if err != nil {
		return fmt.Errorf("failed to refresh daily analytics: %w", err)
	}

	e.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"date":      date,
	}).Info("Daily analytics refreshed")

	return nil
}

func (e *Engine) GetTenantSummary(ctx context.Context, tenantID uuid.UUID) (map[string]any, error) {
	summary := make(map[string]any)

	// Total calls
	var totalCalls int64
	err := e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cdr WHERE tenant_id = $1", tenantID).Scan(&totalCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to get total calls: %w", err)
	}
	summary["total_calls"] = totalCalls

	// Today's calls
	var todayCalls int64
	today := time.Now().Format("2006-01-02")
	err = e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cdr WHERE tenant_id = $1 AND DATE(start_timestamp) = $2", tenantID, today).Scan(&todayCalls)
	if err == nil {
		summary["today_calls"] = todayCalls
	}

	// Active calls (no end timestamp)
	var activeCalls int64
	err = e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cdr WHERE tenant_id = $1 AND end_timestamp IS NULL", tenantID).Scan(&activeCalls)
	if err == nil {
		summary["active_calls"] = activeCalls
	}

	// Average duration
	var avgDuration sql.NullFloat64
	err = e.db.QueryRowContext(ctx, "SELECT AVG(duration_seconds) FROM cdr WHERE tenant_id = $1 AND duration_seconds IS NOT NULL", tenantID).Scan(&avgDuration)
	if err == nil && avgDuration.Valid {
		summary["avg_duration_seconds"] = avgDuration.Float64
	}

	// Total cost
	var totalCost sql.NullFloat64
	err = e.db.QueryRowContext(ctx, "SELECT SUM(cost) FROM cdr WHERE tenant_id = $1", tenantID).Scan(&totalCost)
	if err == nil && totalCost.Valid {
		summary["total_cost"] = totalCost.Float64
	}

	// Success rate
	var successfulCalls int64
	err = e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cdr WHERE tenant_id = $1 AND hangup_cause = 'NORMAL_CLEARING'", tenantID).Scan(&successfulCalls)
	if err == nil && totalCalls > 0 {
		summary["success_rate"] = float64(successfulCalls) / float64(totalCalls) * 100
	}

	return summary, nil
}
