package domain

import "context"

// Recent activity (home page, #12): the latest change-history events across all
// applications. Everyone sees all applications, so everyone sees all events.

// RecentEvent is a stored change event together with the application it belongs to.
type RecentEvent struct {
	StoredChangeEvent
	AppKey  string
	AppName string
}

// ActivityRepository is the recent-activity part of Repository.
type ActivityRepository interface {
	// ListRecentChangeEvents returns the newest events of all applications, newest
	// first (occurred_at, then ID), at most limit.
	ListRecentChangeEvents(ctx context.Context, limit int) ([]RecentEvent, error)
}

const (
	defaultActivityLimit = 8
	maxActivityLimit     = 50
)

// Activity is a change event ready for display, with its application.
type Activity struct {
	ChangeEvent
	AppKey  string
	AppName string
}

// ListRecentActivity returns the latest change-history events of every application,
// newest first. limit is the raw query string (empty = 8; 1 to 50). Any logged-in
// user may read it.
func (s *Service) ListRecentActivity(ctx context.Context, _ Actor, limit string) ([]Activity, error) {
	var c Collector
	n := positiveInt(&c, "limit", limit, defaultActivityLimit, maxActivityLimit)
	if err := c.Err(); err != nil {
		return nil, err
	}
	stored, err := s.repo.ListRecentChangeEvents(ctx, n)
	if err != nil {
		return nil, err
	}
	users := userResolver{repo: s.repo, cache: map[UserID]UserRef{}}
	out := make([]Activity, 0, len(stored))
	for _, e := range stored {
		ev, err := users.changeEvent(ctx, e.StoredChangeEvent)
		if err != nil {
			return nil, err
		}
		out = append(out, Activity{ChangeEvent: ev, AppKey: e.AppKey, AppName: e.AppName})
	}
	return out, nil
}
