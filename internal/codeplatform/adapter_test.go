package codeplatform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/admax12959/X-Project/internal/domain"
)

// These tests run the real adapters against local httptest servers that play the
// platform's API. Contract tests against a real GitLab/GitHub are not part of the
// default suite.

func resolve(t *testing.T, kind domain.PlatformKind, baseURL, path string) domain.RepoResult {
	t.Helper()
	return New().ResolveRepo(context.Background(), domain.PlatformConn{Kind: kind, BaseURL: baseURL, Token: "tok-123"}, path)
}

func TestGitLabResolvesAProjectIDFromTheEncodedPath(t *testing.T) {
	t.Parallel()
	var uri, token string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri, token = r.RequestURI, r.Header.Get("PRIVATE-TOKEN")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 9007199254740993, "path_with_namespace": "payments/payments-core"}`))
	}))
	defer srv.Close()

	got := resolve(t, domain.PlatformGitLab, srv.URL+"/gitlab", "payments/payments-core")
	if got.Outcome != domain.RepoFound || got.ProjectID != "9007199254740993" {
		t.Fatalf("got %+v", got)
	}
	if uri != "/gitlab/api/v4/projects/payments%2Fpayments-core" {
		t.Fatalf("request URI = %q, want the path slash-encoded as %%2F", uri)
	}
	if token != "tok-123" {
		t.Fatalf("PRIVATE-TOKEN = %q", token)
	}
}

func TestGitLabOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status int
		want   domain.RepoOutcome
	}{
		{http.StatusNotFound, domain.RepoNotFound},
		{http.StatusUnauthorized, domain.RepoForbidden},
		{http.StatusForbidden, domain.RepoForbidden},
		{http.StatusInternalServerError, domain.PlatformUnreachable},
		{http.StatusBadGateway, domain.PlatformUnreachable},
		{http.StatusTooManyRequests, domain.PlatformUnreachable},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status) }))
		got := resolve(t, domain.PlatformGitLab, srv.URL, "a/b")
		srv.Close()
		if got.Outcome != tc.want {
			t.Errorf("status %d: outcome %v, want %v", tc.status, got.Outcome, tc.want)
		}
	}
}

func TestAnUnreachablePlatformIsReportedAsSuch(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens any more
	for _, kind := range []domain.PlatformKind{domain.PlatformGitLab, domain.PlatformGitHub} {
		if got := resolve(t, kind, url, "a/b"); got.Outcome != domain.PlatformUnreachable {
			t.Errorf("%s: outcome %v, want PlatformUnreachable", kind, got.Outcome)
		}
	}
	// Garbage instead of JSON on a 200 is not a usable answer either.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>login</html>")) }))
	defer bad.Close()
	if got := resolve(t, domain.PlatformGitLab, bad.URL, "a/b"); got.Outcome != domain.PlatformUnreachable {
		t.Errorf("html body: outcome %v, want PlatformUnreachable", got.Outcome)
	}
}

func TestGitHubEnterpriseResolvesTheRepositoryID(t *testing.T) {
	t.Parallel()
	var uri, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri, auth = r.RequestURI, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id": 552310987, "full_name": "platform/order-service"}`))
	}))
	defer srv.Close()

	got := resolve(t, domain.PlatformGitHub, srv.URL, "platform/order-service")
	if got.Outcome != domain.RepoFound || got.ProjectID != "552310987" {
		t.Fatalf("got %+v", got)
	}
	if uri != "/api/v3/repos/platform/order-service" || auth != "Bearer tok-123" {
		t.Fatalf("uri %q auth %q", uri, auth)
	}
}

func TestGitHubOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		status  int
		headers map[string]string
		want    domain.RepoOutcome
	}{
		{"not found", http.StatusNotFound, nil, domain.RepoNotFound},
		{"bad token", http.StatusUnauthorized, nil, domain.RepoForbidden},
		{"no access", http.StatusForbidden, nil, domain.RepoForbidden},
		{"rate limited", http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "0"}, domain.PlatformUnreachable},
		{"server error", http.StatusInternalServerError, nil, domain.PlatformUnreachable},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range tc.headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(tc.status)
		}))
		got := resolve(t, domain.PlatformGitHub, srv.URL, "o/r")
		srv.Close()
		if got.Outcome != tc.want {
			t.Errorf("%s: outcome %v, want %v", tc.name, got.Outcome, tc.want)
		}
	}
}

func TestGitHubPathsMustBeOwnerSlashRepo(t *testing.T) {
	t.Parallel()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()
	if got := resolve(t, domain.PlatformGitHub, srv.URL, "group/sub/repo"); got.Outcome != domain.RepoNotFound || called {
		t.Fatalf("got %+v, called=%v", got, called)
	}
}

func TestGitHubDotComUsesTheHostedAPI(t *testing.T) {
	t.Parallel()
	for base, want := range map[string]string{
		"https://github.com":          "https://api.github.com",
		"https://www.github.com":      "https://api.github.com",
		"https://api.github.com":      "https://api.github.com",
		"https://github.corp.example": "https://github.corp.example/api/v3",
		"http://ghe.local:8443/":      "http://ghe.local:8443/api/v3",
	} {
		if got := githubAPIBase(base); got != want {
			t.Errorf("githubAPIBase(%q) = %q, want %q", base, got, want)
		}
	}
}
