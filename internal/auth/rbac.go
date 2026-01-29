package auth

import (
	"context"
	"fmt"
)

// Roles defined for RBAC
const (
	RoleOwner     = "owner"
	RoleAdmin     = "admin"
	RoleMember    = "member"
	RoleDeveloper = "developer"
)

func (r *Repository) RemoveMember(ctx context.Context, userID, orgID string) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM memberships WHERE user_id = $1 AND org_id = $2",
		userID, orgID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("membership not found")
	}
	return nil
}

func (r *Repository) UpdateMemberRole(ctx context.Context, userID, orgID, role string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE memberships SET role = $1 WHERE user_id = $2 AND org_id = $3",
		role, userID, orgID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("membership not found")
	}
	return nil
}

func (r *Repository) ListOrgMembers(ctx context.Context, orgID string) ([]Membership, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT user_id, org_id, role, created_at FROM memberships WHERE org_id = $1",
		orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var memberships []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.UserID, &m.OrgID, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, nil
}

func (r *Repository) HasPermission(ctx context.Context, userID, orgID, requiredRole string) (bool, error) {
	var role string
	err := r.db.QueryRowContext(ctx,
		"SELECT role FROM memberships WHERE user_id = $1 AND org_id = $2",
		userID, orgID).Scan(&role)

	if err != nil {
		return false, err
	}

	// Simple hierarchy check: owner > admin > developer > member
	roles := map[string]int{
		RoleOwner:     4,
		RoleAdmin:     3,
		RoleDeveloper: 2,
		RoleMember:    1,
	}

	return roles[role] >= roles[requiredRole], nil
}
