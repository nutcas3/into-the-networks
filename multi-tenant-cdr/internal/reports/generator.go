package reports

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"
	"github.com/nutcas3/multi-tenant-cdr/internal/analytics"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

type Generator struct {
	db        *sql.DB
	analytics *analytics.Engine
	logger    *logrus.Logger
}

type GeneratedReport struct {
	ID           uuid.UUID      `json:"id"`
	TenantID     uuid.UUID      `json:"tenant_id"`
	TemplateID   uuid.UUID      `json:"template_id"`
	ReportType   string         `json:"report_type"`
	Status       string         `json:"status"`
	Parameters   map[string]any `json:"parameters"`
	FilePath     string         `json:"file_path"`
	FileSize     int64          `json:"file_size"`
	GeneratedAt  time.Time      `json:"generated_at"`
	ExpiresAt    time.Time      `json:"expires_at"`
	ErrorMessage string         `json:"error_message,omitempty"`
}

func NewGenerator(db *sql.DB, analyticsEngine *analytics.Engine) *Generator {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Generator{
		db:        db,
		analytics: analyticsEngine,
		logger:    logger,
	}
}

func (g *Generator) GenerateReport(ctx context.Context, tenantID, templateID uuid.UUID, parameters map[string]any) (*GeneratedReport, error) {
	// Get template
	template, err := getTemplate(ctx, g.db, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Create report record
	report := &GeneratedReport{
		ID:         uuid.New(),
		TenantID:   tenantID,
		TemplateID: templateID,
		ReportType: template.TemplateType,
		Status:     "generating",
		Parameters: parameters,
	}

	// Insert report record
	if err := g.insertReportRecord(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to create report record: %w", err)
	}

	// Generate report based on type
	var filePath string
	var fileSize int64
	var genErr error

	switch template.TemplateType {
	case "cdr_summary":
		filePath, fileSize, genErr = g.generateCDRSummary(ctx, tenantID, template, parameters)
	case "billing":
		filePath, fileSize, genErr = g.generateBillingReport(ctx, tenantID, template, parameters)
	case "quality":
		filePath, fileSize, genErr = g.generateQualityReport(ctx, tenantID, template, parameters)
	default:
		genErr = fmt.Errorf("unsupported report type: %s", template.TemplateType)
	}

	if genErr != nil {
		report.Status = "failed"
		report.ErrorMessage = genErr.Error()
		g.updateReportStatus(ctx, report.ID, "failed", genErr.Error())
		return report, genErr
	}

	// Update report with file info
	report.FilePath = filePath
	report.FileSize = fileSize
	report.Status = "completed"
	report.GeneratedAt = time.Now()
	report.ExpiresAt = time.Now().Add(30 * 24 * time.Hour) // 30 days

	g.updateReportRecord(ctx, report)

	g.logger.WithFields(logrus.Fields{
		"report_id":   report.ID,
		"tenant_id":   tenantID,
		"report_type": template.TemplateType,
		"file_path":   filePath,
	}).Info("Report generated successfully")

	return report, nil
}

func (g *Generator) generateCDRSummary(ctx context.Context, tenantID uuid.UUID, template *Template, parameters map[string]any) (string, int64, error) {
	// Get analytics data
	var startDate, endDate time.Time
	if start, ok := parameters["start_date"].(string); ok {
		startDate, _ = time.Parse("2006-01-02", start)
	} else {
		startDate = time.Now().AddDate(0, 0, -1)
	}

	if end, ok := parameters["end_date"].(string); ok {
		endDate, _ = time.Parse("2006-01-02", end)
	} else {
		endDate = time.Now()
	}

	dailyAnalytics, err := g.analytics.GetDailyAnalytics(ctx, tenantID, startDate, endDate)
	if err != nil {
		return "", 0, err
	}

	// Generate PDF report
	filePath := fmt.Sprintf("/tmp/report_%s.pdf", uuid.New().String())
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(190, 10, "Daily Call Summary Report")
	pdf.Ln(12)

	// Tenant info
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(190, 6, fmt.Sprintf("Tenant ID: %s", tenantID))
	pdf.Ln(6)
	pdf.Cell(190, 6, fmt.Sprintf("Period: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))
	pdf.Ln(12)

	// Summary table
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(190, 8, "Call Summary")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 10)
	for _, analytics := range dailyAnalytics {
		pdf.Cell(190, 6, fmt.Sprintf("%s: %d calls (%d successful, %d failed)",
			analytics.Date.Format("2006-01-02"), analytics.TotalCalls,
			analytics.SuccessfulCalls, analytics.FailedCalls))
		pdf.Ln(6)
	}

	err = pdf.OutputFileAndClose(filePath)
	if err != nil {
		return "", 0, err
	}

	return filePath, 0, nil
}

func (g *Generator) generateBillingReport(ctx context.Context, tenantID uuid.UUID, template *Template, parameters map[string]any) (string, int64, error) {
	// Create Excel file
	filePath := fmt.Sprintf("/tmp/billing_%s.xlsx", uuid.New().String())
	f := excelize.NewFile()
	defer f.Close()

	// Create sheet
	sheetName := "Billing Report"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return "", 0, err
	}
	f.SetActiveSheet(index)

	// Set headers
	headers := []string{"Date", "Total Calls", "Successful Calls", "Failed Calls", "Total Duration (sec)", "Total Cost"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// Get analytics data
	var startDate, endDate time.Time
	if start, ok := parameters["start_date"].(string); ok {
		startDate, _ = time.Parse("2006-01-02", start)
	} else {
		startDate = time.Now().AddDate(0, 0, -7)
	}

	if end, ok := parameters["end_date"].(string); ok {
		endDate, _ = time.Parse("2006-01-02", end)
	} else {
		endDate = time.Now()
	}

	dailyAnalytics, err := g.analytics.GetDailyAnalytics(ctx, tenantID, startDate, endDate)
	if err != nil {
		return "", 0, err
	}

	// Fill data
	row := 2
	for _, analytics := range dailyAnalytics {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), analytics.Date.Format("2006-01-02"))
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), analytics.TotalCalls)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), analytics.SuccessfulCalls)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), analytics.FailedCalls)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), analytics.TotalDuration)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), analytics.TotalCost)
		row++
	}

	// Save file
	if err := f.SaveAs(filePath); err != nil {
		return "", 0, err
	}

	return filePath, 0, nil
}

