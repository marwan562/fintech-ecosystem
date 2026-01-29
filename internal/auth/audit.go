package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type AuditLog struct {
	ID           string          `json:"id"`
	OrgID        string          `json:"org_id"`
	UserID       string          `json:"user_id"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Metadata     json.RawMessage `json:"metadata"`
	IPAddress    string          `json:"ip_address"`
	UserAgent    string          `json:"user_agent"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (r *Repository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs (org_id, user_id, action, resource_type, resource_id, metadata, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		log.OrgID, log.UserID, log.Action, log.ResourceType, log.ResourceID, log.Metadata, log.IPAddress, log.UserAgent)

	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}

func (r *Repository) GetAuditLogs(ctx context.Context, orgID string, limit, offset int, action string) ([]AuditLog, int, error) {
	query := `SELECT id, org_id, user_id, action, resource_type, resource_id, metadata, ip_address, created_at 
			  FROM audit_logs WHERE org_id = $1`
	args := []interface{}{orgID}
	placeholder := 2

	if action != "" {
		query += fmt.Sprintf(" AND action = $%d", placeholder)
		args = append(args, action)
		placeholder++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", placeholder, placeholder+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.OrgID, &l.UserID, &l.Action, &l.ResourceType, &l.ResourceID, &l.Metadata, &l.IPAddress, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs WHERE org_id = $1"
	countArgs := []interface{}{orgID}
	if action != "" {
		countQuery += " AND action = $2"
		countArgs = append(countArgs, action)
	}
	err = r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
