// Package app wires configuration, database, domain service and HTTP layer.
// Start is the single startup path, used by cmd/xproject and by the API tests.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/httpapi"
	"github.com/admax12959/X-Project/internal/store/mysqlstore"
)

// Deps are the external adapters. Tests pass fakes (internal/fakes);
// production passes real implementations.
type Deps struct {
	Directories   domain.DirectoryFactory    // builds the LDAP directory from the stored configuration; nil = LDAP unavailable
	CodePlatforms domain.CodePlatformAdapter // nil = no platform adapter available
	MailSenders   domain.MailSenderFactory   // builds the SMTP sender from the stored mail configuration; nil = mail cannot be sent
	ObjectStores  domain.ObjectStoreFactory  // builds the object store from the stored configuration; nil = object storage unavailable
	Now           func() time.Time           // nil = time.Now
}

// App is a started application.
type App struct {
	Handler http.Handler
	Service *domain.Service // the domain service behind Handler, for callers that are not HTTP (tests such as env.EnqueueMail, CLI commands)
	db      *sql.DB
	stop    context.CancelFunc
	workers sync.WaitGroup
}

// Close stops the background tasks and closes the database.
func (a *App) Close() {
	a.stop()
	a.workers.Wait()
	a.Service.WaitBackground()
	_ = a.db.Close()
}

// OpenService connects to the database and builds the domain service for the command line
// (#23): no migration, no bootstrap, no background workers and no HTTP, so it can run next to
// a live server (only deps.Now is used, a test clock). Mail it queues is delivered by the server's worker. The schema must exist
// already (the server has started once). The returned function closes the database.
func OpenService(ctx context.Context, cfg Config, deps Deps) (*domain.Service, func(), error) {
	masterKey, err := cfg.MasterKey()
	if err != nil {
		return nil, nil, err
	}
	db, err := mysqlstore.Open(ctx, cfg.Database.DSN, cfg.Database.ConnectTimeout)
	if err != nil {
		return nil, nil, err
	}
	svc := domain.NewService(mysqlstore.New(db), domain.Options{
		Now: deps.Now, SessionTTL: cfg.Session.TTL, SecretKey: masterKey, SecretKeyVersion: cfg.SecretKeyVersion,
		MailRetryBackoff: cfg.Mail.RetryBackoff, LocalStorageDir: cfg.Storage.LocalDir,
	})
	return svc, func() { svc.WaitBackground(); _ = db.Close() }, nil
}

// avatarSweepInterval is how often orphan avatar objects are cleaned up.
const avatarSweepInterval = 6 * time.Hour

// Start connects to the database (retrying while it starts), runs migrations,
// creates the initial administrator on first start, and builds the handler.
func Start(ctx context.Context, cfg Config, deps Deps) (*App, error) {
	masterKey, err := cfg.MasterKey()
	if err != nil {
		return nil, err
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
		DirectoryFactory: deps.Directories, CodePlatforms: deps.CodePlatforms, MailSenders: deps.MailSenders, Now: deps.Now, SessionTTL: cfg.Session.TTL,
		SecretKey: masterKey, SecretKeyVersion: cfg.SecretKeyVersion, MailRetryBackoff: cfg.Mail.RetryBackoff,
		ObjectStoreFactory: deps.ObjectStores, LocalStorageDir: cfg.Storage.LocalDir,
	})
	if err := svc.Bootstrap(ctx, domain.InitialAdmin{
		Username: cfg.Admin.Username, Password: cfg.Admin.Password,
		DisplayName: cfg.Admin.DisplayName, Email: cfg.Admin.Email,
	}); err != nil {
		db.Close()
		return nil, err
	}
	// The LDAP block of the configuration file is imported once; the database rules after that.
	l := cfg.LDAP
	if err := svc.ImportLDAPConfig(ctx, domain.LDAPImport{
		URL: l.URL, StartTLS: l.StartTLS, SkipVerify: l.SkipVerify, BindDN: l.BindDN, BindPassword: l.BindPassword,
		BaseDN: l.BaseDN, UserFilter: l.UserFilter, LoginAttr: l.LoginAttr, NameAttr: l.NameAttr, EmailAttr: l.EmailAttr,
		DepartmentAttr: l.DepartmentAttr, DisabledFilter: l.DisabledFilter, Timeout: l.Timeout,
	}); err != nil {
		db.Close()
		return nil, err
	}
	h := httpapi.New(svc, httpapi.Options{CookieSecure: cfg.Session.CookieSecure, WebDir: cfg.WebDir})
	// Background tasks run until Close. The mail worker delivers the outgoing mail queue.
	workCtx, stop := context.WithCancel(context.Background())
	a := &App{Handler: h, Service: svc, db: db, stop: stop}
	a.workers.Add(1)
	go func() {
		defer a.workers.Done()
		svc.RunMailWorker(workCtx, cfg.Mail.PollInterval)
	}()
	// The avatar sweeper deletes avatar objects nobody points to (left behind by failures).
	a.workers.Add(1)
	go func() {
		defer a.workers.Done()
		svc.RunAvatarSweeper(workCtx, avatarSweepInterval)
	}()
	return a, nil
}
