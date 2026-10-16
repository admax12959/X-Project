package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// System configuration (#19, ADR-0005). Administrators only (config.manage). Secret fields are
// write-only: a request may carry them, a response never does, it only says whether one is set.
//
// The LDAP, mail and storage sections look alike on the wire, so one registration serves all of
// them (registerConfigSection): the request body is the domain's input type itself (its JSON
// tags are the API names), the response is the settings plus what a section adds to them.

func (s *server) systemConfigRoutes() {
	registerConfigSection(s, "ldap", configSectionAPI[domain.LDAPConfigInput, domain.LDAPConfigView]{
		get: s.svc.GetLDAPConfig, put: s.svc.UpdateLDAPConfig, view: toLDAPConfigJSON,
		test: func(ctx context.Context, a domain.Actor, in domain.LDAPConfigInput) (any, error) {
			res, err := s.svc.TestLDAPConfig(ctx, a, in)
			return connectionTestJSON{OK: res.OK, Code: res.Code}, err
		},
	})
	registerConfigSection(s, "smtp", configSectionAPI[domain.SMTPConfigInput, domain.SMTPConfigView]{
		get: s.svc.GetSMTPConfig, put: s.svc.UpdateSMTPConfig, view: toSMTPConfigJSON,
		test: func(ctx context.Context, a domain.Actor, in domain.SMTPConfigInput) (any, error) {
			res, err := s.svc.TestSMTPConfig(ctx, a, in)
			return mailTestJSON{OK: res.OK, Code: res.Code, Detail: res.Detail, To: res.To}, err
		},
	})
	registerConfigSection(s, "storage", configSectionAPI[domain.StorageConfigInput, domain.StorageConfigView]{
		get: s.svc.GetStorageConfig, put: s.svc.UpdateStorageConfig, view: toStorageConfigJSON,
		test: func(ctx context.Context, a domain.Actor, in domain.StorageConfigInput) (any, error) {
			res, err := s.svc.TestStorageConfig(ctx, a, in)
			return connectionTestJSON{OK: res.OK, Code: res.Code}, err
		},
	})
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

// ldapConfigJSON is the response: the form without the password. bindPasswordSet says whether
// one is stored.
type ldapConfigJSON struct {
	domain.LDAPSettings
	BindPasswordSet bool `json:"bindPasswordSet"`
	configStatusJSON
}

func toLDAPConfigJSON(v domain.LDAPConfigView) any {
	return ldapConfigJSON{
		LDAPSettings: v.LDAPSettings, BindPasswordSet: v.BindPasswordSet,
		configStatusJSON: newConfigStatus(v.SecretsUnreadable, v.Configured, v.Configured, v.UpdatedAt),
	}
}

// connectionTestJSON is the answer of every "test connection" route: ok, or ok=false with the
// error code (shown as the message error.<code>). A failed test is a 200, not an error.
type connectionTestJSON struct {
	OK   bool   `json:"ok"`
	Code string `json:"code,omitempty"`
}

// configSectionAPI is what a section hands registerConfigSection: the domain calls, with In
// the request body (the whole form; a secret left empty or absent keeps the stored one) and V
// the view, and the view's JSON.
type configSectionAPI[In, V any] struct {
	get  func(context.Context, domain.Actor) (V, error)
	put  func(context.Context, domain.Actor, In) (V, error)
	test func(context.Context, domain.Actor, In) (any, error)
	view func(V) any
}

// registerConfigSection serves GET and PUT /api/system-config/<name> and POST .../test.
func registerConfigSection[In, V any](s *server, name string, api configSectionAPI[In, V]) {
	path := "/api/system-config/" + name
	s.route("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		v, err := api.get(r.Context(), actorOf(r))
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, api.view(v))
	})
	s.route("PUT "+path, func(w http.ResponseWriter, r *http.Request) {
		var in In
		if !decode(w, r, &in) {
			return
		}
		v, err := api.put(r.Context(), actorOf(r), in)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, api.view(v))
	})
	s.route("POST "+path+"/test", func(w http.ResponseWriter, r *http.Request) {
		var in In
		if !decode(w, r, &in) {
			return
		}
		res, err := api.test(r.Context(), actorOf(r), in)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})
}

// configStatusJSON is what every section's response says besides its settings and which
// secrets are set: secretsUnreadable, that a stored secret cannot be decrypted with the current
// master key (enter it again); configured, as each section defines it; updatedAt, when it was
// last saved (absent while never saved).
type configStatusJSON struct {
	SecretsUnreadable bool   `json:"secretsUnreadable"`
	Configured        bool   `json:"configured"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

func newConfigStatus(unreadable, configured, saved bool, updatedAt time.Time) configStatusJSON {
	out := configStatusJSON{SecretsUnreadable: unreadable, Configured: configured}
	if saved {
		out.UpdatedAt = formatTime(updatedAt)
	}
	return out
}
