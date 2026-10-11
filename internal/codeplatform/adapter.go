// Package codeplatform holds the real GitLab and GitHub implementations of
// domain.CodePlatformAdapter. Each makes exactly one read-only API call to turn a
// repository path into the platform's stable project ID (ADR-0001).
package codeplatform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// Adapter dispatches on PlatformConn.Kind.
type Adapter struct {
	client *http.Client
}

var _ domain.CodePlatformAdapter = (*Adapter)(nil)

// New returns the production adapter (10 second timeout per call).
// It never follows redirects, so the token cannot leak to another host.
func New() *Adapter {
	return &Adapter{client: &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (a *Adapter) ResolveRepo(ctx context.Context, p domain.PlatformConn, path string) domain.RepoResult {
	switch p.Kind {
	case domain.PlatformGitLab:
		return a.get(ctx, gitlabRequest(p, path))
	case domain.PlatformGitHub:
		owner, repo, ok := strings.Cut(path, "/")
		if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
			return domain.RepoResult{Outcome: domain.RepoNotFound} // GitHub paths are always owner/repo
		}
		return a.get(ctx, githubRequest(p, owner, repo))
	}
	return domain.RepoResult{Outcome: domain.PlatformUnreachable}
}

// apiCall is the one request an adapter makes.
type apiCall struct {
	url     string
	headers map[string]string
	// forbidden reports whether a response means "the token cannot see this repository".
	forbidden func(*http.Response) bool
}

func gitlabRequest(p domain.PlatformConn, path string) apiCall {
	return apiCall{
		// PathEscape turns the slashes of "group/repo" into %2F, as the GitLab API requires.
		url:     strings.TrimRight(p.BaseURL, "/") + "/api/v4/projects/" + url.PathEscape(path),
		headers: map[string]string{"PRIVATE-TOKEN": p.Token},
		// GitLab answers 404, not 403, for a project the token cannot see; a bad token is 401.
		forbidden: func(r *http.Response) bool { return r.StatusCode == http.StatusUnauthorized || r.StatusCode == http.StatusForbidden },
	}
}

func githubRequest(p domain.PlatformConn, owner, repo string) apiCall {
	return apiCall{
		url: githubAPIBase(p.BaseURL) + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo),
		headers: map[string]string{
			"Authorization":        "Bearer " + p.Token,
			"Accept":               "application/vnd.github+json",
			"X-GitHub-Api-Version": "2022-11-28",
		},
		forbidden: func(r *http.Response) bool {
			if r.StatusCode == http.StatusForbidden && r.Header.Get("X-RateLimit-Remaining") == "0" {
				return false // rate limited: not a permission problem, try again later
			}
			return r.StatusCode == http.StatusUnauthorized || r.StatusCode == http.StatusForbidden
		},
	}
}

// githubAPIBase maps the instance address to its REST API root: github.com has a
// dedicated API host, GitHub Enterprise Server serves the API under /api/v3.
func githubAPIBase(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil {
		switch strings.ToLower(u.Hostname()) {
		case "github.com", "www.github.com", "api.github.com":
			return "https://api.github.com"
		}
	}
	return strings.TrimRight(baseURL, "/") + "/api/v3"
}

func (a *Adapter) get(ctx context.Context, call apiCall) domain.RepoResult {
	unreachable := domain.RepoResult{Outcome: domain.PlatformUnreachable}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, call.url, nil)
	if err != nil {
		return unreachable
	}
	for k, v := range call.headers {
		req.Header.Set(k, v)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return unreachable
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusOK:
		var body struct {
			ID json.Number `json:"id"`
		}
		dec := json.NewDecoder(io.LimitReader(res.Body, 1<<20))
		if dec.Decode(&body) != nil || body.ID == "" {
			return unreachable // 200 but not the platform's API (for example a login page)
		}
		if _, err := body.ID.Int64(); err != nil {
			return unreachable
		}
		return domain.RepoResult{Outcome: domain.RepoFound, ProjectID: body.ID.String()}
	case res.StatusCode == http.StatusNotFound:
		return domain.RepoResult{Outcome: domain.RepoNotFound}
	case call.forbidden(res):
		return domain.RepoResult{Outcome: domain.RepoForbidden}
	}
	return unreachable // 5xx, 429, anything unexpected: the answer could not be obtained
}
