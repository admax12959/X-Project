// Package app wires configuration, database, domain service and HTTP layer.
// Start is the single startup path, used by cmd/xproject and by the API tests.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/httpapi"
	"github.com/admax12959/X-Project/internal/ldapdir"
	"github.com/admax12959/X-Project/internal/store/mysqlstore"
)

// Deps are the external adapters. Tests pass fakes (internal/fakes);
// production passes real implementations once they exist.
type Deps struct {
	Directory     domain.Directory           // nil = LDAP not configured
	CodePlatforms domain.CodePlatformAdapter // nil = no platform adapter available
	Now           func() time.Time           // nil = time.Now
}

// NewDirectory builds the real LDAP adapter from the configuration. It returns nil
// (LDAP not configured) when no LDAP url is set, and an error for a partial configuration.
func NewDirectory(cfg Config) (domain.Directory, error) {
	l := cfg.LDAP
	if l.URL == "" {
		if l.BaseDN != "" || l.BindDN != "" {
			return nil, fmt.Errorf("ldap: base_dn/bind_dn are set but url is empty (ldap.url or XP_LDAP_URL)")
		}
		return nil, nil
	}
	d, err := ldapdir.New(ldapdir.Config{
		URL: l.URL, StartTLS: l.StartTLS, SkipVerify: l.SkipVerify,
		BindDN: l.BindDN, BindPassword: l.BindPassword, BaseDN: l.BaseDN,
		UserFilter: l.UserFilter, LoginAttr: l.LoginAttr, NameAttr: l.NameAttr,
		EmailAttr: l.EmailAttr, DeptAttr: l.DepartmentAttr, DisabledFilter: l.DisabledFilter, Timeout: l.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("ldap: %w", err)
	}
	return d, nil
}

// App is a started application.
type App struct {
	Handler http.Handler
	db      *sql.DB
}

func (a *App) Close() { _ = a.db.Close() }

// Start connects to the database (retrying while it starts), runs migrations,
// creates the initial administrator on first start, and builds the handler.
func Start(ctx context.Context, cfg Config, deps Deps) (*App, error) {
	if cfg.SecretKey == "" {
		return nil, errors.New("secret key is required (secret_key or XP_SECRET_KEY)")
	}
	db, err := mysqlstore.Open(ctx, cfg.Database.DSN, cfg.Database.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	if err := mysqlstore.Migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	svc := domain.NewService(mysqlstore.New(db), domain.Options{
		Directory: deps.Directory, CodePlatforms: deps.CodePlatforms, Now: deps.Now, SessionTTL: cfg.Session.TTL,
		SecretKey: cfg.SecretKey,
	})
	if err := svc.Bootstrap(ctx, domain.InitialAdmin{
		Username: cfg.Admin.Username, Password: cfg.Admin.Password,
		DisplayName: cfg.Admin.DisplayName, Email: cfg.Admin.Email,
	}); err != nil {
		db.Close()
		return nil, err
	}
	h := httpapi.New(svc, httpapi.Options{CookieSecure: cfg.Session.CookieSecure, WebDir: cfg.WebDir})
	return &App{Handler: h, db: db}, nil
}
