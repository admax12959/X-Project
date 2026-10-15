package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/fakes"
)

// Initial administrator configured for every test environment.
const (
	AdminUsername = "admin"
	AdminPassword = "initial-admin-pw"
)

// Env is one isolated backend: its own database, its own server, its own fakes.
type Env struct {
	t      testing.TB
	DSN    string
	Config app.Config
	// Fakes for the adapters. Configure them before the calls that need them.
	Directory     *fakes.Directory
	CodePlatforms *fakes.CodePlatform

	server *httptest.Server
}

// New starts a backend on a fresh database through the production startup path.
// mutate (optional) adjusts the configuration before the first start.
func New(t testing.TB, mutate ...func(*app.Config)) *Env {
	t.Helper()
	cfg := app.DefaultConfig()
	cfg.Database.DSN = freshDatabase(t)
	cfg.SecretKey = "test-secret-key"
	cfg.Admin.Username = AdminUsername
	cfg.Admin.Password = AdminPassword
	cfg.Admin.DisplayName = "Initial Admin"
	for _, m := range mutate {
		m(&cfg)
	}
	e := &Env{t: t, DSN: cfg.Database.DSN, Config: cfg, Directory: fakes.NewDirectory(), CodePlatforms: fakes.NewCodePlatform()}
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
	cfg.Admin.Password = AdminPassword
	mutate(&cfg)
	a, err := app.Start(context.Background(), cfg, app.Deps{Directory: fakes.NewDirectory(), CodePlatforms: fakes.NewCodePlatform()})
	if err == nil {
		a.Close()
	}
	return err
}

func (e *Env) start() {
	e.t.Helper()
	a, err := app.Start(context.Background(), e.Config, app.Deps{Directory: e.Directory, CodePlatforms: e.CodePlatforms})
	if err != nil {
		e.t.Fatalf("app.Start: %v", err)
	}
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

// Client returns a new browser-like client with its own cookie jar.
func (e *Env) Client() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{t: e.t, env: e, http: &http.Client{Jar: jar}}
}

// AdminClient returns a client already logged in as the initial administrator.
func (e *Env) AdminClient() *Client {
	c := e.Client()
	c.LoginLocal(AdminUsername, AdminPassword).ExpectStatus(http.StatusOK)
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

// LoginLocal logs in with a local identity.
func (c *Client) LoginLocal(username, password string) *Response {
	c.t.Helper()
	return c.Post("/api/session", map[string]string{"method": "local", "username": username, "password": password})
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
