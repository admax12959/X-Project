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
	Department  string `json:"directoryDepartment"`
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
	// The administrator's login email is the very address the directory reports for Emma.
	admin := userOf(t, env.Client().LoginEmail("emma.larsen@corp.example", apitest.AdminPassword))
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
	admin := userOf(t, env.Client().LoginEmail(apitest.AdminEmail, apitest.AdminPassword))
	if admin.DisplayName != "Initial Admin" || admin.Email != apitest.AdminEmail {
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

func TestLoginPageCanLearnWhichLoginMethodsExistWithoutASession(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	var body struct {
		Items []string `json:"items"`
	}
	env.Client().Get("/api/login-methods").ExpectStatus(http.StatusOK).JSON(&body)
	if len(body.Items) != 2 || body.Items[0] != "email" || body.Items[1] != "ldap" {
		t.Fatalf("login methods = %v, want [email ldap]", body.Items)
	}
}

func TestFirstLDAPLoginRecordsAnLDAPIdentityBoundByFirstLogin(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))

	emma := findUser(t, listUsers(t, env.AdminClient()), "elarsen")
	if len(emma.Identities) != 1 || emma.Identities[0].Kind != "ldap" || emma.Identities[0].BoundVia != "firstLogin" {
		t.Fatalf("identities = %+v, want one ldap identity bound by firstLogin", emma.Identities)
	}
}

func TestDisabledUserCannotLogInWithLDAPEither(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	emma := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	admin := env.AdminClient()

	admin.Post("/api/users/"+emma.ID+"/disable", nil).ExpectStatus(http.StatusOK)
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")

	admin.Post("/api/users/"+emma.ID+"/enable", nil).ExpectStatus(http.StatusOK)
	if again := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw")); again.ID != emma.ID {
		t.Fatalf("after re-enable the LDAP login opened user %s, want %s", again.ID, emma.ID)
	}
}

func TestRenamingTheLocalUserLetsTheLDAPUserLogInForTheFirstTime(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	// The local user predates the directory account (otherwise the create would be refused).
	local := createUser(t, admin, map[string]any{"username": "elarsen", "displayName": "Eric Larsen", "password": "local-pw-1"})
	addEmma(env)

	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectError(http.StatusUnprocessableEntity, "username", "usernameTakenByLocal")

	patchUser(admin, local.ID, map[string]any{"username": "elarsen2"}).ExpectStatus(http.StatusOK)
	ldap := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	if ldap.ID == local.ID || ldap.Username != "elarsen" || ldap.DisplayName != "Emma Larsen" {
		t.Fatalf("LDAP user = %+v, local = %+v: want a new, separate user", ldap, local)
	}

	// The renamed user keeps logging in with the same login email.
	if got := userOf(t, env.Client().LoginEmail("elarsen@example.com", "local-pw-1")); got.ID != local.ID || got.DisplayName != "Eric Larsen" {
		t.Fatalf("renamed user = %+v, want ID %s", got, local.ID)
	}
}

func TestAdminCannotCreateALocalUserWhoseUsernameExistsInLDAP(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	env.Directory.AddAccount("retired", "pw", domain.DirectoryEntry{DisplayName: "Retired Person"})
	env.Directory.SetDisabled("retired", true)
	admin := env.AdminClient()

	admin.Post("/api/users", newUser(map[string]any{"username": "elarsen"})).
		ExpectError(http.StatusUnprocessableEntity, "username", "usernameExistsInLdap")
	// A disabled directory account still owns its name.
	admin.Post("/api/users", newUser(map[string]any{"username": "retired"})).
		ExpectError(http.StatusUnprocessableEntity, "username", "usernameExistsInLdap")
	if got := len(listUsers(t, admin)); got != 1 {
		t.Fatalf("%d users after refused creates, want only the initial admin", got)
	}

	// A name the directory does not know is fine.
	createUser(t, admin, map[string]any{"username": "contractor1"})
}

func TestCreatingALocalUserFailsWhenTheDirectoryCannotBeAsked(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.SetUnreachable(true)
	admin := env.AdminClient()

	admin.Post("/api/users", newUser(nil)).ExpectError(http.StatusServiceUnavailable, "", "ldapUnreachable")
	if got := len(listUsers(t, admin)); got != 1 {
		t.Fatalf("%d users after a create with an unreachable directory, want only the initial admin", got)
	}

	env.Directory.SetUnreachable(false)
	createUser(t, admin, nil)
}

func TestLocalLoginStillWorksWhenTheDirectoryIsDown(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.SetUnreachable(true)
	env.Client().LoginAs(apitest.AdminUsername, apitest.AdminPassword).ExpectStatus(http.StatusOK)
}
