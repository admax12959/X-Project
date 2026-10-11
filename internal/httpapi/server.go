// Package httpapi is the HTTP layer: it parses requests, authenticates the session
// cookie, calls the domain service and serializes the result. No business rules here.
package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// SessionCookie is the HttpOnly session cookie name.
const SessionCookie = "xp_session"

// Options configures the HTTP layer.
type Options struct {
	CookieSecure bool   // set Secure on the session cookie (enable behind HTTPS)
	WebDir       string // built frontend (index.html + assets); empty serves no UI
}

type server struct {
	svc    *domain.Service
	opts   Options
	mux    *http.ServeMux
	public map[string]bool // route patterns reachable without a session
}

type actorKey struct{}

// New returns the full handler: /api/* plus the single-page app.
func New(svc *domain.Service, opts Options) http.Handler {
	s := &server{svc: svc, opts: opts, mux: http.NewServeMux(), public: map[string]bool{}}
	s.routes()
	root := http.NewServeMux()
	root.Handle("/api/", http.HandlerFunc(s.serveAPI))
	root.Handle("/", spaHandler(opts.WebDir))
	return root
}

// routes registers every API endpoint. Routes are authenticated by default;
// only those registered with publicRoute are reachable without a session.
func (s *server) routes() {
	s.publicRoute("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.publicRoute("GET /api/login-methods", s.loginMethods)
	s.publicRoute("POST /api/session", s.login)
	s.route("GET /api/session", s.currentSession)
	s.route("DELETE /api/session", s.logout)

	s.route("GET /api/applications", s.listApplications)
	s.route("POST /api/applications", s.createApplication)
	s.route("GET /api/applications/{key}", s.getApplication)
	s.route("GET /api/assignable-users", s.listAssignableUsers)
	s.route("PATCH /api/applications/{key}", s.updateApplication)
	s.route("DELETE /api/applications/{key}", s.deleteApplication)
	s.route("PUT /api/applications/{key}/members", s.updateMembers)
	s.route("PUT /api/applications/{key}/deploy-info", s.updateDeployInfo)
	s.route("GET /api/applications/{key}/lines", s.listLines)
	s.route("POST /api/applications/{key}/lines", s.forkLine)
	s.route("GET /api/applications/{key}/lineage", s.lineage)
	s.route("GET /api/applications/{key}/history", s.listHistory)
	s.route("GET /api/applications/{key}/versions", s.listVersions)
	s.route("GET /api/versions", s.listAllVersions)
	s.route("GET /api/recent-activity", s.listRecentActivity)
	s.route("POST /api/applications/{key}/lines/{lineId}/versions", s.registerVersion)
	s.route("POST /api/repo-verifications", s.verifyRepo)

	s.route("GET /api/code-platforms", s.listCodePlatforms)
	s.route("POST /api/code-platforms", s.createCodePlatform)
	s.userRoutes()
	s.meRoutes()
}

func (s *server) route(pattern string, h http.HandlerFunc) { s.mux.HandleFunc(pattern, h) }

func (s *server) publicRoute(pattern string, h http.HandlerFunc) {
	s.public[pattern] = true
	s.mux.HandleFunc(pattern, h)
}

// serveAPI enforces default-deny: unless the matched pattern is public, a valid
// session is required before routing, so unknown paths also answer 401 when
// logged out.
func (s *server) serveAPI(w http.ResponseWriter, r *http.Request) {
	_, pattern := s.mux.Handler(r)
	if !s.public[pattern] {
		actor, err := s.svc.Authenticate(r.Context(), sessionToken(r))
		if err != nil {
			writeError(w, r, err)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), actorKey{}, actor))
	}
	if pattern == "" {
		writeCode(w, http.StatusNotFound, domain.CodeNotFound)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func actorOf(r *http.Request) domain.Actor { return r.Context().Value(actorKey{}).(domain.Actor) }

func sessionToken(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	c := &http.Cookie{
		Name: SessionCookie, Value: token, Path: "/",
		HttpOnly: true, Secure: s.opts.CookieSecure, SameSite: http.SameSiteLaxMode,
	}
	if token == "" {
		c.MaxAge = -1
	} else {
		c.Expires = expires
	}
	http.SetCookie(w, c)
}

// spaHandler serves files from dir and falls back to index.html for client routes.
// Route guarding (redirect to /login) happens in the SPA, which asks GET /api/session.
func spaHandler(dir string) http.Handler {
	if dir == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not configured (web_dir)", http.StatusNotFound)
		})
	}
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
