package reports

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Template struct {
	ID             uuid.UUID              `json:"id"`
	TenantID       uuid.UUID              `json:"tenant_id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	TemplateType   string                 `json:"template_type"`
	TemplateConfig map[string]interface{} `json:"template_config"`
	ScheduleConfig map[string]interface{} `json:"schedule_config"`
	OutputFormat   string                 `json:"output_format"`
	CreatedBy      string                 `json:"created_by"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type ReportTemplateConfig struct {
	Columns      []string          `json:"columns"`
	Filters      map[string]string `json:"filters"`
	GroupBy      []string          `json:"group_by"`
	Aggregations []string          `json:"aggregations"`
	SortBy       string            `json:"sort_by"`
	SortOrder    string            `json:"sort_order"`
	Limit        int               `json:"limit"`
}

type ScheduleConfig struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"` // daily, weekly, monthly
	Cron      string `json:"cron"`
	Timezone  string `json:"timezone"`
	Recipients []string `json:"recipients"`
}

type Manager struct {
	db     *sql.DB
	logger *logrus.Logger
}

func NewManager(db *sql.DB) *Manager {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &Manager{
		db:     db,
		logger: logger,
	}
}

func (m *Manager) CreateTemplate(ctx context.Context, template *Template) error {
	templateConfigJSON, err := json.Marshal(template.TemplateConfig)
	if err != nil {
		return err
	}

	scheduleConfigJSON, err := json.Marshal(template.ScheduleConfig)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO report_templates (
			id, tenant_id, name, description, template_type, 
			template_config, schedule_config, output_format, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	err = m.db.QueryRowContext(ctx, query,
		template.ID, template.TenantID, template.Name, template.Description,
		template.TemplateType, templateConfigJSON, scheduleConfigJSON,
		template.OutputFormat, template.CreatedBy,
	).Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)

	if err != nil {
		m.logger.WithError(err).Error("Failed to create report template")
		return err
	}

	m.logger.WithFields(logrus.Fields{
		"template_id": template.ID,
		"tenant_id":   template.TenantID,
		"name":        template.Name,
	}).Info("Report template created")

	return nil
}

func (m *Manager) GetTemplate(ctx context.Context, id uuid.UUID) (*Template, error) {
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

	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&template.ID, &template.TenantID, &template.Name, &template.Description,
		&template.TemplateType, &templateConfigJSON, &scheduleConfigJSON,
		&template.OutputFormat, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(templateConfigJSON, &template.TemplateConfig); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(scheduleConfigJSON, &template.ScheduleConfig); err != nil {
		return nil, err
	}

	return &template, nil
}

func (m *Manager) ListTemplates(ctx context.Context, tenantID uuid.UUID, templateType string) ([]*Template, error) {
	query := `
		SELECT 
			id, tenant_id, name, description, template_type,
			template_config, schedule_config, output_format,
			created_by, created_at, updated_at
		FROM report_templates
		WHERE tenant_id = $1
	`

	args := []any{tenantID}
	argIndex := 2

	if templateType != "" {
		query += " AND template_type = $" + string(rune(argIndex+'0'))
		args = append(args, templateType)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*Template
	for rows.Next() {
		var template Template
		var templateConfigJSON, scheduleConfigJSON []byte

		err := rows.Scan(
			&template.ID, &template.TenantID, &template.Name, &template.Description,
			&template.TemplateType, &templateConfigJSON, &scheduleConfigJSON,
			&template.OutputFormat, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt,
		)
		if err != nil {
			m.logger.WithError(err).Error("Failed to scan template row")
			continue
		}

		if err := json.Unmarshal(templateConfigJSON, &template.TemplateConfig); err != nil {
			m.logger.WithError(err).Error("Failed to unmarshal template config")
			continue
		}

		if err := json.Unmarshal(scheduleConfigJSON, &template.ScheduleConfig); err != nil {
			m.logger.WithError(err).Error("Failed to unmarshal schedule config")
			continue
		}

		templates = append(templates, &template)
	}

	return templates, nil
}

func (m *Manager) UpdateTemplate(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	setClause := ""
	args := []any{}
	argIndex := 1

	for key, value := range updates {
		if setClause != "" {
			setClause += ", "
		}

		switch key {
		case "name":
			setClause += "name = $" + string(rune(argIndex+'0'))
			args = append(args, value)
			argIndex++
		case "description":
			setClause += "description = $" + string(rune(argIndex+'0'))
			args = append(args, value)
			argIndex++
		case "template_config":
			configJSON, err := json.Marshal(value)
			if err != nil {
				return err
			}
			setClause += "template_config = $" + string(rune(argIndex+'0'))
			args = append(args, configJSON)
			argIndex++
		case "schedule_config":
			configJSON, err := json.Marshal(value)
			if err != nil {
				return err
			}
			setClause += "schedule_config = $" + string(rune(argIndex+'0'))
			args = append(args, configJSON)
			argIndex++
		case "output_format":
			setClause += "output_format = $" + string(rune(argIndex+'0'))
			args = append(args, value)
			argIndex++
		}
	}

	if setClause == "" {
		return nil
	}

	args = append(args, id)
	query := "UPDATE report_templates SET " + setClause + " WHERE id = $" + string(rune(argIndex+'0'))

	_, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		m.logger.WithError(err).Error("Failed to update report template")
		return err
	}

	m.logger.WithField("template_id", id).Info("Report template updated")
	return nil
}