func (g *Generator) generateQualityReport(ctx context.Context, tenantID uuid.UUID, template *Template, parameters map[string]any) (string, int64, error) {
	period := "week"
	if p, ok := parameters["period"].(string); ok {
		period = p
	}

	metrics, err := g.analytics.GetQualityMetrics(ctx, tenantID, period)
	if err != nil {
		return "", 0, err
	}

	// Generate PDF report
	filePath := fmt.Sprintf("/tmp/quality_%s.pdf", uuid.New().String())
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(190, 10, "Call Quality Report")
	pdf.Ln(12)

	// Metrics
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(190, 6, fmt.Sprintf("Tenant ID: %s", tenantID))
	pdf.Ln(6)
	pdf.Cell(190, 6, fmt.Sprintf("Period: %s", period))
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(190, 8, "Quality Metrics")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 10)
	pdf.Cell(190, 6, fmt.Sprintf("Average Quality Score: %.2f", metrics.AvgQualityScore))
	pdf.Ln(6)
	pdf.Cell(190, 6, fmt.Sprintf("Packet Loss Rate: %.2f%%", metrics.PacketLossRate))
	pdf.Ln(6)
	pdf.Cell(190, 6, fmt.Sprintf("Jitter: %.2f ms", metrics.Jitter))
	pdf.Ln(6)
	pdf.Cell(190, 6, fmt.Sprintf("Latency: %.2f ms", metrics.Latency))
	pdf.Ln(6)
	pdf.Cell(190, 6, fmt.Sprintf("Calls with Quality Data: %d", metrics.CallsWithQuality))

	err = pdf.OutputFileAndClose(filePath)
	if err != nil {
		return "", 0, err
	}

	return filePath, 0, nil
}

func (g *Generator) insertReportRecord(ctx context.Context, report *GeneratedReport) error {
	query := `
		INSERT INTO generated_reports (
			id, tenant_id, template_id, report_type, status, 
			parameters, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	parametersJSON, _ := json.Marshal(report.Parameters)

	_, err := g.db.ExecContext(ctx, query,
		report.ID, report.TenantID, report.TemplateID, report.ReportType,
		report.Status, parametersJSON, time.Now(),
	)

	return err
}

func (g *Generator) updateReportRecord(ctx context.Context, report *GeneratedReport) error {
	query := `
		UPDATE generated_reports 
		SET status = $1, file_path = $2, file_size = $3, 
		    generated_at = $4, expires_at = $5
		WHERE id = $6
	`

	_, err := g.db.ExecContext(ctx, query,
		report.Status, report.FilePath, report.FileSize,
		report.GeneratedAt, report.ExpiresAt, report.ID,
	)

	return err
}

func (g *Generator) updateReportStatus(ctx context.Context, reportID uuid.UUID, status, errorMessage string) error {
	query := `
		UPDATE generated_reports 
		SET status = $1, error_message = $2
		WHERE id = $3
	`

	_, err := g.db.ExecContext(ctx, query, status, errorMessage, reportID)
	return err
}

func getTemplate(ctx context.Context, db *sql.DB, id uuid.UUID) (*Template, error) {
	query := `
		SELECT 
			id, tenant_id, name, description, template_type,
			template_config, schedule_config, output_format,
			created_by, created_at, updated_at
		FROM report_templates
		WHERE id = $1
	`

	var template Template
	var templateConfigJSON, scheduleConfigJSON []byte

	err := db.QueryRowContext(ctx, query, id).Scan(
		&template.ID, &template.TenantID, &template.Name, &template.Description,
		&template.TemplateType, &templateConfigJSON, &scheduleConfigJSON,
		&template.OutputFormat, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	json.Unmarshal(templateConfigJSON, &template.TemplateConfig)
	json.Unmarshal(scheduleConfigJSON, &template.ScheduleConfig)

	return &template, nil
}
