package httpapi

import (
	"net/http"

	"github.com/admax12959/X-Project/internal/domain"
)

// System configuration (#19, ADR-0005). Administrators only (config.manage). Secret fields are
// write-only: a request may carry them, a response never does, it only says whether one is set.

func (s *server) systemConfigRoutes() {
	s.route("GET /api/system-config/ldap", s.getLDAPConfig)
	s.route("PUT /api/system-config/ldap", s.putLDAPConfig)
	s.route("POST /api/system-config/ldap/test", s.testLDAPConfig)
	s.route("GET /api/system-config/storage", s.getStorageConfig)
	s.route("PUT /api/system-config/storage", s.putStorageConfig)
	s.route("POST /api/system-config/storage/test", s.testStorageConfig)
	s.route("GET /api/system-config/security", s.getSecurityPolicy)
	s.route("PUT /api/system-config/security", s.putSecurityPolicy)
}

// securityPolicyJSON is both the request of PUT and the answer of GET and PUT (#23). Administrators
// are required to have a second factor whatever this says; the page shows that as a fact.
type securityPolicyJSON struct {
	ForceSecondFactor bool   `json:"forceSecondFactor"`
	Configured        bool   `json:"configured"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

func toSecurityPolicyJSON(v domain.SecurityPolicyView) securityPolicyJSON {
	out := securityPolicyJSON{ForceSecondFactor: v.ForceSecondFactor, Configured: v.Configured}
	if v.Configured {
		out.UpdatedAt = formatTime(v.UpdatedAt)
	}
	return out
}

func (s *server) getSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	v, err := s.svc.GetSecurityPolicy(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSecurityPolicyJSON(v))
}

func (s *server) putSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ForceSecondFactor bool `json:"forceSecondFactor"`
	}
	if !decode(w, r, &body) {
		return
	}
	v, err := s.svc.UpdateSecurityPolicy(r.Context(), actorOf(r), domain.SecurityPolicy{ForceSecondFactor: body.ForceSecondFactor})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSecurityPolicyJSON(v))
}

// ldapConfigBody is the request of PUT and POST .../test: the whole form. bindPassword empty
// or absent keeps the stored password.
type ldapConfigBody struct {
	Enabled        bool   `json:"enabled"`
	URL            string `json:"url"`
	StartTLS       bool   `json:"startTls"`
	SkipVerify     bool   `json:"skipVerify"`
	BindDN         string `json:"bindDn"`
	BindPassword   string `json:"bindPassword"`
	BaseDN         string `json:"baseDn"`
	UserFilter     string `json:"userFilter"`
	LoginAttr      string `json:"loginAttr"`
	NameAttr       string `json:"nameAttr"`
	EmailAttr      string `json:"emailAttr"`
	DepartmentAttr string `json:"departmentAttr"`
	DisabledFilter string `json:"disabledFilter"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

func (b ldapConfigBody) input() domain.LDAPConfigInput {
	return domain.LDAPConfigInput{BindPassword: b.BindPassword, LDAPSettings: domain.LDAPSettings{
		Enabled: b.Enabled, URL: b.URL, StartTLS: b.StartTLS, SkipVerify: b.SkipVerify, BindDN: b.BindDN, BaseDN: b.BaseDN,
		UserFilter: b.UserFilter, LoginAttr: b.LoginAttr, NameAttr: b.NameAttr, EmailAttr: b.EmailAttr,
		DepartmentAttr: b.DepartmentAttr, DisabledFilter: b.DisabledFilter, TimeoutSeconds: b.TimeoutSeconds,
	}}
}

// ldapConfigJSON is the response: the form without the password. bindPasswordSet says whether
// one is stored; secretsUnreadable that a stored secret cannot be decrypted with the current
// master key (enter it again); configured that the section was ever saved (false = defaults).
type ldapConfigJSON struct {
	Enabled           bool   `json:"enabled"`
	URL               string `json:"url"`
	StartTLS          bool   `json:"startTls"`
	SkipVerify        bool   `json:"skipVerify"`
	BindDN            string `json:"bindDn"`
	BindPasswordSet   bool   `json:"bindPasswordSet"`
	BaseDN            string `json:"baseDn"`
	UserFilter        string `json:"userFilter"`
	LoginAttr         string `json:"loginAttr"`
	NameAttr          string `json:"nameAttr"`
	EmailAttr         string `json:"emailAttr"`
	DepartmentAttr    string `json:"departmentAttr"`
	DisabledFilter    string `json:"disabledFilter"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
	SecretsUnreadable bool   `json:"secretsUnreadable"`
	Configured        bool   `json:"configured"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

func toLDAPConfigJSON(v domain.LDAPConfigView) ldapConfigJSON {
	out := ldapConfigJSON{
		Enabled: v.Enabled, URL: v.URL, StartTLS: v.StartTLS, SkipVerify: v.SkipVerify, BindDN: v.BindDN,
		BindPasswordSet: v.BindPasswordSet, BaseDN: v.BaseDN, UserFilter: v.UserFilter, LoginAttr: v.LoginAttr,
		NameAttr: v.NameAttr, EmailAttr: v.EmailAttr, DepartmentAttr: v.DepartmentAttr, DisabledFilter: v.DisabledFilter,
		TimeoutSeconds: v.TimeoutSeconds, SecretsUnreadable: v.SecretsUnreadable, Configured: v.Configured,
	}
	if v.Configured {
		out.UpdatedAt = formatTime(v.UpdatedAt)
	}
	return out
}

// connectionTestJSON is the answer of every "test connection" route: ok, or ok=false with the
// error code (shown as the message error.<code>). A failed test is a 200, not an error.
type connectionTestJSON struct {
	OK   bool   `json:"ok"`
	Code string `json:"code,omitempty"`
}

func (s *server) getLDAPConfig(w http.ResponseWriter, r *http.Request) {
	v, err := s.svc.GetLDAPConfig(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLDAPConfigJSON(v))
}

func (s *server) putLDAPConfig(w http.ResponseWriter, r *http.Request) {
	var body ldapConfigBody
	if !decode(w, r, &body) {
		return
	}
	v, err := s.svc.UpdateLDAPConfig(r.Context(), actorOf(r), body.input())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLDAPConfigJSON(v))
}

func (s *server) testLDAPConfig(w http.ResponseWriter, r *http.Request) {
	var body ldapConfigBody
	if !decode(w, r, &body) {
		return
	}
	res, err := s.svc.TestLDAPConfig(r.Context(), actorOf(r), body.input())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, connectionTestJSON{OK: res.OK, Code: res.Code})
}
