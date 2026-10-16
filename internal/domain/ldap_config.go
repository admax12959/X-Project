package domain

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// LDAP configuration (#19): the one corporate directory, edited on the administrator's LDAP
// page and stored in the database (system_config.go). The service account password
// ("bindPassword") is the only secret.
//
// The directory is built from the stored configuration whenever it is needed (a login, a
// binding, a username check): there is no cached directory, so a saved change is in effect
// at once and on every instance. "Test connection" builds a throw-away directory from the
// unsaved form.

const (
	ldapSecretBindPassword = "bindPassword"

	defaultLDAPTimeoutSeconds = 10
	maxLDAPTimeoutSeconds     = 120
	maxLDAPFieldLength        = 1000

	// ldapProbeLogin is looked up by "test connection": it only matters that the search runs.
	ldapProbeLogin = "xp-connection-test"
)

// LDAPSettings are the non-secret LDAP fields. The JSON tags are the stored format and the
// API names.
type LDAPSettings struct {
	Enabled        bool   `json:"enabled"`
	URL            string `json:"url"`
	StartTLS       bool   `json:"startTls"`
	SkipVerify     bool   `json:"skipVerify"`
	BindDN         string `json:"bindDn"`
	BaseDN         string `json:"baseDn"`
	UserFilter     string `json:"userFilter"`
	LoginAttr      string `json:"loginAttr"`
	NameAttr       string `json:"nameAttr"`
	EmailAttr      string `json:"emailAttr"`
	DepartmentAttr string `json:"departmentAttr"`
	DisabledFilter string `json:"disabledFilter"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

// LDAPConfigInput is a save or test request. BindPassword is write-only: empty keeps the
// stored password, anything else replaces it.
type LDAPConfigInput struct {
	LDAPSettings
	BindPassword string
}

// LDAPConfigView is what an administrator gets to see. The password is never part of it,
// only whether one is set. SecretsUnreadable means a stored secret cannot be decrypted with
// the current master key (it changed): enter the password again.
type LDAPConfigView struct {
	LDAPSettings
	BindPasswordSet   bool
	SecretsUnreadable bool
	Configured        bool // saved at least once (imported or edited); false = defaults shown
	UpdatedAt         time.Time
}

func ldapDefaults() LDAPSettings {
	return LDAPSettings{
		UserFilter: "(uid=%s)", LoginAttr: "uid", NameAttr: "displayName", EmailAttr: "mail",
		DepartmentAttr: "department", TimeoutSeconds: defaultLDAPTimeoutSeconds,
	}
}

func (l *LDAPSettings) fillDefaults() {
	d := ldapDefaults()
	set := func(p *string, def string) {
		if strings.TrimSpace(*p) == "" {
			*p = def
		}
	}
	set(&l.UserFilter, d.UserFilter)
	set(&l.LoginAttr, d.LoginAttr)
	set(&l.NameAttr, d.NameAttr)
	set(&l.EmailAttr, d.EmailAttr)
	set(&l.DepartmentAttr, d.DepartmentAttr)
	if l.TimeoutSeconds == 0 {
		l.TimeoutSeconds = d.TimeoutSeconds
	}
}

// readLDAP returns the stored settings (defaults when never saved).
func (s *Service) readLDAP(ctx context.Context, repo Repository) (settings LDAPSettings, saved bool, at time.Time, err error) {
	settings = ldapDefaults()
	saved, at, err = loadSection(ctx, repo, SectionLDAP, &settings)
	settings.fillDefaults()
	return settings, saved, at, err
}

// GetLDAPConfig returns the LDAP configuration for the LDAP page. Administrators only.
func (s *Service) GetLDAPConfig(ctx context.Context, actor Actor) (LDAPConfigView, error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return LDAPConfigView{}, err
	}
	return s.ldapView(ctx, s.repo)
}

func (s *Service) ldapView(ctx context.Context, repo Repository) (LDAPConfigView, error) {
	settings, saved, at, err := s.readLDAP(ctx, repo)
	if err != nil {
		return LDAPConfigView{}, err
	}
	secrets, err := s.loadSecrets(ctx, repo, SectionLDAP)
	if err != nil {
		return LDAPConfigView{}, err
	}
	return LDAPConfigView{
		LDAPSettings: settings, BindPasswordSet: secrets.set[ldapSecretBindPassword],
		SecretsUnreadable: secrets.anyUnreadable(), Configured: saved, UpdatedAt: at,
	}, nil
}

// validateLDAP checks a request against the stored state (the stored password counts when
// the request leaves the password empty) and returns the settings to save, with defaults
// filled, and the service account password to use. It enforces what the LDAP adapter needs,
// so a configuration that passes here can be built.
func validateLDAP(in LDAPConfigInput, secrets configSecrets) (LDAPSettings, string, error) {
	var c Collector
	l := in.LDAPSettings
	trim := func(field string, p *string) {
		*p = strings.TrimSpace(*p)
		if len([]rune(*p)) > maxLDAPFieldLength {
			c.Add(field, CodeTooLong, map[string]any{"max": maxLDAPFieldLength})
		}
	}
	trim("url", &l.URL)
	trim("bindDn", &l.BindDN)
	trim("baseDn", &l.BaseDN)
	trim("userFilter", &l.UserFilter)
	trim("loginAttr", &l.LoginAttr)
	trim("nameAttr", &l.NameAttr)
	trim("emailAttr", &l.EmailAttr)
	trim("departmentAttr", &l.DepartmentAttr)
	trim("disabledFilter", &l.DisabledFilter)
	l.fillDefaults()

	if l.URL != "" {
		if u, err := url.Parse(l.URL); err != nil || (u.Scheme != "ldap" && u.Scheme != "ldaps") || u.Hostname() == "" {
			c.Add("url", CodeInvalid, map[string]any{"allowed": []string{"ldap://", "ldaps://"}})
		}
	} else if l.Enabled {
		c.Add("url", CodeRequired, nil)
	}
	if l.Enabled {
		c.Required("baseDn", l.BaseDN)
	}
	if strings.Count(l.UserFilter, "%s") != 1 {
		c.Add("userFilter", CodeInvalid, nil)
	}
	if l.TimeoutSeconds < 1 || l.TimeoutSeconds > maxLDAPTimeoutSeconds {
		c.Add("timeoutSeconds", CodeInvalid, map[string]any{"min": 1, "max": maxLDAPTimeoutSeconds})
	}

	password := ""
	switch {
	case l.BindDN == "" && in.BindPassword != "":
		c.Add("bindDn", CodeRequired, nil) // a password without an account to use it
	case l.BindDN != "":
		password = in.BindPassword
		if password == "" {
			password = secrets.plain[ldapSecretBindPassword]
		}
		if password == "" {
			c.Add("bindPassword", CodeRequired, nil)
		}
	}
	if err := c.Err(); err != nil {
		return LDAPSettings{}, "", err
	}
	return l, password, nil
}

// changedLDAPFields names the fields that differ, in form order. Values are never listed.
func changedLDAPFields(a, b LDAPSettings) []string {
	var out []string
	add := func(name string, differs bool) {
		if differs {
			out = append(out, name)
		}
	}
	add("enabled", a.Enabled != b.Enabled)
	add("url", a.URL != b.URL)
	add("startTls", a.StartTLS != b.StartTLS)
	add("skipVerify", a.SkipVerify != b.SkipVerify)
	add("bindDn", a.BindDN != b.BindDN)
	add("baseDn", a.BaseDN != b.BaseDN)
	add("userFilter", a.UserFilter != b.UserFilter)
	add("loginAttr", a.LoginAttr != b.LoginAttr)
	add("nameAttr", a.NameAttr != b.NameAttr)
	add("emailAttr", a.EmailAttr != b.EmailAttr)
	add("departmentAttr", a.DepartmentAttr != b.DepartmentAttr)
	add("disabledFilter", a.DisabledFilter != b.DisabledFilter)
	add("timeoutSeconds", a.TimeoutSeconds != b.TimeoutSeconds)
	return out
}

// UpdateLDAPConfig saves the LDAP configuration and records which fields changed (names
// only) in the audit log. Administrators only. The new configuration is in effect at once.
func (s *Service) UpdateLDAPConfig(ctx context.Context, actor Actor, in LDAPConfigInput) (LDAPConfigView, error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return LDAPConfigView{}, err
	}
	var view LDAPConfigView
	err := s.repo.InTx(ctx, func(tx Repository) error {
		old, _, _, err := s.readLDAP(ctx, tx)
		if err != nil {
			return err
		}
		secrets, err := s.loadSecrets(ctx, tx, SectionLDAP)
		if err != nil {
			return err
		}
		next, _, err := validateLDAP(in, secrets)
		if err != nil {
			return err
		}
		changes := changedLDAPFields(old, next)
		data, err := jsonBytes(next)
		if err != nil {
			return err
		}
		if err := tx.PutConfigSection(ctx, SectionLDAP, data, s.nowUTC()); err != nil {
			return err
		}
		switch {
		case in.BindPassword != "": // validateLDAP guarantees a bind DN here
			if err := s.saveSecret(ctx, tx, SectionLDAP, ldapSecretBindPassword, in.BindPassword); err != nil {
				return err
			}
			changes = append(changes, ldapSecretBindPassword)
		case next.BindDN == "" && secrets.set[ldapSecretBindPassword]: // anonymous search: no password to keep
			if err := tx.DeleteConfigSecret(ctx, SectionLDAP, ldapSecretBindPassword); err != nil {
				return err
			}
			changes = append(changes, ldapSecretBindPassword)
		}
		if len(changes) > 0 {
			if err := s.recordAudit(ctx, tx, AuditActorOf(actor), configChangedAudit(SectionLDAP, changes)); err != nil {
				return err
			}
		}
		view, err = s.ldapView(ctx, tx)
		return err
	})
	return view, err
}

// TestLDAPConfig checks that the directory described by the (unsaved) form can be reached,
// the service account can sign in and the base DN can be searched. Nothing is saved and
// nothing is audited. A password left empty falls back to the stored one. Administrators only.
func (s *Service) TestLDAPConfig(ctx context.Context, actor Actor, in LDAPConfigInput) (ConnectionTest, error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return ConnectionTest{}, err
	}
	secrets, err := s.loadSecrets(ctx, s.repo, SectionLDAP)
	if err != nil {
		return ConnectionTest{}, err
	}
	in.Enabled = true // testing a form means: as if it were switched on
	settings, password, err := validateLDAP(in, secrets)
	if err != nil {
		return ConnectionTest{}, err
	}
	if s.directories == nil {
		return ConnectionTest{Code: CodeLdapUnreachable}, nil
	}
	dir, err := s.directories.Build(ldapConfig(settings, password))
	if err != nil {
		slog.Warn("ldap: test connection: cannot build the directory", "err", err)
		return ConnectionTest{Code: CodeLdapUnreachable}, nil
	}
	if dir.Lookup(ctx, ldapProbeLogin).Outcome == DirectoryUnreachable {
		return ConnectionTest{Code: CodeLdapUnreachable}, nil
	}
	return ConnectionTest{OK: true}, nil
}

func ldapConfig(l LDAPSettings, password string) LDAPConfig {
	return LDAPConfig{
		URL: l.URL, StartTLS: l.StartTLS, SkipVerify: l.SkipVerify, BindDN: l.BindDN, BindPassword: password,
		BaseDN: l.BaseDN, UserFilter: l.UserFilter, LoginAttr: l.LoginAttr, NameAttr: l.NameAttr,
		EmailAttr: l.EmailAttr, DepartmentAttr: l.DepartmentAttr, DisabledFilter: l.DisabledFilter,
		Timeout: time.Duration(l.TimeoutSeconds) * time.Second,
	}
}

// ldapEnabled reports whether LDAP login is switched on (and can be built at all).
func (s *Service) ldapEnabled(ctx context.Context) (bool, error) {
	if s.directories == nil {
		return false, nil
	}
	settings, _, _, err := s.readLDAP(ctx, s.repo)
	return settings.Enabled && settings.URL != "", err
}

// currentDirectory is the one way the rest of the domain gets the directory: built from the
// stored configuration on every call. It returns nil when LDAP is switched off. A stored
// password that cannot be decrypted (the master key changed) or a configuration the adapter
// refuses is reported as an unreachable directory and logged; the administrator fixes it on
// the LDAP page.
func (s *Service) currentDirectory(ctx context.Context) (Directory, error) {
	if s.directories == nil {
		return nil, nil
	}
	settings, _, _, err := s.readLDAP(ctx, s.repo)
	if err != nil || !settings.Enabled || settings.URL == "" {
		return nil, err
	}
	secrets, err := s.loadSecrets(ctx, s.repo, SectionLDAP)
	if err != nil {
		return nil, err
	}
	if secrets.unreadable[ldapSecretBindPassword] {
		slog.Error("ldap: the stored service account password cannot be decrypted with the current master key; enter it again on the LDAP page")
		return nil, ldapUnreachable()
	}
	dir, err := s.directories.Build(ldapConfig(settings, secrets.plain[ldapSecretBindPassword]))
	if err != nil {
		slog.Error("ldap: the stored configuration cannot be used", "err", err)
		return nil, ldapUnreachable()
	}
	return dir, nil
}

// requireDirectory is currentDirectory for the callers that need LDAP to be on: switched off
// counts as unavailable.
func (s *Service) requireDirectory(ctx context.Context) (Directory, error) {
	dir, err := s.currentDirectory(ctx)
	if err == nil && dir == nil {
		return nil, ldapUnreachable()
	}
	return dir, err
}

// LDAPImport is the LDAP block of the configuration file (see ImportLDAPConfig).
type LDAPImport struct {
	URL            string
	StartTLS       bool
	SkipVerify     bool
	BindDN         string
	BindPassword   string
	BaseDN         string
	UserFilter     string
	LoginAttr      string
	NameAttr       string
	EmailAttr      string
	DepartmentAttr string
	DisabledFilter string
	Timeout        time.Duration
}

// ImportLDAPConfig is the one-time import of the pre-#19 configuration file: when the
// database has no LDAP configuration yet and the file has one, the file's becomes the stored,
// enabled configuration. Every later start ignores the file, even if it changes (the database
// is the source of truth, ADR-0005). It runs at every start and is safe to run concurrently
// on several instances. An unusable file configuration stops startup. The import is not an
// operation of any user and is not audited.
func (s *Service) ImportLDAPConfig(ctx context.Context, in LDAPImport) error {
	if in.URL == "" {
		if in.BaseDN != "" || in.BindDN != "" {
			return fmt.Errorf("ldap: base_dn/bind_dn are set but url is empty (ldap.url or XP_LDAP_URL)")
		}
		return nil
	}
	form := LDAPConfigInput{BindPassword: in.BindPassword, LDAPSettings: LDAPSettings{
		Enabled: true, URL: in.URL, StartTLS: in.StartTLS, SkipVerify: in.SkipVerify, BindDN: in.BindDN, BaseDN: in.BaseDN,
		UserFilter: in.UserFilter, LoginAttr: in.LoginAttr, NameAttr: in.NameAttr, EmailAttr: in.EmailAttr,
		DepartmentAttr: in.DepartmentAttr, DisabledFilter: in.DisabledFilter, TimeoutSeconds: int(in.Timeout / time.Second),
	}}
	if in.Timeout > 0 && in.Timeout < time.Second {
		form.TimeoutSeconds = 1
	}
	settings, password, err := validateLDAP(form, configSecrets{})
	if err != nil {
		return fmt.Errorf("ldap configuration in the configuration file is unusable: %w", err)
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		data, err := jsonBytes(settings)
		if err != nil {
			return err
		}
		inserted, err := tx.InsertConfigSectionIfAbsent(ctx, SectionLDAP, data, s.nowUTC())
		if err != nil || !inserted {
			return err
		}
		if password == "" {
			return nil
		}
		return s.saveSecret(ctx, tx, SectionLDAP, ldapSecretBindPassword, password)
	})
}
