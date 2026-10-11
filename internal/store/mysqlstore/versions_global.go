package mysqlstore

import (
	"context"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.GlobalVersionRepository = (*Store)(nil)

func (s *Store) ListAllVersions(ctx context.Context, f domain.GlobalVersionFilter) ([]domain.GlobalVersion, error) {
	q := `SELECT v.id, v.application_id, v.line_id, v.number, v.ref_kind, v.ref_value, v.released_at, v.note, v.created_at,
  l.id, l.is_main, l.name, l.branch, l.created_at, a.app_key, a.name
FROM app_versions v JOIN app_lines l ON l.id = v.line_id JOIN applications a ON a.id = v.application_id`
	var args []any
	if f.AppKey != "" {
		q += " WHERE a.app_key = ?"
		args = append(args, f.AppKey)
	}
	rows, err := s.q.QueryContext(ctx, q+" ORDER BY v.released_at DESC, v.id DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.GlobalVersion{}
	for rows.Next() {
		var g domain.GlobalVersion
		var kind string
		v := &g.Version
		if err := rows.Scan(&v.ID, &v.ApplicationID, &v.LineID, &v.Number, &kind, &v.RefValue, &v.ReleasedAt, &v.Note, &v.CreatedAt,
			&v.Line.ID, &v.Line.IsMain, &v.Line.Name, &v.Line.Branch, &v.Line.CreatedAt, &g.AppKey, &g.AppName); err != nil {
			return nil, err
		}
		v.RefKind = domain.RefKind(kind)
		out = append(out, g)
	}
	return out, rows.Err()
}
