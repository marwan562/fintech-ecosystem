package infrastructure

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/marwan562/fintech-ecosystem/internal/support/domain"
)

// SQLTicketRepository implements TicketRepository using SQL database.
type SQLTicketRepository struct {
	db *sql.DB
}

// NewSQLTicketRepository creates a new SQL-based ticket repository.
func NewSQLTicketRepository(db *sql.DB) *SQLTicketRepository {
	return &SQLTicketRepository{db: db}
}

// CreateTicket inserts a new support ticket.
func (r *SQLTicketRepository) CreateTicket(t *domain.SupportTicket) error {
	query := `
		INSERT INTO support_tickets (
			id, ticket_number, org_id, contract_id, requester_email, requester_name,
			subject, description, priority, status, category, assigned_to,
			sla_first_response_at, sla_resolution_at, sla_breached, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`
	_, err := r.db.Exec(query,
		t.ID, t.TicketNumber, t.OrgID, t.ContractID, t.RequesterEmail, t.RequesterName,
		t.Subject, t.Description, t.Priority, t.Status, t.Category, t.AssignedTo,
		t.SLAFirstResponseAt, t.SLAResolutionAt, t.SLABreached, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

// GetTicketByID retrieves a ticket by its ID.
func (r *SQLTicketRepository) GetTicketByID(id string) (*domain.SupportTicket, error) {
	query := `
		SELECT id, ticket_number, org_id, contract_id, requester_email, requester_name,
			subject, description, priority, status, category, assigned_to,
			sla_first_response_at, sla_resolution_at, first_responded_at, resolved_at,
			sla_breached, created_at, updated_at
		FROM support_tickets WHERE id = $1
	`
	return r.scanTicket(r.db.QueryRow(query, id))
}

// GetTicketByNumber retrieves a ticket by its human-readable number.
func (r *SQLTicketRepository) GetTicketByNumber(number string) (*domain.SupportTicket, error) {
	query := `
		SELECT id, ticket_number, org_id, contract_id, requester_email, requester_name,
			subject, description, priority, status, category, assigned_to,
			sla_first_response_at, sla_resolution_at, first_responded_at, resolved_at,
			sla_breached, created_at, updated_at
		FROM support_tickets WHERE ticket_number = $1
	`
	return r.scanTicket(r.db.QueryRow(query, number))
}

// ListTicketsByOrg returns paginated tickets for an organization.
func (r *SQLTicketRepository) ListTicketsByOrg(orgID string, status *domain.TicketStatus, limit, offset int) ([]*domain.SupportTicket, int, error) {
	var tickets []*domain.SupportTicket
	var total int

	countQuery := "SELECT COUNT(*) FROM support_tickets WHERE org_id = $1"
	listQuery := `
		SELECT id, ticket_number, org_id, contract_id, requester_email, requester_name,
			subject, description, priority, status, category, assigned_to,
			sla_first_response_at, sla_resolution_at, first_responded_at, resolved_at,
			sla_breached, created_at, updated_at
		FROM support_tickets WHERE org_id = $1
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
		t, err := r.scanTicketRows(rows)
		if err != nil {
			return nil, 0, err
		}
		tickets = append(tickets, t)
	}

	return tickets, total, rows.Err()
}

// UpdateTicket updates an existing ticket.
func (r *SQLTicketRepository) UpdateTicket(t *domain.SupportTicket) error {
	query := `
		UPDATE support_tickets SET
			priority = $2, status = $3, assigned_to = $4,
			first_responded_at = $5, resolved_at = $6, sla_breached = $7, updated_at = $8
		WHERE id = $1
	`
	_, err := r.db.Exec(query,
		t.ID, t.Priority, t.Status, t.AssignedTo,
		t.FirstRespondedAt, t.ResolvedAt, t.SLABreached, t.UpdatedAt,
	)
	return err
}

// AddComment adds a comment to a ticket.
func (r *SQLTicketRepository) AddComment(c *domain.TicketComment) error {
	attachments, _ := json.Marshal(c.Attachments)
	query := `
		INSERT INTO support_ticket_comments (id, ticket_id, author_email, author_name, is_internal, is_staff, content, attachments, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(query,
		c.ID, c.TicketID, c.AuthorEmail, c.AuthorName, c.IsInternal, c.IsStaff, c.Content, attachments, c.CreatedAt,
	)
	return err
}

// ListCommentsByTicket returns all comments for a ticket.
func (r *SQLTicketRepository) ListCommentsByTicket(ticketID string) ([]*domain.TicketComment, error) {
	query := `
		SELECT id, ticket_id, author_email, author_name, is_internal, is_staff, content, attachments, created_at
		FROM support_ticket_comments WHERE ticket_id = $1 ORDER BY created_at ASC
	`
	rows, err := r.db.Query(query, ticketID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var comments []*domain.TicketComment
	for rows.Next() {
		var c domain.TicketComment
		var attachments []byte
		if err := rows.Scan(&c.ID, &c.TicketID, &c.AuthorEmail, &c.AuthorName, &c.IsInternal, &c.IsStaff, &c.Content, &attachments, &c.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(attachments, &c.Attachments)
		comments = append(comments, &c)
	}

	return comments, rows.Err()
}

// AddEscalation records a ticket escalation.
func (r *SQLTicketRepository) AddEscalation(e *domain.Escalation) error {
	query := `
		INSERT INTO support_escalations (id, ticket_id, escalated_by, escalated_to, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(query, e.ID, e.TicketID, e.EscalatedBy, e.EscalatedTo, e.Reason, e.CreatedAt)
	return err
}

func (r *SQLTicketRepository) scanTicket(row *sql.Row) (*domain.SupportTicket, error) {
	var t domain.SupportTicket
	err := row.Scan(
		&t.ID, &t.TicketNumber, &t.OrgID, &t.ContractID, &t.RequesterEmail, &t.RequesterName,
		&t.Subject, &t.Description, &t.Priority, &t.Status, &t.Category, &t.AssignedTo,
		&t.SLAFirstResponseAt, &t.SLAResolutionAt, &t.FirstRespondedAt, &t.ResolvedAt,
		&t.SLABreached, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *SQLTicketRepository) scanTicketRows(rows *sql.Rows) (*domain.SupportTicket, error) {
	var t domain.SupportTicket
	err := rows.Scan(
		&t.ID, &t.TicketNumber, &t.OrgID, &t.ContractID, &t.RequesterEmail, &t.RequesterName,
		&t.Subject, &t.Description, &t.Priority, &t.Status, &t.Category, &t.AssignedTo,
		&t.SLAFirstResponseAt, &t.SLAResolutionAt, &t.FirstRespondedAt, &t.ResolvedAt,
		&t.SLABreached, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// SQLTierRepository implements TierRepository using SQL database.
type SQLTierRepository struct {
	db *sql.DB
}

// NewSQLTierRepository creates a new SQL-based tier repository.
func NewSQLTierRepository(db *sql.DB) *SQLTierRepository {
	return &SQLTierRepository{db: db}
}

// ListTiers returns available support tiers.
func (r *SQLTierRepository) ListTiers(activeOnly bool) ([]*domain.SupportTier, error) {
	query := "SELECT id, name, display_name, description, price_monthly, price_yearly, currency, features, is_active, sort_order, created_at, updated_at FROM support_tiers"
	if activeOnly {
		query += " WHERE is_active = true"
	}
	query += " ORDER BY sort_order ASC"

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tiers []*domain.SupportTier
	for rows.Next() {
		t, err := r.scanTier(rows)
		if err != nil {
			return nil, err
		}
		tiers = append(tiers, t)
	}

	return tiers, rows.Err()
}

// GetTierByID retrieves a tier by ID.
func (r *SQLTierRepository) GetTierByID(id string) (*domain.SupportTier, error) {
	query := "SELECT id, name, display_name, description, price_monthly, price_yearly, currency, features, is_active, sort_order, created_at, updated_at FROM support_tiers WHERE id = $1"
	row := r.db.QueryRow(query, id)

	var t domain.SupportTier
	var features []byte
	err := row.Scan(&t.ID, &t.Name, &t.DisplayName, &t.Description, &t.PriceMonthly, &t.PriceYearly, &t.Currency, &features, &t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(features, &t.Features)
	return &t, nil
}

// GetTierByName retrieves a tier by name.
func (r *SQLTierRepository) GetTierByName(name string) (*domain.SupportTier, error) {
	query := "SELECT id, name, display_name, description, price_monthly, price_yearly, currency, features, is_active, sort_order, created_at, updated_at FROM support_tiers WHERE name = $1"
	row := r.db.QueryRow(query, name)

	var t domain.SupportTier
	var features []byte
	err := row.Scan(&t.ID, &t.Name, &t.DisplayName, &t.Description, &t.PriceMonthly, &t.PriceYearly, &t.Currency, &features, &t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(features, &t.Features)
	return &t, nil
}

// GetSLADefinitions returns SLA definitions for a tier.
func (r *SQLTierRepository) GetSLADefinitions(tierID string) ([]*domain.SLADefinition, error) {
	query := "SELECT id, tier_id, priority, first_response_minutes, resolution_target_minutes, uptime_percentage, created_at FROM sla_definitions WHERE tier_id = $1"
	rows, err := r.db.Query(query, tierID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var slas []*domain.SLADefinition
	for rows.Next() {
		var s domain.SLADefinition
		if err := rows.Scan(&s.ID, &s.TierID, &s.Priority, &s.FirstResponseMinutes, &s.ResolutionTargetMinutes, &s.UptimePercentage, &s.CreatedAt); err != nil {
			return nil, err
		}
		slas = append(slas, &s)
	}

	return slas, rows.Err()
}

func (r *SQLTierRepository) scanTier(rows *sql.Rows) (*domain.SupportTier, error) {
	var t domain.SupportTier
	var features []byte
	err := rows.Scan(&t.ID, &t.Name, &t.DisplayName, &t.Description, &t.PriceMonthly, &t.PriceYearly, &t.Currency, &features, &t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(features, &t.Features)
	return &t, nil
}

// SQLContractRepository implements ContractRepository using SQL database.
type SQLContractRepository struct {
	db *sql.DB
}

// NewSQLContractRepository creates a new SQL-based contract repository.
func NewSQLContractRepository(db *sql.DB) *SQLContractRepository {
	return &SQLContractRepository{db: db}
}

// CreateContract inserts a new support contract.
func (r *SQLContractRepository) CreateContract(c *domain.SupportContract) error {
	query := `
		INSERT INTO support_contracts (id, org_id, tier_id, status, billing_cycle, start_date, end_date, auto_renew, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(query, c.ID, c.OrgID, c.TierID, c.Status, c.BillingCycle, c.StartDate, c.EndDate, c.AutoRenew, c.CreatedAt, c.UpdatedAt)
	return err
}

// GetContractByID retrieves a contract by ID.
func (r *SQLContractRepository) GetContractByID(id string) (*domain.SupportContract, error) {
	query := "SELECT id, org_id, tier_id, status, billing_cycle, start_date, end_date, auto_renew, created_at, updated_at FROM support_contracts WHERE id = $1"
	return r.scanContract(r.db.QueryRow(query, id))
}

// GetActiveContractByOrg retrieves the active contract for an organization.
func (r *SQLContractRepository) GetActiveContractByOrg(orgID string) (*domain.SupportContract, error) {
	query := "SELECT id, org_id, tier_id, status, billing_cycle, start_date, end_date, auto_renew, created_at, updated_at FROM support_contracts WHERE org_id = $1 AND status = 'active' ORDER BY created_at DESC LIMIT 1"
	return r.scanContract(r.db.QueryRow(query, orgID))
}

// UpdateContract updates an existing contract.
func (r *SQLContractRepository) UpdateContract(c *domain.SupportContract) error {
	query := "UPDATE support_contracts SET status = $2, auto_renew = $3, updated_at = $4 WHERE id = $1"
	_, err := r.db.Exec(query, c.ID, c.Status, c.AutoRenew, time.Now().UTC())
	return err
}

// ListContractsByOrg returns all contracts for an organization.
func (r *SQLContractRepository) ListContractsByOrg(orgID string) ([]*domain.SupportContract, error) {
	query := "SELECT id, org_id, tier_id, status, billing_cycle, start_date, end_date, auto_renew, created_at, updated_at FROM support_contracts WHERE org_id = $1 ORDER BY created_at DESC"
	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var contracts []*domain.SupportContract
	for rows.Next() {
		c, err := r.scanContractRows(rows)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, c)
	}

	return contracts, rows.Err()
}

func (r *SQLContractRepository) scanContract(row *sql.Row) (*domain.SupportContract, error) {
	var c domain.SupportContract
	err := row.Scan(&c.ID, &c.OrgID, &c.TierID, &c.Status, &c.BillingCycle, &c.StartDate, &c.EndDate, &c.AutoRenew, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *SQLContractRepository) scanContractRows(rows *sql.Rows) (*domain.SupportContract, error) {
	var c domain.SupportContract
	err := rows.Scan(&c.ID, &c.OrgID, &c.TierID, &c.Status, &c.BillingCycle, &c.StartDate, &c.EndDate, &c.AutoRenew, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
