package mysqlstore

import (
	"context"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.ApplicationMaintenanceRepository = (*Store)(nil)

func (s *Store) LockApplication(ctx context.Context, id int64) error {
	var got int64
	return s.q.QueryRowContext(ctx, "SELECT id FROM applications WHERE id = ? FOR UPDATE", id).Scan(&got)
}

func (s *Store) SetDeployInfo(ctx context.Context, id int64, text string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE applications SET deploy_info = ? WHERE id = ?", text, id)
	return err
}

func (s *Store) SetMainBranch(ctx context.Context, applicationID int64, branch string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE app_lines SET branch = ? WHERE application_id = ? AND is_main", branch, applicationID)
	return err
}

func (s *Store) UpdateApplicationInfo(ctx context.Context, id int64, name, description string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE applications SET name = ?, description = ? WHERE id = ?", name, description, id)
	return err
}
