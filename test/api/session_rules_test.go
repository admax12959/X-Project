package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/domain"
)

func TestLoginWithWrongPasswordOrUnknownUserGivesTheSameError(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := env.Client()

	wrongPw := c.LoginAs(apitest.AdminUsername, "not-the-password")
	wrongPw.ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	unknown := c.LoginAs("nobody", apitest.AdminPassword)
	unknown.ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	if string(wrongPw.Body) != string(unknown.Body) {
		t.Fatalf("bodies differ: %s vs %s", wrongPw.Body, unknown.Body)
	}
	if len(wrongPw.Cookies) != 0 {
		t.Fatalf("failed login set cookies: %v", wrongPw.Cookies)
	}
	c.Get("/api/session").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}

func TestLoginWithBlankFieldsReportsEachField(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	res := env.Client().Post("/api/session", map[string]string{"method": "local", "username": " ", "password": ""})
	res.ExpectError(http.StatusUnprocessableEntity, "username", "required")
	res.ExpectError(http.StatusUnprocessableEntity, "password", "required")
}

func TestLoginWithUnsupportedMethodIsRejected(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Client().Post("/api/session", map[string]string{"method": "kerberos", "username": "admin", "password": "x"}).
		ExpectError(http.StatusUnprocessableEntity, "method", "invalid")
}

func TestMalformedJSONIsABadRequest(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Client().Post("/api/session", "not an object").ExpectError(http.StatusBadRequest, "", "badRequest")
}

func TestSessionCookieIsHttpOnly(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	res := env.Client().LoginAs(apitest.AdminUsername, apitest.AdminPassword).ExpectStatus(http.StatusOK)
	if len(res.Cookies) != 1 || !res.Cookies[0].HttpOnly || res.Cookies[0].Value == "" {
		t.Fatalf("cookies = %+v", res.Cookies)
	}
}

func TestLogoutInvalidatesTheSessionOnTheServer(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := env.AdminClient()
	old := c.Cookies()

	c.Delete("/api/session").ExpectStatus(http.StatusNoContent)
	c.Get("/api/session").ExpectError(http.StatusUnauthorized, "", "unauthenticated")

	// Replaying the old cookie must not work either.
	c.WithCookies(old).Get("/api/session").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}

// registeredRoutes reads the route table (every s.route / s.publicRoute call) out of the
// httpapi sources, so a new route is covered by the test below without anyone remembering it.
func registeredRoutes(t *testing.T) (all, public []string) {
	t.Helper()
	files, err := filepath.Glob("../../internal/httpapi/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("httpapi sources not found: %v", err)
	}
	re := regexp.MustCompile(`s\.(route|publicRoute)\("([A-Z]+ /api/[^"]*)"`)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			all = append(all, m[2])
			if m[1] == "publicRoute" {
				public = append(public, m[2])
			}
		}
	}
	return all, public
}

func TestEveryAPIRouteRequiresASessionUnlessPublic(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := env.Client()
	routes, public := registeredRoutes(t)

	// The only routes reachable without a session. Adding one is a deliberate decision.
	wantPublic := []string{"GET /api/healthz", "GET /api/login-methods", "POST /api/password-recovery", "POST /api/password-setup", "POST /api/session"}
	slices.Sort(public)
	if !slices.Equal(public, wantPublic) {
		t.Fatalf("public routes = %v, want %v", public, wantPublic)
	}
	if len(routes) < 30 {
		t.Fatalf("found only %d routes in the httpapi sources; has the registration style changed?", len(routes))
	}

	pathParam := regexp.MustCompile(`\{[^}]+\}`)
	for _, route := range routes {
		if slices.Contains(public, route) {
			continue
		}
		method, path, _ := strings.Cut(route, " ")
		path = pathParam.ReplaceAllString(path, "1")
		c.Do(method, path, map[string]any{}).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	}
	for _, path := range []string{"/api/session", "/api/no-such-route"} {
		c.Get(path).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	}
	c.Get("/api/healthz").ExpectStatus(http.StatusOK)

	env.AdminClient().Get("/api/no-such-route").ExpectError(http.StatusNotFound, "", "notFound")
}

func TestRestartDoesNotRecreateTheInitialAdministrator(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	var first sessionBody
	env.AdminClient().Get("/api/session").JSON(&first)

	env.Restart(func(c *app.Config) { c.Admin.Password = "a-different-password" })

	c := env.Client()
	c.LoginAs(apitest.AdminUsername, "a-different-password").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	// The administrator has a second factor (always, #23): the password opens the second step.
	c.LoginAs(apitest.AdminUsername, apitest.AdminPassword).ExpectStatus(http.StatusOK)
	env.Clock.Advance(2 * time.Minute) // the code of the enrolment's step is spent
	var again sessionBody
	c.Post("/api/session/second-factor", map[string]string{"code": apitest.TOTPCode(env.AdminTOTPSecret(), env.Clock.Now())}).ExpectStatus(http.StatusOK).JSON(&again)
	if again.User.ID != first.User.ID {
		t.Fatalf("admin id changed: %s → %s", first.User.ID, again.User.ID)
	}
}

func TestRestartWithoutAdminConfigurationStillStarts(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Restart(func(c *app.Config) { c.Admin.Username, c.Admin.Email, c.Admin.Password = "", "", "" })
	env.AdminClient().Get("/api/session").ExpectStatus(http.StatusOK)
}

func TestFirstStartWithoutAdminConfigurationFails(t *testing.T) {
	t.Parallel()
	// The password, the username and the login email are required on the very first start,
	// and the refusal is the bootstrap configuration error, not some other failure.
	for name, mutate := range map[string]func(*app.Config){
		"password": func(c *app.Config) { c.Admin.Password = "" },
		"username": func(c *app.Config) { c.Admin.Username = "" },
		"email":    func(c *app.Config) { c.Admin.Email = "" },
	} {
		if err := apitest.StartError(t, mutate); !errors.Is(err, domain.ErrBootstrapConfig) {
			t.Fatalf("first start without the admin %s: err = %v, want ErrBootstrapConfig", name, err)
		}
	}
}
