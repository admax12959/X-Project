package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/domain"
)

func domainEntry(name, email string) domain.DirectoryEntry {
	return domain.DirectoryEntry{DisplayName: name, Email: email, Department: "Payments Platform"}
}

// loginState is the answer of POST /api/session and GET /api/session.
type loginState struct {
	User               ldapUser `json:"user"`
	MustChangePassword bool     `json:"mustChangePassword"`
}

func changePassword(c *apitest.Client, current, next string) *apitest.Response {
	return c.Put("/api/me/password", map[string]string{"currentPassword": current, "newPassword": next})
}

// ---------------------------------------------------------------- login

func TestEmailLoginNeedsTheEmailAndTheLDAPLoginNeedsTheUsername(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)

	c := env.Client()
	c.Post("/api/session", map[string]string{"method": "email", "password": "x"}).ExpectError(http.StatusUnprocessableEntity, "email", "required")
	// The username is no substitute for the email.
	c.Post("/api/session", map[string]string{"method": "email", "username": apitest.AdminUsername, "password": apitest.AdminPassword}).
		ExpectError(http.StatusUnprocessableEntity, "email", "required")
	c.Post("/api/session", map[string]string{"method": "ldap", "email": "emma.larsen@corp.example", "password": "ldap-pw"}).
		ExpectError(http.StatusUnprocessableEntity, "username", "required")
	c.Post("/api/session", map[string]string{"method": "local", "username": apitest.AdminUsername, "password": apitest.AdminPassword}).
		ExpectError(http.StatusUnprocessableEntity, "method", "invalid")
}

func TestUnknownEmailAndWrongPasswordGiveTheSameAnswer(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	wrongPw := env.Client().LoginEmail(apitest.AdminEmail, "not-the-password")
	unknown := env.Client().LoginEmail("nobody@example.com", apitest.AdminPassword)
	wrongPw.ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	unknown.ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	if string(wrongPw.Body) != string(unknown.Body) {
		t.Fatalf("answers differ: %s vs %s", wrongPw.Body, unknown.Body)
	}
}

// ---------------------------------------------------------------- the initial administrator

func TestInitialAdministratorLogsInWithTheConfiguredEmailAndIsNotAskedToChangeThePassword(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	var s loginState
	env.Client().LoginEmail(apitest.AdminEmail, apitest.AdminPassword).ExpectStatus(http.StatusOK).JSON(&s)
	if !s.User.IsAdmin || s.User.Email != apitest.AdminEmail || s.MustChangePassword {
		t.Fatalf("login = %+v", s)
	}
	// The username does not open the door.
	env.Client().LoginEmail(apitest.AdminUsername, apitest.AdminPassword).ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
}

func TestFirstStartNeedsAValidAdministratorEmail(t *testing.T) {
	t.Parallel()
	for name, email := range map[string]string{"missing": "", "malformed": "admin-at-example"} {
		email := email
		if err := apitest.StartError(t, func(c *app.Config) { c.Admin.Email = email }); err == nil {
			t.Fatalf("first start with a %s administrator email started", name)
		}
	}
}

// ---------------------------------------------------------------- the temporary password

func TestUserWithATemporaryPasswordCanOnlyChangeItBeforeDoingAnythingElse(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	createUserRaw(t, env.AdminClient(), map[string]any{"username": "jdoe", "password": "temp-pw-0001"})

	jane := env.Client()
	var s loginState
	jane.LoginEmail("jdoe@example.com", "temp-pw-0001").ExpectStatus(http.StatusOK).JSON(&s)
	if !s.MustChangePassword {
		t.Fatalf("login = %+v, want mustChangePassword", s)
	}

	// Everything else is closed, the session itself is not.
	for _, path := range []string{"/api/applications", "/api/me", "/api/versions", "/api/users"} {
		jane.Get(path).ExpectError(http.StatusForbidden, "", "passwordChangeRequired")
	}
	jane.Post("/api/applications", map[string]any{}).ExpectError(http.StatusForbidden, "", "passwordChangeRequired")
	var again loginState
	jane.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&again)
	if again.User.Username != "jdoe" || !again.MustChangePassword {
		t.Fatalf("GET /api/session = %+v", again)
	}

	changePassword(jane, "temp-pw-0001", "my-own-pw-0002").ExpectStatus(http.StatusNoContent)

	// The same session is free from then on, and so is every later login.
	jane.Get("/api/applications").ExpectStatus(http.StatusOK)
	jane.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&again)
	if again.MustChangePassword {
		t.Fatalf("GET /api/session after the change = %+v", again)
	}
	env.Client().LoginEmail("jdoe@example.com", "temp-pw-0001").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	var next loginState
	env.Client().LoginEmail("jdoe@example.com", "my-own-pw-0002").ExpectStatus(http.StatusOK).JSON(&next)
	if next.MustChangePassword {
		t.Fatalf("login after the change = %+v", next)
	}
}

