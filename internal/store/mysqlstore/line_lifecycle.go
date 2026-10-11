package mysqlstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.LineLifecycleRepository = (*Store)(nil)

func (s *Store) GetLine(ctx context.Context, applicationID, lineID int64) (domain.Line, bool, error) {
	l, err := scanLine(s.q.QueryRowContext(ctx, lineSelect+" WHERE l.application_id = ? AND l.id = ?", applicationID, lineID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Line{}, false, nil
	}
	return l, err == nil, err
}

func (s *Store) SetLineStatus(ctx context.Context, lineID int64, status domain.LineStatus) error {
	_, err := s.q.ExecContext(ctx, "UPDATE app_lines SET status = ? WHERE id = ? AND NOT is_main", string(status), lineID)
	return err
}

func (s *Store) AddLineStatusChange(ctx context.Context, c domain.StoredLineStatusChange) error {
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO app_line_status_changes (line_id, from_status, to_status, changed_at, actor_user_id) VALUES (?, ?, ?, ?, ?)",
		c.LineID, string(c.From), string(c.To), c.At, c.ActorID)
	return err
}

func (s *Store) ListLineStatusChanges(ctx context.Context, lineID int64) ([]domain.StoredLineStatusChange, error) {
	rows, err := s.q.QueryContext(ctx,
		"SELECT line_id, from_status, to_status, changed_at, actor_user_id FROM app_line_status_changes WHERE line_id = ? ORDER BY changed_at, id", lineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.StoredLineStatusChange{}
	for rows.Next() {
		var c domain.StoredLineStatusChange
		var from, to string
		if err := rows.Scan(&c.LineID, &from, &to, &c.At, &c.ActorID); err != nil {
			return nil, err
		}
		c.From, c.To = domain.LineStatus(from), domain.LineStatus(to)
		out = append(out, c)
	}
	return out, rows.Err()
}
