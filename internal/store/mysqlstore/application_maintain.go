package mysqlstore

import (
	"context"
	"strings"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.ApplicationMaintenanceRepository = (*Store)(nil)

func (s *Store) LockApplication(ctx context.Context, id int64) error {
	var got int64
	return s.q.QueryRowContext(ctx, "SELECT id FROM applications WHERE id = ? FOR UPDATE", id).Scan(&got)
}

func (s *Store) LockOwnedApplications(ctx context.Context, userID domain.UserID) ([]domain.OwnedApplication, error) {
	rows, err := s.q.QueryContext(ctx,
		`SELECT a.id, a.app_key, a.name FROM application_members m JOIN applications a ON a.id = m.application_id
WHERE m.user_id = ? AND m.role = 'owner' ORDER BY a.app_key FOR UPDATE`, userID)
	if err != nil {
		return nil, err
	}
	var out []domain.OwnedApplication
	index := map[int64]int{}
	var args []any
	for rows.Next() {
		var id int64
		var a domain.OwnedApplication
		if err := rows.Scan(&id, &a.Key, &a.Name); err != nil {
			rows.Close()
			return nil, err
		}
		index[id] = len(out)
		out = append(out, a)
		args = append(args, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(out) == 0 {
		return out, err
	}
	// Shared locks: a concurrent change of the other Owners' status waits for us (or the reverse).
	others, err := s.q.QueryContext(ctx,
		`SELECT m.application_id, u.status FROM application_members m JOIN users u ON u.id = m.user_id
WHERE m.role = 'owner' AND m.user_id <> ? AND m.application_id IN (?`+strings.Repeat(",?", len(args)-1)+`) FOR SHARE`,
		append([]any{userID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer others.Close()
	for others.Next() {
		var appID int64
		var status string
		if err := others.Scan(&appID, &status); err != nil {
			return nil, err
		}
		if domain.UserStatus(status) == domain.UserEnabled {
			out[index[appID]].OtherEnabledOwners++
		}
	}
	return out, others.Err()
}

func (s *Store) DeleteApplication(ctx context.Context, id int64) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM applications WHERE id = ?", id)
	return err
}

func (s *Store) SetDeployInfo(ctx context.Context, id int64, text string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE applications SET deploy_info = ? WHERE id = ?", text, id)
	return err
}

func (s *Store) SetMainBranch(ctx context.Context, applicationID int64, branch string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE app_lines SET branch = ? WHERE application_id = ? AND is_main", branch, applicationID)
	if isDuplicate(err) {
		return domain.ErrBranchTaken // a maintenance line already has this branch
	}
	return err
}

func (s *Store) UpdateApplicationInfo(ctx context.Context, id int64, name, description string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE applications SET name = ?, description = ? WHERE id = ?", name, description, id)
	return err
}
