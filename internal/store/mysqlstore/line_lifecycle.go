package mysqlstore

import (
	"context"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.LineLifecycleRepository = (*Store)(nil)

func (s *Store) SetLineStatus(ctx context.Context, lineID int64, status domain.LineStatus) error {
	_, err := s.q.ExecContext(ctx, "UPDATE app_lines SET status = ? WHERE id = ? AND NOT is_main", string(status), lineID)
	return err
}
