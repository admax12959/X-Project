package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/domain"
)

type ldapUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Department  string `json:"department"`
	IsAdmin     bool   `json:"isAdmin"`
}

type ldapSession struct {
	User ldapUser `json:"user"`
}

func loginLDAP(c *apitest.Client, username, password string) *apitest.Response {
	return c.Post("/api/session", map[string]string{"method": "ldap", "username": username, "password": password})
}

func addEmma(env *apitest.Env) {
	env.Directory.AddAccount("elarsen", "ldap-pw", domain.DirectoryEntry{
		DisplayName: "Emma Larsen", Email: "emma.larsen@corp.example", Department: "Payments Platform",
	})
}

func userOf(t *testing.T, r *apitest.Response) ldapUser {
	t.Helper()
	var s ldapSession
	r.ExpectStatus(http.StatusOK).JSON(&s)
	return s.User
}

func TestFirstLDAPLoginCreatesTheUserFromDirectoryAttributes(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	c := env.Client()

	s := userOf(t, loginLDAP(c, "elarsen", "ldap-pw"))
	want := ldapUser{Username: "elarsen", DisplayName: "Emma Larsen", Email: "emma.larsen@corp.example", Department: "Payments Platform"}
	got := s
	got.ID = ""
	if got != want || s.ID == "" {
		t.Fatalf("user = %+v, want %+v", s, want)
	}

	me := userOf(t, c.Get("/api/session"))
	if me != s {
		t.Fatalf("GET /api/session = %+v, want %+v", me, s)
	}
}

func TestEveryLDAPLoginOverwritesProfileFromTheDirectory(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	first := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))

	env.Directory.AddAccount("elarsen", "ldap-pw", domain.DirectoryEntry{
		DisplayName: "Emma Larsen-Brooks", Email: "emma.lb@corp.example", Department: "Risk",
	})
	second := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))

	want := first
	want.DisplayName, want.Email, want.Department = "Emma Larsen-Brooks", "emma.lb@corp.example", "Risk"
	if second != want {
		t.Fatalf("second login user = %+v, want %+v", second, want)
	}
}

func TestLDAPUserWithSameEmailAsLocalUserIsNotMerged(t *testing.T) {
	t.Parallel()
	env := apitest.New(t, func(c *app.Config) { c.Admin.Email = "emma.larsen@corp.example" })
	addEmma(env)

	ldap := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	if ldap.IsAdmin || ldap.Username != "elarsen" {
		t.Fatalf("LDAP user = %+v, want a separate non-admin user elarsen", ldap)
	}
	admin := userOf(t, env.Client().LoginLocal(apitest.AdminUsername, apitest.AdminPassword))
	if admin.ID == ldap.ID || admin.DisplayName != "Initial Admin" || !admin.IsAdmin {
		t.Fatalf("admin = %+v, ldap = %+v: they must be two unrelated users", admin, ldap)
	}
}

func TestFirstLDAPLoginIsRefusedWhenALocalUserOwnsTheUsername(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.AddAccount(apitest.AdminUsername, "ldap-pw", domain.DirectoryEntry{DisplayName: "Someone Else", Email: "x@corp.example"})

	c := env.Client()
	loginLDAP(c, apitest.AdminUsername, "ldap-pw").ExpectError(http.StatusUnprocessableEntity, "username", "usernameTakenByLocal")
	c.Get("/api/session").ExpectStatus(http.StatusUnauthorized)

	// Nothing was bound: the same refusal again, and the local admin is untouched.
	loginLDAP(env.Client(), apitest.AdminUsername, "ldap-pw").ExpectError(http.StatusUnprocessableEntity, "username", "usernameTakenByLocal")
	admin := userOf(t, env.Client().LoginLocal(apitest.AdminUsername, apitest.AdminPassword))
	if admin.DisplayName != "Initial Admin" || admin.Email != "" {
		t.Fatalf("local admin was modified: %+v", admin)
	}
}

func TestLDAPLoginFailsWhenTheDirectoryAccountIsGoneOrDisabledAndRecoversWithTheSameUser(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	first := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))

	env.Directory.SetDisabled("elarsen", true)
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")

	env.Directory.SetDisabled("elarsen", false)
	env.Directory.RemoveAccount("elarsen")
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")

	// The user and the LDAP identity survived: the account coming back signs in to the same user.
	addEmma(env)
	if again := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw")); again != first {
		t.Fatalf("user after recovery = %+v, want %+v", again, first)
	}
}

func TestLDAPLoginWithWrongPasswordOrUnknownAccountFails(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	loginLDAP(env.Client(), "elarsen", "nope").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	loginLDAP(env.Client(), "nobody", "ldap-pw").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
}

func TestUnreachableDirectoryIsNeverReportedAsAWrongPassword(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	env.Directory.SetUnreachable(true)
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectError(http.StatusServiceUnavailable, "", "ldapUnreachable")
}

func TestLocalLoginStillWorksWhenTheDirectoryIsDown(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.SetUnreachable(true)
	env.Client().LoginLocal(apitest.AdminUsername, apitest.AdminPassword).ExpectStatus(http.StatusOK)
}
