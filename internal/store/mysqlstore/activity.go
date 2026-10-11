package mysqlstore

import (
	"context"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.ActivityRepository = (*Store)(nil)

func (s *Store) ListRecentChangeEvents(ctx context.Context, limit int) ([]domain.RecentEvent, error) {
	rows, err := s.q.QueryContext(ctx,
		`SELECT e.id, e.application_id, e.occurred_at, e.actor_user_id, e.event_type, e.params, a.app_key, a.name
FROM change_events e JOIN applications a ON a.id = e.application_id
ORDER BY e.occurred_at DESC, e.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RecentEvent{}
	for rows.Next() {
		var e domain.RecentEvent
		var typ string
		if err := rows.Scan(&e.ID, &e.ApplicationID, &e.At, &e.ActorID, &typ, &e.Params, &e.AppKey, &e.AppName); err != nil {
			return nil, err
		}
		e.Type = domain.EventType(typ)
		out = append(out, e)
	}
	return out, rows.Err()
}
