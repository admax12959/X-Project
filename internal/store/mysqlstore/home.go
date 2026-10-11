package mysqlstore

import (
	"context"
	"strings"
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
