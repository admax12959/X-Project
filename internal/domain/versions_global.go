package domain

import "context"

// GlobalVersion is a version together with the application it belongs to, as the
// cross-application list shows it.
type GlobalVersion struct {
	Version
	AppKey  string
	AppName string
}

// GlobalVersionFilter narrows the cross-application list. AppKey is exact; an
// unknown key simply matches nothing.
type GlobalVersionFilter struct {
	AppKey string
}

// GlobalVersionRepository is the cross-application version listing part of Repository.
type GlobalVersionRepository interface {
	// ListAllVersions returns the versions of all applications that match the
	// filter, latest release date first, then newest registration first (never
	// ordered by number).
	ListAllVersions(ctx context.Context, f GlobalVersionFilter) ([]GlobalVersion, error)
}

// ListAllVersions returns every application's versions, latest release date first.
// Any logged-in user may read them.
func (s *Service) ListAllVersions(ctx context.Context, _ Actor, f GlobalVersionFilter) ([]GlobalVersion, error) {
	return s.repo.ListAllVersions(ctx, f)
}
