package mysqlstore

import (
	"context"
	"strings"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.GlobalVersionRepository = (*Store)(nil)

func (s *Store) ListAllVersions(ctx context.Context, f domain.GlobalVersionFilter, offset, limit int) ([]domain.GlobalVersion, int, error) {
	const from = " FROM app_versions v JOIN app_lines l ON l.id = v.line_id JOIN applications a ON a.id = v.application_id"
	// One WHERE clause for both the count and the page.
	var where []string
	var args []any
	if f.AppKey != "" {
		where = append(where, "a.app_key = ?")
		args = append(args, f.AppKey)
	}
	if f.Search != "" {
		// number is utf8mb4_bin (exact uniqueness); search ignores case but keeps accents apart.
		where = append(where, "v.number COLLATE utf8mb4_0900_as_ci LIKE ? ESCAPE '|'")
		args = append(args, likePattern(f.Search))
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.q.QueryRowContext(ctx, "SELECT COUNT(*)"+from+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.q.QueryContext(ctx,
		`SELECT v.id, v.application_id, v.line_id, v.number, v.ref_kind, v.ref_value, v.released_at, v.note, v.created_at,
  l.id, l.is_main, l.name, l.branch, l.created_at, a.app_key, a.name`+from+cond+
			" ORDER BY v.released_at DESC, v.id DESC LIMIT ? OFFSET ?",
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.GlobalVersion{}
	for rows.Next() {
		var g domain.GlobalVersion
		var kind string
		v := &g.Version
		if err := rows.Scan(&v.ID, &v.ApplicationID, &v.LineID, &v.Number, &kind, &v.RefValue, &v.ReleasedAt, &v.Note, &v.CreatedAt,
			&v.Line.ID, &v.Line.IsMain, &v.Line.Name, &v.Line.Branch, &v.Line.CreatedAt, &g.AppKey, &g.AppName); err != nil {
			return nil, 0, err
		}
		v.RefKind = domain.RefKind(kind)
		out = append(out, g)
	}
	return out, total, rows.Err()
}