func TestChangingThePasswordFreesEveryOpenSessionOpenedWithTheTemporaryOne(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	createUserRaw(t, env.AdminClient(), map[string]any{"username": "jdoe", "password": "temp-pw-0001"})

	laptop, phone := env.Client(), env.Client()
	laptop.LoginEmail("jdoe@example.com", "temp-pw-0001").ExpectStatus(http.StatusOK)
	phone.LoginEmail("jdoe@example.com", "temp-pw-0001").ExpectStatus(http.StatusOK)

	changePassword(laptop, "temp-pw-0001", "my-own-pw-0002").ExpectStatus(http.StatusNoContent)
	phone.Get("/api/applications").ExpectStatus(http.StatusOK)
}

func TestAnAdministratorBoundEmailIdentityAlsoStartsWithATemporaryPassword(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	emma := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	adminBind(env.AdminClient(), emma.ID, map[string]any{"kind": "email", "loginName": "emma@example.com", "password": "temp-pw-0001"}).
		ExpectStatus(http.StatusCreated)

	c := env.Client()
	c.LoginEmail("emma@example.com", "temp-pw-0001").ExpectStatus(http.StatusOK)
	c.Get("/api/applications").ExpectError(http.StatusForbidden, "", "passwordChangeRequired")
	changePassword(c, "temp-pw-0001", "emma-own-pw-0002").ExpectStatus(http.StatusNoContent)
	c.Get("/api/applications").ExpectStatus(http.StatusOK)
}

func TestTheTemporaryPasswordSessionCanStillLogOut(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	createUserRaw(t, env.AdminClient(), map[string]any{"username": "jdoe", "password": "temp-pw-0001"})
	c := env.Client()
	c.Do(http.MethodPost, "/api/session", map[string]string{"method": "email", "email": "jdoe@example.com", "password": "temp-pw-0001"}).ExpectStatus(http.StatusOK)
	c.Delete("/api/session").ExpectStatus(http.StatusNoContent)
	c.Get("/api/session").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}

// ---------------------------------------------------------------- changing the password

func TestChangingOwnPasswordNeedsTheCurrentOneAndAValidNewOne(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane, _ := localJane(t, env)

	changePassword(jane, "", "another-pw-123").ExpectError(http.StatusUnprocessableEntity, "currentPassword", "required")
	changePassword(jane, "not-my-password", "another-pw-123").ExpectError(http.StatusUnprocessableEntity, "currentPassword", "invalidCredentials")
	changePassword(jane, "jane-pw-123", "").ExpectError(http.StatusUnprocessableEntity, "newPassword", "required")
	e := changePassword(jane, "jane-pw-123", "short").ExpectError(http.StatusUnprocessableEntity, "newPassword", "tooShort")
	if e.Params["min"] != float64(8) {
		t.Fatalf("params = %v, want min 8", e.Params)
	}
	changePassword(jane, "jane-pw-123", "jane-pw-123").ExpectError(http.StatusUnprocessableEntity, "newPassword", "passwordUnchanged")
	env.Client().LoginAs("jdoe", "jane-pw-123").ExpectStatus(http.StatusOK)

	changePassword(jane, "jane-pw-123", "another-pw-123").ExpectStatus(http.StatusNoContent)
	env.Client().LoginAs("jdoe", "jane-pw-123").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	env.Client().LoginAs("jdoe", "another-pw-123").ExpectStatus(http.StatusOK)
}

func TestAUserWithoutAnEmailIdentityHasNoPasswordToChange(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	changePassword(emma, "ldap-pw", "something-else-1").ExpectError(http.StatusNotFound, "", "notFound")
}

// ---------------------------------------------------------------- the administrator changes a login email

