package mysqlstore

import (
	"context"
	"strings"

	"github.com/admax12959/X-Project/internal/domain"
)

// inList returns "(?,?,...)" and its arguments for an IN clause. ids must not be empty.
func inList(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return "(?" + strings.Repeat(",?", len(ids)-1) + ")", args
}

// lineCounts returns the number of maintenance lines (any status, mainline excluded)
// per application. Applications without any are absent from the map.
func (s *Store) lineCounts(ctx context.Context, appIDs []int64) (map[int64]int, error) {
	out := map[int64]int{}
	if len(appIDs) == 0 {
		return out, nil
	}
	in, args := inList(appIDs)
	rows, err := s.q.QueryContext(ctx,
		"SELECT application_id, COUNT(*) FROM app_lines WHERE NOT is_main AND application_id IN "+in+" GROUP BY application_id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// latestVersions returns, per application, the version with the newest release date
// on any line (ties: registered last), never by number. Only ID, Number, ReleasedAt
// and the short Line are filled. Applications without versions are absent.
func (s *Store) latestVersions(ctx context.Context, appIDs []int64) (map[int64]*domain.Version, error) {
	out := map[int64]*domain.Version{}
	if len(appIDs) == 0 {
		return out, nil
	}
	in, args := inList(appIDs)
	rows, err := s.q.QueryContext(ctx, `SELECT application_id, id, number, released_at, line_id, is_main, line_name, branch FROM (
  SELECT v.application_id, v.id, v.number, v.released_at, l.id AS line_id, l.is_main, l.name AS line_name, l.branch,
    ROW_NUMBER() OVER (PARTITION BY v.application_id ORDER BY v.released_at DESC, v.id DESC) AS rn
  FROM app_versions v JOIN app_lines l ON l.id = v.line_id WHERE v.application_id IN `+in+`) ranked WHERE rn = 1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var appID int64
		v := &domain.Version{}
		if err := rows.Scan(&appID, &v.ID, &v.Number, &v.ReleasedAt, &v.Line.ID, &v.Line.IsMain, &v.Line.Name, &v.Line.Branch); err != nil {
			return nil, err
		}
		v.LineID = v.Line.ID
		out[appID] = v
	}
	return out, rows.Err()
}
