package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/fakes"
)

// Initial administrator configured for every test environment.
const (
	AdminUsername = "admin"
	AdminEmail    = "admin@example.com"
	AdminPassword = "initial-admin-pw"
)

// Email is the login email tests give a user: <username>@example.com.
func Email(username string) string { return username + "@example.com" }

// Env is one isolated backend: its own database, its own server, its own fakes.
type Env struct {
	t      testing.TB
	DSN    string
	Config app.Config
	// Fakes for the adapters. Configure them before the calls that need them.
	Directory *fakes.Directory
	// Directories is the factory the application builds its directory with from the saved LDAP
	// configuration; it always returns Directory and records the configurations it was given.
	Directories   *fakes.DirectoryFactory
	CodePlatforms *fakes.CodePlatform
	// Objects is the in-memory object store the application uses whatever the saved storage
	// configuration says; ObjectStores is the factory that hands it out and records the
	// configurations it was asked to build a store for.
	Objects      *fakes.ObjectStore
	ObjectStores *fakes.ObjectStoreFactory

	server *httptest.Server
	app    *app.App
}

// New starts a backend on a fresh database through the production startup path.
// mutate (optional) adjusts the configuration before the first start.
func New(t testing.TB, mutate ...func(*app.Config)) *Env {
	t.Helper()
	cfg := app.DefaultConfig()
	cfg.Database.DSN = freshDatabase(t)
	cfg.SecretKey = "test-secret-key"
	cfg.Admin.Username = AdminUsername
	cfg.Admin.Email = AdminEmail
	cfg.Admin.Password = AdminPassword
	cfg.Admin.DisplayName = "Initial Admin"
	cfg.Storage.LocalDir = filepath.Join(t.TempDir(), "storage")
	defaultLDAP(&cfg)
	for _, m := range mutate {
		m(&cfg)
	}
	dir := fakes.NewDirectory()
	objects := fakes.NewObjectStore()
	e := &Env{t: t, DSN: cfg.Database.DSN, Config: cfg, Directory: dir, Directories: fakes.NewDirectoryFactory(dir), CodePlatforms: fakes.NewCodePlatform(),
		Objects: objects, ObjectStores: fakes.NewObjectStoreFactory(objects)}
	e.start()
	return e
}

// StartError attempts a first start on a fresh database with the adjusted
// configuration and returns the startup error (nil if it started).
func StartError(t testing.TB, mutate func(*app.Config)) error {
	t.Helper()
	cfg := app.DefaultConfig()
	cfg.Database.DSN = freshDatabase(t)
	cfg.SecretKey = "test-secret-key"
	cfg.Admin.Username = AdminUsername
	cfg.Admin.Email = AdminEmail
	cfg.Admin.Password = AdminPassword
	defaultLDAP(&cfg)
	mutate(&cfg)
	a, err := app.Start(context.Background(), cfg, app.Deps{Directories: fakes.NewDirectoryFactory(fakes.NewDirectory()), CodePlatforms: fakes.NewCodePlatform(), ObjectStores: fakes.NewObjectStoreFactory(fakes.NewObjectStore())})
	if err == nil {
		a.Close()
	}
	return err
}

// defaultLDAP puts an LDAP block in the configuration file, as the pre-#19 test environments
// had a directory: the first start imports it, so LDAP login is on. Tests that want LDAP off
// (or other file values) clear or change it in their mutate function.
func defaultLDAP(cfg *app.Config) {
	cfg.LDAP.URL = "ldap://directory.test:389"
	cfg.LDAP.BaseDN = "dc=corp,dc=example"
}

func (e *Env) start() {
	e.t.Helper()
	a, err := app.Start(context.Background(), e.Config, app.Deps{Directories: e.Directories, CodePlatforms: e.CodePlatforms, ObjectStores: e.ObjectStores})
	if err != nil {
		e.t.Fatalf("app.Start: %v", err)
	}
	e.app = a
	e.server = httptest.NewServer(a.Handler)
	e.t.Cleanup(func() {
		e.server.Close()
		a.Close()
	})
}

// Restart simulates a process restart on the same database. mutate adjusts the
// configuration for the new process (for example a different admin password).
func (e *Env) Restart(mutate ...func(*app.Config)) {
	e.t.Helper()
	for _, m := range mutate {
		m(&e.Config)
	}
	e.server.Close()
	e.start()
}