func TestAdministratorChangesALoginEmailAndTheContactEmailFollows(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)

	open := env.Client()
	open.LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)

	var edited userBody
	patchUser(admin, jane.ID, map[string]any{"loginEmail": " Jane.New@Example.com "}).ExpectStatus(http.StatusOK).JSON(&edited)
	if edited.Email != "jane.new@example.com" || len(edited.Identities) != 1 || edited.Identities[0].LoginName != "jane.new@example.com" {
		t.Fatalf("edited = %+v", edited)
	}
	env.Client().LoginEmail("jane.new@example.com", "initial-pw-1").ExpectStatus(http.StatusOK)
	env.Client().LoginEmail("jdoe@example.com", "initial-pw-1").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	// A session opened before the change belongs to the same user and keeps working.
	open.Get("/api/applications").ExpectStatus(http.StatusOK)
	if me := getMe(t, open); me.Email != "jane.new@example.com" {
		t.Fatalf("me = %+v", me)
	}
	// The old address is free again.
	createUser(t, admin, map[string]any{"username": "other", "loginEmail": "jdoe@example.com"})
}

func TestLoginEmailEditRules(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)
	createUser(t, admin, map[string]any{"username": "bob"})
	emma := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))

	patchUser(admin, jane.ID, map[string]any{"loginEmail": "not-an-email"}).ExpectError(http.StatusUnprocessableEntity, "loginEmail", "invalid")
	patchUser(admin, jane.ID, map[string]any{"loginEmail": " "}).ExpectError(http.StatusUnprocessableEntity, "loginEmail", "required")
	patchUser(admin, jane.ID, map[string]any{"loginEmail": "bob@example.com"}).ExpectError(http.StatusUnprocessableEntity, "loginEmail", "loginEmailTaken")
	patchUser(admin, jane.ID, map[string]any{"loginEmail": "BOB@EXAMPLE.COM"}).ExpectError(http.StatusUnprocessableEntity, "loginEmail", "loginEmailTaken")
	// Nothing changed; sending the current address is not a change.
	patchUser(admin, jane.ID, map[string]any{"loginEmail": "jdoe@example.com", "phone": "555-0100"}).ExpectStatus(http.StatusOK)
	env.Client().LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)

	// A directory-only user has no login email to change.
	patchUser(admin, emma.ID, map[string]any{"loginEmail": "emma@example.com"}).ExpectError(http.StatusUnprocessableEntity, "loginEmail", "noEmailIdentity")

	// Only administrators.
	plain := plainUserClient(t, env, "plain")
	patchUser(plain, jane.ID, map[string]any{"loginEmail": "mine@example.com"}).ExpectError(http.StatusForbidden, "", "forbidden")
}

func TestForAnLDAPBoundUserTheLoginEmailIsNotTheContactEmail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	admin := env.AdminClient()
	emma := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	adminBind(admin, emma.ID, map[string]any{"kind": "email", "loginName": "emma@example.com", "password": "temp-pw-0001"}).ExpectStatus(http.StatusCreated)

	var edited userBody
	patchUser(admin, emma.ID, map[string]any{"loginEmail": "emma.new@example.com"}).ExpectStatus(http.StatusOK).JSON(&edited)
	if edited.Email != "emma.larsen@corp.example" {
		t.Fatalf("contact email = %q, want the directory's, untouched by the login email", edited.Email)
	}
	if got := findUser(t, listUsers(t, admin), "elarsen"); got.Identities[1].LoginName != "emma.new@example.com" {
		t.Fatalf("identities = %+v", got.Identities)
	}
	patchUser(admin, emma.ID, map[string]any{"email": "x@example.com"}).ExpectError(http.StatusUnprocessableEntity, "email", "ldapManaged")
}

// A directory login must not change the login email of the user it belongs to.
func TestDirectorySyncDoesNotTouchTheLoginEmail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	admin := env.AdminClient()
	emma := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	adminBind(admin, emma.ID, map[string]any{"kind": "email", "loginName": "emma@example.com", "password": "temp-pw-0001"}).ExpectStatus(http.StatusCreated)

	env.Directory.AddAccount("elarsen", "ldap-pw", domainEntry("Emma Larsen", "emma.moved@corp.example"))
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)

	got := findUser(t, listUsers(t, admin), "elarsen")
	if got.Email != "emma.moved@corp.example" || got.Identities[1].LoginName != "emma@example.com" {
		t.Fatalf("user = %+v, want the contact email synced and the login email untouched", got)
	}
}
