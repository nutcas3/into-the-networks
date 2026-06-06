package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nutcas3/multi-tenant-cdr/internal/analytics"
)

type Handler struct {
	db         *sql.DB
	analytics  *analytics.Engine
}

func NewHandler(db *sql.DB, analyticsEngine *analytics.Engine) *Handler {
	return &Handler{
		db:        db,
		analytics: analyticsEngine,
	}
}

type CDRResponse struct {
	ID                    uuid.UUID  `json:"id"`
	TenantID              uuid.UUID  `json:"tenant_id"`
	AccountCode           string     `json:"account_code"`
	CallerIDName          string     `json:"caller_id_name"`
	CallerIDNumber        string     `json:"caller_id_number"`
	DestinationNumber     string     `json:"destination_number"`
	StartTimestamp        time.Time  `json:"start_timestamp"`
	AnswerTimestamp       *time.Time `json:"answer_timestamp,omitempty"`
	EndTimestamp          *time.Time `json:"end_timestamp,omitempty"`
	DurationSeconds       *int64     `json:"duration_seconds,omitempty"`
	BillsecSeconds        *int64     `json:"billsec_seconds,omitempty"`
	HangupCause           string     `json:"hangup_cause"`
	ChannelUUID           uuid.UUID  `json:"channel_uuid"`
	DestinationChannelUUID *string    `json:"destination_channel_uuid,omitempty"`
	Context               string     `json:"context"`
	CreatedAt             time.Time  `json:"created_at"`
	Cost                  *float64   `json:"cost,omitempty"`
	QualityScore          *float64   `json:"quality_score,omitempty"`
}

type CDRQueryParams struct {
	StartDate         *time.Time `form:"start_date"`
	EndDate           *time.Time `form:"end_date"`
	CallerIDNumber    string     `form:"caller_id_number"`
	DestinationNumber string     `form:"destination_number"`
	HangupCause       string     `form:"hangup_cause"`
	MinDuration       *int       `form:"min_duration"`
	MaxDuration       *int       `form:"max_duration"`
	Limit             int        `form:"limit"`
	Offset            int        `form:"offset"`
}

func (h *Handler) GetCDRs(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	
	var params CDRQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default pagination
	if params.Limit == 0 {
		params.Limit = 100
	}
	if params.Limit > 1000 {
		params.Limit = 1000
	}

	query := `
		SELECT 
			id, tenant_id, account_code, caller_id_name, caller_id_number,
			destination_number, start_timestamp, answer_timestamp, end_timestamp,
			duration_seconds, billsec_seconds, hangup_cause, channel_uuid,
			destination_channel_uuid, context, created_at, cost, quality_score
		FROM cdr
		WHERE tenant_id = $1
	`
	args := []any{tenantID}
	argIndex := 2

	// Build dynamic WHERE clause
	if params.StartDate != nil {
		query += " AND start_timestamp >= $" + strconv.Itoa(argIndex)
		args = append(args, params.StartDate)
		argIndex++
	}
	if params.EndDate != nil {
		query += " AND start_timestamp <= $" + strconv.Itoa(argIndex)
		args = append(args, params.EndDate)
		argIndex++
	}
	if params.CallerIDNumber != "" {
		query += " AND caller_id_number = $" + strconv.Itoa(argIndex)
		args = append(args, params.CallerIDNumber)
		argIndex++
	}
	if params.DestinationNumber != "" {
		query += " AND destination_number = $" + strconv.Itoa(argIndex)
		args = append(args, params.DestinationNumber)
		argIndex++
	}
	if params.HangupCause != "" {
		query += " AND hangup_cause = $" + strconv.Itoa(argIndex)
		args = append(args, params.HangupCause)
		argIndex++
	}
	if params.MinDuration != nil {
		query += " AND duration_seconds >= $" + strconv.Itoa(argIndex)
		args = append(args, *params.MinDuration)
		argIndex++
	}
	if params.MaxDuration != nil {
		query += " AND duration_seconds <= $" + strconv.Itoa(argIndex)
		args = append(args, *params.MaxDuration)
		argIndex++
	}

	query += " ORDER BY start_timestamp DESC LIMIT $" + strconv.Itoa(argIndex) + " OFFSET $" + strconv.Itoa(argIndex+1)
	args = append(args, params.Limit, params.Offset)

	rows, err := h.db.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query CDRs"})
		return
	}
	defer rows.Close()

	var cdrs []CDRResponse
	for rows.Next() {
		var cdr CDRResponse
		var answerTimestamp, endTimestamp sql.NullTime
		var durationSeconds, billsecSeconds sql.NullInt64
		var destinationChannelUUID sql.NullString
		var cost, qualityScore sql.NullFloat64

		err := rows.Scan(
			&cdr.ID, &cdr.TenantID, &cdr.AccountCode, &cdr.CallerIDName, &cdr.CallerIDNumber,
			&cdr.DestinationNumber, &cdr.StartTimestamp, &answerTimestamp, &endTimestamp,
			&durationSeconds, &billsecSeconds, &cdr.HangupCause, &cdr.ChannelUUID,
			&destinationChannelUUID, &cdr.Context, &cdr.CreatedAt, &cost, &qualityScore,
		)
		if err != nil {
			continue
		}

		if answerTimestamp.Valid {
			cdr.AnswerTimestamp = &answerTimestamp.Time
		}
		if endTimestamp.Valid {
			cdr.EndTimestamp = &endTimestamp.Time
		}
		if durationSeconds.Valid {
			cdr.DurationSeconds = &durationSeconds.Int64
		}
		if billsecSeconds.Valid {
			cdr.BillsecSeconds = &billsecSeconds.Int64
		}
		if destinationChannelUUID.Valid {
			cdr.DestinationChannelUUID = &destinationChannelUUID.String
		}
		if cost.Valid {
			cdr.Cost = &cost.Float64
		}
		if qualityScore.Valid {
			cdr.QualityScore = &qualityScore.Float64
		}

		cdrs = append(cdrs, cdr)
	}

	// Get total count for pagination
	countQuery := "SELECT COUNT(*) FROM cdr WHERE tenant_id = $1"
	var total int64
	h.db.QueryRowContext(c.Request.Context(), countQuery, tenantID).Scan(&total)

	c.JSON(http.StatusOK, gin.H{
		"data":  cdrs,
		"total": total,
		"limit": params.Limit,
		"offset": params.Offset,
	})
}

