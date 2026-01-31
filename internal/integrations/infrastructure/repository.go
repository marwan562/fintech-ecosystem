package infrastructure

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/marwan562/fintech-ecosystem/internal/integrations/domain"
)

// SQLTemplateRepository implements TemplateRepository using SQL database.
type SQLTemplateRepository struct {
	db *sql.DB
}

// NewSQLTemplateRepository creates a new SQL-based template repository.
func NewSQLTemplateRepository(db *sql.DB) *SQLTemplateRepository {
	return &SQLTemplateRepository{db: db}
}

// ListTemplates returns available integration templates.
func (r *SQLTemplateRepository) ListTemplates(activeOnly bool, category *domain.TemplateCategory) ([]*domain.IntegrationTemplate, error) {
	query := `SELECT id, name, display_name, description, source_platform, category, 
		estimated_days, base_price, currency, requirements, deliverables, is_active, created_at, updated_at
		FROM integration_templates WHERE 1=1`

	args := []interface{}{}
	argNum := 1

	if activeOnly {
		query += fmt.Sprintf(" AND is_active = $%d", argNum)
		args = append(args, true)
		argNum++
	}

	if category != nil {
		query += fmt.Sprintf(" AND category = $%d", argNum)
		args = append(args, *category)
	}

	query += " ORDER BY category, display_name"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var templates []*domain.IntegrationTemplate
	for rows.Next() {
		t, err := r.scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}

	return templates, rows.Err()
}

// GetTemplateByID retrieves a template by ID.
func (r *SQLTemplateRepository) GetTemplateByID(id string) (*domain.IntegrationTemplate, error) {
	query := `SELECT id, name, display_name, description, source_platform, category,
		estimated_days, base_price, currency, requirements, deliverables, is_active, created_at, updated_at
		FROM integration_templates WHERE id = $1`

	row := r.db.QueryRow(query, id)
	return r.scanTemplateRow(row)
}

// GetTemplateByName retrieves a template by name.
func (r *SQLTemplateRepository) GetTemplateByName(name string) (*domain.IntegrationTemplate, error) {
	query := `SELECT id, name, display_name, description, source_platform, category,
		estimated_days, base_price, currency, requirements, deliverables, is_active, created_at, updated_at
		FROM integration_templates WHERE name = $1`

	row := r.db.QueryRow(query, name)
	return r.scanTemplateRow(row)
}

