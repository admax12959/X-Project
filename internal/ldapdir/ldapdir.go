// Package ldapdir is the real domain.Directory: it talks to the one configured LDAP
// server (ADR-0002, ADR-0004). Tests of the domain use internal/fakes instead; this
// package has its own contract tests behind the "ldapcontract" build tag.
package ldapdir

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/admax12959/X-Project/internal/domain"
)

// Config describes the single LDAP directory. URL is required; everything else has a default
// or is optional.
type Config struct {
	URL            string        // ldap://host:389 or ldaps://host:636
	StartTLS       bool          // upgrade an ldap:// connection with StartTLS
	SkipVerify     bool          // do not verify the server certificate (not for production)
	BindDN         string        // service account used to search; empty = anonymous search
	BindPassword   string        //
	BaseDN         string        // where users are searched (subtree)
	UserFilter     string        // contains one %s for the escaped login name; default (uid=%s)
	LoginAttr      string        // attribute holding the canonical login name; default uid
	NameAttr       string        // display name attribute; default displayName
	EmailAttr      string        // default mail
	DeptAttr       string        // department attribute; default department
	DisabledFilter string        // optional filter that matches an account when it is disabled, e.g. AD: (userAccountControl:1.2.840.113556.1.4.803:=2)
	Timeout        time.Duration // per connection and per operation; default 10s
}

// Validate fills defaults and reports a configuration that cannot work.
func (c *Config) Validate() error {
	if c.URL == "" {
		return errors.New("ldap url is required")
	}
	if c.BaseDN == "" {
		return errors.New("ldap base_dn is required")
	}
	if (c.BindDN == "") != (c.BindPassword == "") {
		return errors.New("ldap bind_dn and bind_password must be set together")
	}
	set := func(p *string, def string) {
		if *p == "" {
			*p = def
		}
	}
	set(&c.UserFilter, "(uid=%s)")
	set(&c.LoginAttr, "uid")
	set(&c.NameAttr, "displayName")
	set(&c.EmailAttr, "mail")
	set(&c.DeptAttr, "department")
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if strings.Count(c.UserFilter, "%s") != 1 {
		return errors.New("ldap user_filter must contain exactly one %s")
	}
	return nil
}

// Directory implements domain.Directory.
type Directory struct {
	cfg Config
}

var _ domain.Directory = (*Directory)(nil)

// New validates cfg and returns the adapter. It does not connect: an unreachable
// directory at startup must not keep local logins from working.
func New(cfg Config) (*Directory, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Directory{cfg: cfg}, nil
}

// Authenticate searches the account, then verifies the password by binding as it.
func (d *Directory) Authenticate(ctx context.Context, login, password string) domain.DirectoryResult {
	// An empty password would be an unauthenticated bind, which servers accept.
	if login == "" || password == "" {
		return domain.DirectoryResult{Outcome: domain.DirectoryBadPassword}
	}
	return d.run(ctx, login, password)
}

// Lookup reports whether login currently names an enabled account.
func (d *Directory) Lookup(ctx context.Context, login string) domain.DirectoryResult {
	if login == "" {
		return domain.DirectoryResult{Outcome: domain.DirectoryNotFound}
	}
	return d.run(ctx, login, "")
}

var unreachable = domain.DirectoryResult{Outcome: domain.DirectoryUnreachable}

