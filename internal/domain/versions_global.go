package domain

import (
	"context"
	"strconv"
	"strings"
)

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
	// Search is a case-insensitive substring of the version number; % and _ are
	// taken literally. Empty matches everything.
	Search string
}

// GlobalVersionRepository is the cross-application version listing part of Repository.
type GlobalVersionRepository interface {
	// ListAllVersions returns one page (offset, limit) of the versions of all
	// applications that match the filter, latest release date first, then newest
	// registration first (never ordered by number), and the number of matches in all pages.
	ListAllVersions(ctx context.Context, f GlobalVersionFilter, offset, limit int) (items []GlobalVersion, total int, err error)
}

const (
	defaultVersionPageSize = 20
	maxVersionPageSize     = 100
)

// GlobalVersionQuery is the request as sent: Page and PageSize are the raw query
// strings (empty = default: page 1, 20 per page; at most 100 per page).
type GlobalVersionQuery struct {
	GlobalVersionFilter
	Page     string
	PageSize string
}

// GlobalVersionPage is one page of the list. Total counts all matches.
type GlobalVersionPage struct {
	Items    []GlobalVersion
	Total    int
	Page     int
	PageSize int
}

// positiveInt parses an optional positive integer parameter no larger than max.
func positiveInt(c *Collector, field, raw string, def, max int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > max {
		c.Add(field, CodeInvalid, nil)
		return def
	}
	return n
}

// ListAllVersions returns one page of every application's versions, latest release
// date first. Any logged-in user may read them. A page past the end is empty.
func (s *Service) ListAllVersions(ctx context.Context, _ Actor, q GlobalVersionQuery) (GlobalVersionPage, error) {
	var c Collector
	page := positiveInt(&c, "page", q.Page, 1, 1<<31-1)
	size := positiveInt(&c, "pageSize", q.PageSize, defaultVersionPageSize, maxVersionPageSize)
	if err := c.Err(); err != nil {
		return GlobalVersionPage{}, err
	}
	q.Search = strings.TrimSpace(q.Search)
	items, total, err := s.repo.ListAllVersions(ctx, q.GlobalVersionFilter, (page-1)*size, size)
	if err != nil {
		return GlobalVersionPage{}, err
	}
	return GlobalVersionPage{Items: items, Total: total, Page: page, PageSize: size}, nil
}
