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

func (s *Store) UpdateApplicationInfo(ctx context.Context, id int64, name, description string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE applications SET name = ?, description = ? WHERE id = ?", name, description, id)
	return err
}
