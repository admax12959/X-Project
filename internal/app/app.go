// Package app wires configuration, database, domain service and HTTP layer.
// Start is the single startup path, used by cmd/xproject and by the API tests.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/httpapi"
	"github.com/admax12959/X-Project/internal/store/mysqlstore"
)

// Deps are the external adapters. Tests pass fakes (internal/fakes);
// production passes real implementations once they exist.
type Deps struct {
	Directory     domain.Directory           // nil = LDAP not configured
	CodePlatforms domain.CodePlatformAdapter // nil = no platform adapter available
	Now           func() time.Time           // nil = time.Now
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