// run performs the lookup and, when password is non-empty, the verifying bind.
func (d *Directory) run(ctx context.Context, login, password string) domain.DirectoryResult {
	conn, err := d.dial(ctx)
	if err != nil {
		slog.Warn("ldap: connect failed", "err", err)
		return unreachable
	}
	defer conn.Close()

	if d.cfg.BindDN != "" {
		if err := conn.Bind(d.cfg.BindDN, d.cfg.BindPassword); err != nil {
			// A refused service account is our misconfiguration, not an unknown user.
			slog.Warn("ldap: service account bind failed", "err", err)
			return unreachable
		}
	}

	res, err := conn.Search(ldap.NewSearchRequest(
		d.cfg.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, int(d.cfg.Timeout.Seconds()), false,
		fmt.Sprintf(d.cfg.UserFilter, ldap.EscapeFilter(login)),
		[]string{d.cfg.LoginAttr, d.cfg.NameAttr, d.cfg.EmailAttr, d.cfg.DeptAttr, "thumbnailPhoto", "jpegPhoto"}, nil))
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) {
			slog.Warn("ldap: user_filter matches more than one entry", "login", login)
		} else {
			slog.Warn("ldap: search failed", "err", err)
		}
		return unreachable
	}
	switch len(res.Entries) {
	case 0:
		return domain.DirectoryResult{Outcome: domain.DirectoryNotFound}
	case 1:
	default:
		slog.Warn("ldap: user_filter matches more than one entry", "login", login)
		return unreachable
	}
	e := res.Entries[0]

	if d.cfg.DisabledFilter != "" {
		dis, err := conn.Search(ldap.NewSearchRequest(
			e.DN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, int(d.cfg.Timeout.Seconds()), false,
			d.cfg.DisabledFilter, []string{"dn"}, nil))
		if err != nil {
			slog.Warn("ldap: disabled check failed", "err", err)
			return unreachable
		}
		if len(dis.Entries) > 0 {
			return domain.DirectoryResult{Outcome: domain.DirectoryDisabled}
		}
	}

	if password != "" {
		if err := conn.Bind(e.DN, password); err != nil {
			if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
				return domain.DirectoryResult{Outcome: domain.DirectoryBadPassword}
			}
			slog.Warn("ldap: user bind failed", "err", err)
			return unreachable
		}
	}

	entry := domain.DirectoryEntry{
		Login:       e.GetAttributeValue(d.cfg.LoginAttr),
		DisplayName: e.GetAttributeValue(d.cfg.NameAttr),
		Email:       e.GetAttributeValue(d.cfg.EmailAttr),
		Department:  e.GetAttributeValue(d.cfg.DeptAttr),
	}
	// The directory photo: thumbnailPhoto first, jpegPhoto otherwise. Raw bytes; the domain
	// decides whether it is a usable picture.
	for _, attr := range []string{"thumbnailPhoto", "jpegPhoto"} {
		if b := e.GetRawAttributeValue(attr); len(b) > 0 {
			entry.Photo = b
			break
		}
	}
	if entry.Login == "" {
		entry.Login = login
	}
	return domain.DirectoryResult{Outcome: domain.DirectoryOK, Entry: entry}
}

func (d *Directory) dial(ctx context.Context) (*ldap.Conn, error) {
	opts := []ldap.DialOpt{ldap.DialWithDialer(&net.Dialer{Timeout: d.cfg.Timeout})}
	tlsCfg := &tls.Config{InsecureSkipVerify: d.cfg.SkipVerify} //nolint:gosec // explicit operator opt-out
	if u, err := url.Parse(d.cfg.URL); err == nil {
		tlsCfg.ServerName = u.Hostname()
	}
	if strings.HasPrefix(strings.ToLower(d.cfg.URL), "ldaps://") {
		opts = append(opts, ldap.DialWithTLSConfig(tlsCfg))
	}
	conn, err := ldap.DialURL(d.cfg.URL, opts...)
	if err != nil {
		return nil, err
	}
	conn.SetTimeout(d.cfg.Timeout)
	if d.cfg.StartTLS {
		if err := conn.StartTLS(tlsCfg); err != nil {
			conn.Close()
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Factory implements domain.DirectoryFactory: it builds the adapter for a configuration the
// administrator saved (or is about to test) on the LDAP page.
type Factory struct{}

var _ domain.DirectoryFactory = Factory{}

// Build validates cfg and returns the adapter. It does not connect.
func (Factory) Build(cfg domain.LDAPConfig) (domain.Directory, error) {
	return New(Config{
		URL: cfg.URL, StartTLS: cfg.StartTLS, SkipVerify: cfg.SkipVerify,
		BindDN: cfg.BindDN, BindPassword: cfg.BindPassword, BaseDN: cfg.BaseDN,
		UserFilter: cfg.UserFilter, LoginAttr: cfg.LoginAttr, NameAttr: cfg.NameAttr,
		EmailAttr: cfg.EmailAttr, DeptAttr: cfg.DepartmentAttr, DisabledFilter: cfg.DisabledFilter, Timeout: cfg.Timeout,
	})
}