func (h *Handler) GetCDR(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	cdrID := c.Param("id")

	cdrUUID, err := uuid.Parse(cdrID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CDR ID"})
		return
	}

	query := `
		SELECT 
			id, tenant_id, account_code, caller_id_name, caller_id_number,
			destination_number, start_timestamp, answer_timestamp, end_timestamp,
			duration_seconds, billsec_seconds, hangup_cause, channel_uuid,
			destination_channel_uuid, context, created_at, cost, quality_score
		FROM cdr
		WHERE id = $1 AND tenant_id = $2
	`

	var cdr CDRResponse
	var answerTimestamp, endTimestamp sql.NullTime
	var durationSeconds, billsecSeconds sql.NullInt64
	var destinationChannelUUID sql.NullString
	var cost, qualityScore sql.NullFloat64

	err = h.db.QueryRowContext(c.Request.Context(), query, cdrUUID, tenantID).Scan(
		&cdr.ID, &cdr.TenantID, &cdr.AccountCode, &cdr.CallerIDName, &cdr.CallerIDNumber,
		&cdr.DestinationNumber, &cdr.StartTimestamp, &answerTimestamp, &endTimestamp,
		&durationSeconds, &billsecSeconds, &cdr.HangupCause, &cdr.ChannelUUID,
		&destinationChannelUUID, &cdr.Context, &cdr.CreatedAt, &cost, &qualityScore,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "CDR not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query CDR"})
		}
		return
	}

	if answerTimestamp.Valid {
		cdr.AnswerTimestamp = &answerTimestamp.Time
	}
	if endTimestamp.Valid {
		cdr.EndTimestamp = &endTimestamp.Time
	}
	if durationSeconds.Valid {
		cdr.DurationSeconds = &durationSeconds.Int64
	}
	if billsecSeconds.Valid {
		cdr.BillsecSeconds = &billsecSeconds.Int64
	}
	if destinationChannelUUID.Valid {
		cdr.DestinationChannelUUID = &destinationChannelUUID.String
	}
	if cost.Valid {
		cdr.Cost = &cost.Float64
	}
	if qualityScore.Valid {
		cdr.QualityScore = &qualityScore.Float64
	}

	c.JSON(http.StatusOK, cdr)
}

func (h *Handler) GetAnalytics(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	// Parse date range
	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format"})
		return
	}

	// Get daily analytics
	dailyAnalytics, err := h.analytics.GetDailyAnalytics(c.Request.Context(), tenantUUID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get analytics"})
		return
	}

	// Get tenant summary
	summary, err := h.analytics.GetTenantSummary(c.Request.Context(), tenantUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get summary"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"daily_analytics": dailyAnalytics,
		"summary":         summary,
		"start_date":      startDate,
		"end_date":        endDate,
	})
}

func (h *Handler) GetCallPatterns(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format"})
		return
	}

	patterns, err := h.analytics.GetCallPatterns(c.Request.Context(), tenantUUID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get call patterns"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"patterns":   patterns,
		"start_date": startDate,
		"end_date":   endDate,
	})
}

func (h *Handler) GetQualityMetrics(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	period := c.DefaultQuery("period", "week")

	metrics, err := h.analytics.GetQualityMetrics(c.Request.Context(), tenantUUID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get quality metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *Handler) GetTrendAnalysis(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	period := c.DefaultQuery("period", "week")

	analysis, err := h.analytics.GetTrendAnalysis(c.Request.Context(), tenantUUID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trend analysis"})
		return
	}

	c.JSON(http.StatusOK, analysis)
}

func (h *Handler) GetStatistics(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	summary, err := h.analytics.GetTenantSummary(c.Request.Context(), tenantUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, summary)
}