// CurrentObjectStore asks the application for the object store it would use now, built from
// the saved storage configuration (what the avatar code does).
func (e *Env) CurrentObjectStore(ctx context.Context) (domain.ObjectStore, error) {
	return e.app.Service.ObjectStore(ctx)
}

// Client returns a new browser-like client with its own cookie jar.
func (e *Env) Client() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{t: e.t, env: e, http: &http.Client{Jar: jar}}
}

// AdminClient returns a client already logged in as the initial administrator.
func (e *Env) AdminClient() *Client {
	c := e.Client()
	c.LoginEmail(AdminEmail, AdminPassword).ExpectStatus(http.StatusOK)
	return c
}

// Client talks to the Env's server over HTTP, keeping cookies like a browser.
type Client struct {
	t    testing.TB
	env  *Env
	http *http.Client
}

// Response is a fully read HTTP response.
type Response struct {
	t       testing.TB
	Status  int
	Header  http.Header
	Cookies []*http.Cookie
	Body    []byte
}

// Do sends a request. body, if non-nil, is encoded as JSON.
func (c *Client) Do(method, path string, body any) *Response {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.env.server.URL+path, r)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return &Response{t: c.t, Status: res.StatusCode, Header: res.Header, Cookies: res.Cookies(), Body: b}
}

func (c *Client) Get(path string) *Response { c.t.Helper(); return c.Do(http.MethodGet, path, nil) }
func (c *Client) Post(path string, body any) *Response {
	c.t.Helper()
	return c.Do(http.MethodPost, path, body)
}
func (c *Client) Delete(path string) *Response {
	c.t.Helper()
	return c.Do(http.MethodDelete, path, nil)
}

// LoginEmail logs in with an email identity.
func (c *Client) LoginEmail(email, password string) *Response {
	c.t.Helper()
	return c.Post("/api/session", map[string]string{"method": "email", "email": email, "password": password})
}

// LoginAs logs in with the email identity of a user created the usual way in tests (login
// email <username>@example.com).
func (c *Client) LoginAs(username, password string) *Response {
	c.t.Helper()
	return c.LoginEmail(Email(username), password)
}

// Env returns the environment the client talks to.
func (c *Client) Env() *Env { return c.env }

// Put sends a PUT with a JSON body.
func (c *Client) Put(path string, body any) *Response {
	c.t.Helper()
	return c.Do(http.MethodPut, path, body)
}

// Cookies returns the cookies the client would send to the server.
func (c *Client) Cookies() []*http.Cookie {
	req, _ := http.NewRequest(http.MethodGet, c.env.server.URL, nil)
	return c.http.Jar.Cookies(req.URL)
}

// WithCookies returns a fresh client that sends exactly these cookies (for replay tests).
func (c *Client) WithCookies(cookies []*http.Cookie) *Client {
	n := c.env.Client()
	req, _ := http.NewRequest(http.MethodGet, c.env.server.URL, nil)
	n.http.Jar.SetCookies(req.URL, cookies)
	return n
}

// ExpectStatus fails the test unless the status matches.
func (r *Response) ExpectStatus(want int) *Response {
	r.t.Helper()
	if r.Status != want {
		r.t.Fatalf("status = %d, want %d; body: %s", r.Status, want, r.Body)
	}
	return r
}

// JSON decodes the body into v.
func (r *Response) JSON(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		r.t.Fatalf("decode %s: %v", r.Body, err)
	}
}

// Errors decodes the standard error envelope.
func (r *Response) Errors() []ErrorItem {
	r.t.Helper()
	var env struct {
		Errors []ErrorItem `json:"errors"`
	}
	r.JSON(&env)
	return env.Errors
}

// ErrorItem mirrors one entry of the error envelope.
type ErrorItem struct {
	Field  string         `json:"field"`
	Code   string         `json:"code"`
	Params map[string]any `json:"params"`
}

// ExpectError asserts status and that the envelope contains an error with this field and code.
func (r *Response) ExpectError(status int, field, code string) ErrorItem {
	r.t.Helper()
	r.ExpectStatus(status)
	for _, e := range r.Errors() {
		if e.Field == field && e.Code == code {
			return e
		}
	}
	r.t.Fatalf("no error {field:%q code:%q} in %s", field, code, r.Body)
	return ErrorItem{}
}