func (m *Manager) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM report_templates WHERE id = $1"

	result, err := m.db.ExecContext(ctx, query, id)
	if err != nil {
		m.logger.WithError(err).Error("Failed to delete report template")
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil
	}

	m.logger.WithField("template_id", id).Info("Report template deleted")
	return nil
}

// Predefined report templates
func (m *Manager) CreateDefaultTemplates(ctx context.Context, tenantID uuid.UUID) error {
	defaultTemplates := []*Template{
		{
			ID:           uuid.New(),
			TenantID:     tenantID,
			Name:         "Daily Call Summary",
			Description:  "Summary of all calls for the day",
			TemplateType:  "cdr_summary",
			OutputFormat: "pdf",
			TemplateConfig: map[string]interface{}{
				"columns": []string{"caller_id_number", "destination_number", "start_timestamp", "duration_seconds", "hangup_cause"},
				"filters": map[string]string{
					"date": "today",
				},
				"group_by": []string{"hangup_cause"},
			},
			ScheduleConfig: map[string]interface{}{
				"enabled":   true,
				"frequency": "daily",
				"time":      "08:00",
			},
			CreatedBy: "system",
		},
		{
			ID:           uuid.New(),
			TenantID:     tenantID,
			Name:         "Weekly Billing Report",
			Description:  "Weekly billing summary with call costs",
			TemplateType: "billing",
			OutputFormat: "excel",
			TemplateConfig: map[string]interface{}{
				"columns": []string{"caller_id_number", "destination_number", "duration_seconds", "cost"},
				"filters": map[string]string{
					"period": "last_7_days",
				},
				"aggregations": []string{"sum(cost)", "count(*)"},
			},
			ScheduleConfig: map[string]interface{}{
				"enabled":   true,
				"frequency": "weekly",
				"day":       "monday",
				"time":      "09:00",
			},
			CreatedBy: "system",
		},
		{
			ID:           uuid.New(),
			TenantID:     tenantID,
			Name:         "Call Quality Report",
			Description:  "Quality metrics for all calls",
			TemplateType: "quality",
			OutputFormat: "pdf",
			TemplateConfig: map[string]interface{}{
				"columns": []string{"caller_id_number", "destination_number", "quality_score", "packet_loss", "jitter"},
				"filters": map[string]string{
					"period": "last_7_days",
				},
			},
			ScheduleConfig: map[string]interface{}{
				"enabled": false,
			},
			CreatedBy: "system",
		},
	}

	for _, template := range defaultTemplates {
		if err := m.CreateTemplate(ctx, template); err != nil {
			m.logger.WithError(err).WithField("template", template.Name).Error("Failed to create default template")
		}
	}

	m.logger.WithField("tenant_id", tenantID).Info("Default report templates created")
	return nil
}
