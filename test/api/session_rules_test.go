package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
)

func TestLoginWithWrongPasswordOrUnknownUserGivesTheSameError(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := env.Client()

	wrongPw := c.LoginLocal(apitest.AdminUsername, "not-the-password")
	wrongPw.ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	unknown := c.LoginLocal("nobody", apitest.AdminPassword)
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
	res := env.Client().LoginLocal(apitest.AdminUsername, apitest.AdminPassword).ExpectStatus(http.StatusOK)
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

func TestEveryAPIRouteRequiresASessionUnlessPublic(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := env.Client()
	for _, path := range []string{"/api/session", "/api/applications", "/api/no-such-route"} {
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
	c.LoginLocal(apitest.AdminUsername, "a-different-password").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	var again sessionBody
	c.LoginLocal(apitest.AdminUsername, apitest.AdminPassword).ExpectStatus(http.StatusOK).JSON(&again)
	if again.User.ID != first.User.ID {
		t.Fatalf("admin id changed: %s → %s", first.User.ID, again.User.ID)
	}
}

func TestRestartWithoutAdminConfigurationStillStarts(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Restart(func(c *app.Config) { c.Admin.Username, c.Admin.Password = "", "" })
	env.AdminClient().Get("/api/session").ExpectStatus(http.StatusOK)
}

func TestFirstStartWithoutAdminConfigurationFails(t *testing.T) {
	t.Parallel()
	err := apitest.StartError(t, func(c *app.Config) { c.Admin.Password = "" })
	if err == nil {
		t.Fatal("first start without admin password succeeded")
	}
}