func (r *SQLTemplateRepository) scanTemplate(rows *sql.Rows) (*domain.IntegrationTemplate, error) {
	var t domain.IntegrationTemplate
	var requirements, deliverables []byte
	err := rows.Scan(
		&t.ID, &t.Name, &t.DisplayName, &t.Description, &t.SourcePlatform, &t.Category,
		&t.EstimatedDays, &t.BasePrice, &t.Currency, &requirements, &deliverables, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(requirements, &t.Requirements)
	_ = json.Unmarshal(deliverables, &t.Deliverables)
	return &t, nil
}

func (r *SQLTemplateRepository) scanTemplateRow(row *sql.Row) (*domain.IntegrationTemplate, error) {
	var t domain.IntegrationTemplate
	var requirements, deliverables []byte
	err := row.Scan(
		&t.ID, &t.Name, &t.DisplayName, &t.Description, &t.SourcePlatform, &t.Category,
		&t.EstimatedDays, &t.BasePrice, &t.Currency, &requirements, &deliverables, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(requirements, &t.Requirements)
	_ = json.Unmarshal(deliverables, &t.Deliverables)
	return &t, nil
}

// SQLProjectRepository implements ProjectRepository using SQL database.
type SQLProjectRepository struct {
	db *sql.DB
}

// NewSQLProjectRepository creates a new SQL-based project repository.
func NewSQLProjectRepository(db *sql.DB) *SQLProjectRepository {
	return &SQLProjectRepository{db: db}
}

// CreateProject inserts a new integration project.
func (r *SQLProjectRepository) CreateProject(p *domain.IntegrationProject) error {
	targetConfig, _ := json.Marshal(p.TargetConfig)
	query := `
		INSERT INTO integration_projects (
			id, project_number, org_id, template_id, name, description, source_platform,
			target_config, status, priority, estimated_price, currency, target_completion_date,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err := r.db.Exec(query,
		p.ID, p.ProjectNumber, p.OrgID, p.TemplateID, p.Name, p.Description, p.SourcePlatform,
		targetConfig, p.Status, p.Priority, p.EstimatedPrice, p.Currency, p.TargetCompletionDate,
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// GetProjectByID retrieves a project by ID.
func (r *SQLProjectRepository) GetProjectByID(id string) (*domain.IntegrationProject, error) {
	query := `
		SELECT id, project_number, org_id, template_id, name, description, source_platform,
			target_config, status, priority, estimated_price, final_price, currency,
			assigned_engineer, start_date, target_completion_date, actual_completion_date,
			created_at, updated_at
		FROM integration_projects WHERE id = $1
	`
	return r.scanProject(r.db.QueryRow(query, id))
}

// GetProjectByNumber retrieves a project by its human-readable number.
func (r *SQLProjectRepository) GetProjectByNumber(number string) (*domain.IntegrationProject, error) {
	query := `
		SELECT id, project_number, org_id, template_id, name, description, source_platform,
			target_config, status, priority, estimated_price, final_price, currency,
			assigned_engineer, start_date, target_completion_date, actual_completion_date,
			created_at, updated_at
		FROM integration_projects WHERE project_number = $1
	`
	return r.scanProject(r.db.QueryRow(query, number))
}

// ListProjectsByOrg returns paginated projects for an organization.
func (r *SQLProjectRepository) ListProjectsByOrg(orgID string, status *domain.ProjectStatus, limit, offset int) ([]*domain.IntegrationProject, int, error) {
	var projects []*domain.IntegrationProject
	var total int

	countQuery := "SELECT COUNT(*) FROM integration_projects WHERE org_id = $1"
	listQuery := `
		SELECT id, project_number, org_id, template_id, name, description, source_platform,
			target_config, status, priority, estimated_price, final_price, currency,
			assigned_engineer, start_date, target_completion_date, actual_completion_date,
			created_at, updated_at
		FROM integration_projects WHERE org_id = $1
	`

	args := []interface{}{orgID}
	if status != nil {
		countQuery += " AND status = $2"
		listQuery += " AND status = $2"
		args = append(args, *status)
	}

	listQuery += " ORDER BY created_at DESC LIMIT $" + fmt.Sprintf("%d", len(args)+1) + " OFFSET $" + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	if err := r.db.QueryRow(countQuery, args[:len(args)-2]...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		p, err := r.scanProjectRows(rows)
		if err != nil {
			return nil, 0, err
		}
		projects = append(projects, p)
	}

	return projects, total, rows.Err()
}

// UpdateProject updates an existing project.
func (r *SQLProjectRepository) UpdateProject(p *domain.IntegrationProject) error {
	query := `
		UPDATE integration_projects SET
			status = $2, priority = $3, assigned_engineer = $4, start_date = $5,
			actual_completion_date = $6, final_price = $7, updated_at = $8
		WHERE id = $1
	`
	_, err := r.db.Exec(query,
		p.ID, p.Status, p.Priority, p.AssignedEngineer, p.StartDate,
		p.ActualCompletionDate, p.FinalPrice, time.Now().UTC(),
	)
	return err
}

// CreateMilestone inserts a new milestone.
func (r *SQLProjectRepository) CreateMilestone(m *domain.ProjectMilestone) error {
	deliverables, _ := json.Marshal(m.Deliverables)
	query := `
		INSERT INTO project_milestones (id, project_id, name, description, sequence, status, deliverables, due_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(query,
		m.ID, m.ProjectID, m.Name, m.Description, m.Sequence, m.Status, deliverables, m.DueDate, m.CreatedAt, m.UpdatedAt,
	)
	return err
}

// GetMilestoneByID retrieves a milestone by ID.
func (r *SQLProjectRepository) GetMilestoneByID(id string) (*domain.ProjectMilestone, error) {
	query := `SELECT id, project_id, name, description, sequence, status, deliverables, due_date, completed_at, notes, created_at, updated_at
		FROM project_milestones WHERE id = $1`

	var m domain.ProjectMilestone
	var deliverables []byte
	err := r.db.QueryRow(query, id).Scan(
		&m.ID, &m.ProjectID, &m.Name, &m.Description, &m.Sequence, &m.Status,
		&deliverables, &m.DueDate, &m.CompletedAt, &m.Notes, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(deliverables, &m.Deliverables)
	return &m, nil
}

// ListMilestonesByProject returns milestones for a project.
func (r *SQLProjectRepository) ListMilestonesByProject(projectID string) ([]*domain.ProjectMilestone, error) {
	query := `SELECT id, project_id, name, description, sequence, status, deliverables, due_date, completed_at, notes, created_at, updated_at
		FROM project_milestones WHERE project_id = $1 ORDER BY sequence ASC`

	rows, err := r.db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var milestones []*domain.ProjectMilestone
	for rows.Next() {
		var m domain.ProjectMilestone
		var deliverables []byte
		if err := rows.Scan(
			&m.ID, &m.ProjectID, &m.Name, &m.Description, &m.Sequence, &m.Status,
			&deliverables, &m.DueDate, &m.CompletedAt, &m.Notes, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(deliverables, &m.Deliverables)
		milestones = append(milestones, &m)
	}

	return milestones, rows.Err()
}

// UpdateMilestone updates a milestone.
func (r *SQLProjectRepository) UpdateMilestone(m *domain.ProjectMilestone) error {
	query := `UPDATE project_milestones SET status = $2, notes = $3, completed_at = $4, updated_at = $5 WHERE id = $1`
	_, err := r.db.Exec(query, m.ID, m.Status, m.Notes, m.CompletedAt, time.Now().UTC())
	return err
}

// CreateUpdate adds a project update.
func (r *SQLProjectRepository) CreateUpdate(u *domain.ProjectUpdate) error {
	query := `INSERT INTO project_updates (id, project_id, author, update_type, title, content, is_customer_visible, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.Exec(query, u.ID, u.ProjectID, u.Author, u.UpdateType, u.Title, u.Content, u.IsCustomerVisible, u.CreatedAt)
	return err
}

// ListUpdatesByProject returns updates for a project.
func (r *SQLProjectRepository) ListUpdatesByProject(projectID string, customerVisibleOnly bool) ([]*domain.ProjectUpdate, error) {
	query := `SELECT id, project_id, author, update_type, title, content, is_customer_visible, created_at
		FROM project_updates WHERE project_id = $1`
	if customerVisibleOnly {
		query += " AND is_customer_visible = true"
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var updates []*domain.ProjectUpdate
	for rows.Next() {
		var u domain.ProjectUpdate
		if err := rows.Scan(&u.ID, &u.ProjectID, &u.Author, &u.UpdateType, &u.Title, &u.Content, &u.IsCustomerVisible, &u.CreatedAt); err != nil {
			return nil, err
		}
		updates = append(updates, &u)
	}

	return updates, rows.Err()
}

func (r *SQLProjectRepository) scanProject(row *sql.Row) (*domain.IntegrationProject, error) {
	var p domain.IntegrationProject
	var targetConfig []byte
	err := row.Scan(
		&p.ID, &p.ProjectNumber, &p.OrgID, &p.TemplateID, &p.Name, &p.Description, &p.SourcePlatform,
		&targetConfig, &p.Status, &p.Priority, &p.EstimatedPrice, &p.FinalPrice, &p.Currency,
		&p.AssignedEngineer, &p.StartDate, &p.TargetCompletionDate, &p.ActualCompletionDate,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(targetConfig, &p.TargetConfig)
	return &p, nil
}

func (r *SQLProjectRepository) scanProjectRows(rows *sql.Rows) (*domain.IntegrationProject, error) {
	var p domain.IntegrationProject
	var targetConfig []byte
	err := rows.Scan(
		&p.ID, &p.ProjectNumber, &p.OrgID, &p.TemplateID, &p.Name, &p.Description, &p.SourcePlatform,
		&targetConfig, &p.Status, &p.Priority, &p.EstimatedPrice, &p.FinalPrice, &p.Currency,
		&p.AssignedEngineer, &p.StartDate, &p.TargetCompletionDate, &p.ActualCompletionDate,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(targetConfig, &p.TargetConfig)
	return &p, nil
}

// SQLConsultationRepository implements ConsultationRepository using SQL database.
type SQLConsultationRepository struct {
	db *sql.DB
}

// NewSQLConsultationRepository creates a new SQL-based consultation repository.
func NewSQLConsultationRepository(db *sql.DB) *SQLConsultationRepository {
	return &SQLConsultationRepository{db: db}
}

// CreateConsultation inserts a new consultation request.
func (r *SQLConsultationRepository) CreateConsultation(c *domain.Consultation) error {
	query := `
		INSERT INTO integration_consultations (
			id, org_id, contact_name, contact_email, contact_phone, company_name,
			project_type, current_platform, monthly_volume, requirements, preferred_timeline,
			status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.db.Exec(query,
		c.ID, c.OrgID, c.ContactName, c.ContactEmail, c.ContactPhone, c.CompanyName,
		c.ProjectType, c.CurrentPlatform, c.MonthlyVolume, c.Requirements, c.PreferredTimeline,
		c.Status, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// GetConsultationByID retrieves a consultation by ID.
func (r *SQLConsultationRepository) GetConsultationByID(id string) (*domain.Consultation, error) {
	query := `
		SELECT id, org_id, contact_name, contact_email, contact_phone, company_name,
			project_type, current_platform, monthly_volume, requirements, preferred_timeline,
			status, scheduled_at, completed_at, notes, assigned_to, created_at, updated_at
		FROM integration_consultations WHERE id = $1
	`
	return r.scanConsultation(r.db.QueryRow(query, id))
}

// ListConsultationsByOrg returns consultations for an organization.
func (r *SQLConsultationRepository) ListConsultationsByOrg(orgID string) ([]*domain.Consultation, error) {
	query := `
		SELECT id, org_id, contact_name, contact_email, contact_phone, company_name,
			project_type, current_platform, monthly_volume, requirements, preferred_timeline,
			status, scheduled_at, completed_at, notes, assigned_to, created_at, updated_at
		FROM integration_consultations WHERE org_id = $1 ORDER BY created_at DESC
	`
	return r.scanConsultations(r.db.Query(query, orgID))
}

// ListPendingConsultations returns all pending consultation requests.
func (r *SQLConsultationRepository) ListPendingConsultations() ([]*domain.Consultation, error) {
	query := `
		SELECT id, org_id, contact_name, contact_email, contact_phone, company_name,
			project_type, current_platform, monthly_volume, requirements, preferred_timeline,
			status, scheduled_at, completed_at, notes, assigned_to, created_at, updated_at
		FROM integration_consultations WHERE status = 'pending' ORDER BY created_at ASC
	`
	return r.scanConsultations(r.db.Query(query))
}

// UpdateConsultation updates an existing consultation.
func (r *SQLConsultationRepository) UpdateConsultation(c *domain.Consultation) error {
	query := `
		UPDATE integration_consultations SET
			status = $2, scheduled_at = $3, completed_at = $4, notes = $5, assigned_to = $6, updated_at = $7
		WHERE id = $1
	`
	_, err := r.db.Exec(query, c.ID, c.Status, c.ScheduledAt, c.CompletedAt, c.Notes, c.AssignedTo, time.Now().UTC())
	return err
}

func (r *SQLConsultationRepository) scanConsultation(row *sql.Row) (*domain.Consultation, error) {
	var c domain.Consultation
	err := row.Scan(
		&c.ID, &c.OrgID, &c.ContactName, &c.ContactEmail, &c.ContactPhone, &c.CompanyName,
		&c.ProjectType, &c.CurrentPlatform, &c.MonthlyVolume, &c.Requirements, &c.PreferredTimeline,
		&c.Status, &c.ScheduledAt, &c.CompletedAt, &c.Notes, &c.AssignedTo, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *SQLConsultationRepository) scanConsultations(rows *sql.Rows, err error) ([]*domain.Consultation, error) {
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var consultations []*domain.Consultation
	for rows.Next() {
		var c domain.Consultation
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.ContactName, &c.ContactEmail, &c.ContactPhone, &c.CompanyName,
			&c.ProjectType, &c.CurrentPlatform, &c.MonthlyVolume, &c.Requirements, &c.PreferredTimeline,
			&c.Status, &c.ScheduledAt, &c.CompletedAt, &c.Notes, &c.AssignedTo, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		consultations = append(consultations, &c)
	}

	return consultations, rows.Err()
}
