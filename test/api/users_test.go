package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// userBody mirrors the administrator view of a user (GET /api/users items).
type userBody struct {
	ID                  string `json:"id"`
	Username            string `json:"username"`
	DisplayName         string `json:"displayName"`
	Email               string `json:"email"`
	Phone               string `json:"phone"`
	DirectoryDepartment string `json:"directoryDepartment"`
	Status              string `json:"status"`
	IsAdmin             bool   `json:"isAdmin"`
	Identities          []struct {
		Kind      string `json:"kind"`
		LoginName string `json:"loginName"`
		BoundVia  string `json:"boundVia"`
	} `json:"identities"`
}

type userListBody struct {
	Items []userBody `json:"items"`
}

// newUser is a valid create-user request; tests override single fields. "password" is the
// password the user is meant to end up with; the login email defaults to <username>@example.com.
func newUser(over map[string]any) map[string]any {
	m := map[string]any{
		"username":    "jdoe",
		"displayName": "Jane Doe",
		"password":    "initial-pw-1",
	}
	for k, v := range over {
		m[k] = v
	}
	if _, ok := m["loginEmail"]; !ok {
		m["loginEmail"] = apitest.Email(m["username"].(string))
	}
	return m
}

// createUserRaw creates a user the way an administrator does: the password in the body is
// the temporary one, and the user has to change it at the first login.
func createUserRaw(t *testing.T, c *apitest.Client, over map[string]any) userBody {
	t.Helper()
	var u userBody
	c.Post("/api/users", newUser(over)).ExpectStatus(http.StatusCreated).JSON(&u)
	return u
}

// createUser creates a user and completes their first login, so that the password given
// in the body is the one that works and the user is free to use the API. Tests of the
// first login itself use createUserRaw.
func createUser(t *testing.T, c *apitest.Client, over map[string]any) userBody {
	t.Helper()
	body := newUser(over)
	final := body["password"].(string)
	temp := final + "-temp"
	body["password"] = temp
	var u userBody
	c.Post("/api/users", body).ExpectStatus(http.StatusCreated).JSON(&u)
	first := c.Env().Client()
	first.LoginEmail(body["loginEmail"].(string), temp).ExpectStatus(http.StatusOK)
	first.Put("/api/me/password", map[string]string{"currentPassword": temp, "newPassword": final}).ExpectStatus(http.StatusNoContent)
	return u
}

func listUsers(t *testing.T, c *apitest.Client) []userBody {
	t.Helper()
	var l userListBody
	c.Get("/api/users").ExpectStatus(http.StatusOK).JSON(&l)
	return l.Items
}

func findUser(t *testing.T, users []userBody, username string) userBody {
	t.Helper()
	for _, u := range users {
		if u.Username == username {
			return u
		}
	}
	t.Fatalf("user %q not in list %+v", username, users)
	return userBody{}
}

func TestAdminCreatesAnEmailUserWhoCanThenLogInWithTheLoginEmail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	created := createUserRaw(t, admin, nil)
	if created.ID == "" || created.Username != "jdoe" || created.DisplayName != "Jane Doe" ||
		created.Email != "jdoe@example.com" || created.DirectoryDepartment != "" ||
		created.Status != "enabled" || created.IsAdmin {
		t.Fatalf("created = %+v", created)
	}
	if len(created.Identities) != 1 || created.Identities[0].Kind != "email" || created.Identities[0].LoginName != "jdoe@example.com" {
		t.Fatalf("identities = %+v", created.Identities)
	}

	env.Client().LoginEmail("jdoe@example.com", "initial-pw-1").ExpectStatus(http.StatusOK)
	env.Client().LoginEmail("jdoe@example.com", "wrong-password").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	// The username is a display name, not a way to log in; the email address is not case sensitive.
	env.Client().LoginEmail("jdoe", "initial-pw-1").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	env.Client().LoginEmail("JDoe@Example.COM", "initial-pw-1").ExpectStatus(http.StatusOK)
}

func TestUserListShowsFieldsStatusAndLoginIdentityKinds(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	createUser(t, admin, nil)

	users := listUsers(t, admin)
	if len(users) != 2 {
		t.Fatalf("users = %+v", users)
	}
	jane := findUser(t, users, "jdoe")
	if jane.DisplayName != "Jane Doe" || jane.Email != "jdoe@example.com" || jane.Status != "enabled" {
		t.Fatalf("jane = %+v", jane)
	}
	if len(jane.Identities) != 1 || jane.Identities[0].Kind != "email" {
		t.Fatalf("identities = %+v", jane.Identities)
	}
	root := findUser(t, users, apitest.AdminUsername)
	if !root.IsAdmin || len(root.Identities) != 1 || root.Identities[0].Kind != "email" {
		t.Fatalf("admin = %+v", root)
	}
}

func TestNewUsersLocalIdentityIsBoundByAnAdministrator(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	createUser(t, admin, nil)

	users := listUsers(t, admin)
	if got := findUser(t, users, "jdoe").Identities[0].BoundVia; got != "admin" {
		t.Fatalf("boundVia = %q, want admin", got)
	}
	if got := findUser(t, users, apitest.AdminUsername).Identities[0].BoundVia; got != "bootstrap" {
		t.Fatalf("initial admin boundVia = %q, want bootstrap", got)
	}
}

func TestCreatingAUserReportsEveryInvalidFieldAtOnce(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	res := admin.Post("/api/users", map[string]any{"username": " ", "displayName": " ", "loginEmail": " ", "password": ""})
	res.ExpectError(http.StatusUnprocessableEntity, "username", "required")
	res.ExpectError(http.StatusUnprocessableEntity, "loginEmail", "required")
	res.ExpectError(http.StatusUnprocessableEntity, "displayName", "required")
	res.ExpectError(http.StatusUnprocessableEntity, "password", "required")

	if got := len(listUsers(t, admin)); got != 1 {
		t.Fatalf("a rejected create left %d users, want only the initial admin", got)
	}
}

func TestUsernameFormatIsEnforced(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	bad := []string{
		"J.Doe",                             // uppercase
		"1doe",                              // must start with a letter
		"a",                                 // too short
		"jane doe",                          // space
		"jane@corp",                         // character outside [a-z0-9._-]
		"abcdefghijklmnopqrstuvwxyz0123456", // 33 characters
	}
	for _, name := range bad {
		admin.Post("/api/users", newUser(map[string]any{"username": name})).
			ExpectError(http.StatusUnprocessableEntity, "username", "invalid")
	}
	// Boundaries that are valid: 2 and 32 characters, and the allowed punctuation.
	createUser(t, admin, map[string]any{"username": "ab"})
	createUser(t, admin, map[string]any{"username": "a.b_c-d"})
	createUser(t, admin, map[string]any{"username": "abcdefghijklmnopqrstuvwxyz012345"})
}

func TestInitialPasswordNeedsAtLeastEightCharacters(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	e := admin.Post("/api/users", newUser(map[string]any{"password": "1234567"})).
		ExpectError(http.StatusUnprocessableEntity, "password", "tooShort")
	if e.Params["min"] != float64(8) {
		t.Fatalf("params = %v, want min 8", e.Params)
	}
	createUser(t, admin, map[string]any{"password": "12345678"})
}

func TestLoginEmailIsRequiredAndMustLookLikeAnEmail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	admin.Post("/api/users", newUser(map[string]any{"loginEmail": "not-an-email"})).
		ExpectError(http.StatusUnprocessableEntity, "loginEmail", "invalid")
	admin.Post("/api/users", newUser(map[string]any{"loginEmail": "a b@example.com"})).
		ExpectError(http.StatusUnprocessableEntity, "loginEmail", "invalid")
	admin.Post("/api/users", newUser(map[string]any{"loginEmail": ""})).
		ExpectError(http.StatusUnprocessableEntity, "loginEmail", "required")
	long := strings.Repeat("x", 320) + "@example.com"
	admin.Post("/api/users", newUser(map[string]any{"loginEmail": long})).
		ExpectError(http.StatusUnprocessableEntity, "loginEmail", "tooLong")

	// The login email is the contact email.
	u := createUser(t, admin, map[string]any{"loginEmail": "  Jane.Doe@Example.com "})
	if u.Email != "jane.doe@example.com" || u.Identities[0].LoginName != "jane.doe@example.com" {
		t.Fatalf("created = %+v", u)
	}
}

func TestOverlongFieldsAreRejected(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	long := func(n int) string { return strings.Repeat("x", n) }
	res := admin.Post("/api/users", newUser(map[string]any{
		"displayName": long(201), "phone": long(65),
	}))
	e := res.ExpectError(http.StatusUnprocessableEntity, "displayName", "tooLong")
	if e.Params["max"] != float64(200) {
		t.Fatalf("params = %v, want max 200", e.Params)
	}
	res.ExpectError(http.StatusUnprocessableEntity, "phone", "tooLong")
}

func TestUsernameMustBeUnique(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	createUser(t, admin, nil)

	admin.Post("/api/users", newUser(nil)).ExpectError(http.StatusUnprocessableEntity, "username", "usernameTaken")
	admin.Post("/api/users", newUser(map[string]any{"username": apitest.AdminUsername})).
		ExpectError(http.StatusUnprocessableEntity, "username", "usernameTaken")

	if got := len(listUsers(t, admin)); got != 2 {
		t.Fatalf("users = %d, want 2", got)
	}
}

func TestLoginEmailMustBeUniqueWhateverItsCase(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	createUser(t, admin, nil)

	admin.Post("/api/users", newUser(map[string]any{"username": "other", "loginEmail": "jdoe@example.com"})).
		ExpectError(http.StatusUnprocessableEntity, "loginEmail", "loginEmailTaken")
	admin.Post("/api/users", newUser(map[string]any{"username": "other", "loginEmail": "JDOE@EXAMPLE.COM"})).
		ExpectError(http.StatusUnprocessableEntity, "loginEmail", "loginEmailTaken")
	admin.Post("/api/users", newUser(map[string]any{"username": "other", "loginEmail": apitest.AdminEmail})).
		ExpectError(http.StatusUnprocessableEntity, "loginEmail", "loginEmailTaken")

	if got := len(listUsers(t, admin)); got != 2 {
		t.Fatalf("a rejected create left %d users, want 2", got)
	}
}

func TestOnlyAdministratorsMayManageUsers(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	createUser(t, admin, nil)

	jane := env.Client()
	jane.LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	jane.Get("/api/users").ExpectError(http.StatusForbidden, "", "forbidden")
	jane.Post("/api/users", newUser(map[string]any{"username": "other"})).ExpectError(http.StatusForbidden, "", "forbidden")

	if got := len(listUsers(t, admin)); got != 2 {
		t.Fatalf("a forbidden create changed the user list: %d users", got)
	}
}

// patchUser sends PATCH /api/users/{id}; only the given fields change.
func patchUser(c *apitest.Client, id string, fields map[string]any) *apitest.Response {
	return c.Do(http.MethodPatch, "/api/users/"+id, fields)
}

func TestAdminEditsDisplayNameAndPhone(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)

	var edited userBody
	patchUser(admin, jane.ID, map[string]any{
		"displayName": " Jane Q. Doe ", "phone": "555-0100",
	}).ExpectStatus(http.StatusOK).JSON(&edited)
	if edited.ID != jane.ID || edited.DisplayName != "Jane Q. Doe" || edited.Email != "jdoe@example.com" ||
		edited.Phone != "555-0100" || edited.Username != "jdoe" {
		t.Fatalf("edited = %+v", edited)
	}
	if got := findUser(t, listUsers(t, admin), "jdoe"); got.DisplayName != "Jane Q. Doe" || got.Phone != "555-0100" {
		t.Fatalf("list after edit = %+v", got)
	}

	// Fields that are left out stay as they were; optional fields can be cleared.
	patchUser(admin, jane.ID, map[string]any{"phone": ""}).ExpectStatus(http.StatusOK).JSON(&edited)
	if edited.DisplayName != "Jane Q. Doe" || edited.Phone != "" {
		t.Fatalf("after partial edit = %+v", edited)
	}
}

func TestEditingAUserUsesTheSameFieldRulesAsCreating(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)

	res := patchUser(admin, jane.ID, map[string]any{
		"displayName": " ", "loginEmail": "not-an-email", "phone": strings.Repeat("9", 65),
	})
	res.ExpectError(http.StatusUnprocessableEntity, "displayName", "required")
	res.ExpectError(http.StatusUnprocessableEntity, "loginEmail", "invalid")
	res.ExpectError(http.StatusUnprocessableEntity, "phone", "tooLong")

	if got := findUser(t, listUsers(t, admin), "jdoe"); got.DisplayName != "Jane Doe" || got.Email != "jdoe@example.com" {
		t.Fatalf("a rejected edit changed the user: %+v", got)
	}
}

func TestRenamingAUserKeepsTheSameUserAndTheLoginEmail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)

	// A session opened before the rename keeps working: it refers to the user ID.
	open := env.Client()
	open.LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)

	var renamed userBody
	patchUser(admin, jane.ID, map[string]any{"username": " jane.doe "}).ExpectStatus(http.StatusOK).JSON(&renamed)
	if renamed.ID != jane.ID || renamed.Username != "jane.doe" || renamed.DisplayName != "Jane Doe" {
		t.Fatalf("renamed = %+v", renamed)
	}

	var me struct {
		User userBody `json:"user"`
	}
	open.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&me)
	if me.User.ID != jane.ID || me.User.Username != "jane.doe" {
		t.Fatalf("existing session now sees %+v", me.User)
	}

	// The login email is not the username: it is the same after the rename, and the old
	// username is free again for somebody else.
	env.Client().LoginEmail("jdoe@example.com", "initial-pw-1").ExpectStatus(http.StatusOK)
	users := listUsers(t, admin)
	if got := findUser(t, users, "jane.doe"); got.ID != jane.ID || len(got.Identities) != 1 ||
		got.Identities[0].Kind != "email" || got.Identities[0].LoginName != "jdoe@example.com" {
		t.Fatalf("list after rename = %+v", got)
	}
	other := createUser(t, admin, map[string]any{"username": "jdoe", "displayName": "John Doe", "loginEmail": "john@example.com"})
	if other.ID == jane.ID {
		t.Fatal("a new user reusing the old name must be a different user")
	}
	env.Client().LoginEmail("john@example.com", "initial-pw-1").ExpectStatus(http.StatusOK)
	env.Client().LoginEmail("jdoe@example.com", "initial-pw-1").ExpectStatus(http.StatusOK)
}

func TestRenameFollowsTheSameRulesAsCreation(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)
	createUser(t, admin, map[string]any{"username": "taken"})

	patchUser(admin, jane.ID, map[string]any{"username": "Bad Name"}).ExpectError(http.StatusUnprocessableEntity, "username", "invalid")
	patchUser(admin, jane.ID, map[string]any{"username": " "}).ExpectError(http.StatusUnprocessableEntity, "username", "required")
	patchUser(admin, jane.ID, map[string]any{"username": "taken"}).ExpectError(http.StatusUnprocessableEntity, "username", "usernameTaken")
	patchUser(admin, jane.ID, map[string]any{"username": apitest.AdminUsername}).ExpectError(http.StatusUnprocessableEntity, "username", "usernameTaken")

	// Nothing changed, and sending the current username is not a rename.
	env.Client().LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	patchUser(admin, jane.ID, map[string]any{"username": "jdoe", "phone": "555-0100"}).ExpectStatus(http.StatusOK)
}

func TestOnlyAdministratorsMayEditOrDisableUsers(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)
	bob := createUser(t, admin, map[string]any{"username": "bob", "displayName": "Bob"})

	janeClient := env.Client()
	janeClient.LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	for _, id := range []string{jane.ID, bob.ID} {
		patchUser(janeClient, id, map[string]any{"displayName": "Hacked"}).ExpectError(http.StatusForbidden, "", "forbidden")
		janeClient.Post("/api/users/"+id+"/disable", nil).ExpectError(http.StatusForbidden, "", "forbidden")
		janeClient.Post("/api/users/"+id+"/enable", nil).ExpectError(http.StatusForbidden, "", "forbidden")
	}

	for _, u := range listUsers(t, admin) {
		if u.DisplayName == "Hacked" || u.Status != "enabled" {
			t.Fatalf("a forbidden request changed %+v", u)
		}
	}
}

func TestDisabledUserCannotLogInUntilEnabledAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)

	// An already open session ends as soon as the user is disabled.
	open := env.Client()
	open.LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)

	var disabled userBody
	admin.Post("/api/users/"+jane.ID+"/disable", nil).ExpectStatus(http.StatusOK).JSON(&disabled)
	if disabled.Status != "disabled" || disabled.ID != jane.ID {
		t.Fatalf("disabled = %+v", disabled)
	}
	if got := findUser(t, listUsers(t, admin), "jdoe").Status; got != "disabled" {
		t.Fatalf("list status = %q", got)
	}
	env.Client().LoginAs("jdoe", "initial-pw-1").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	open.Get("/api/session").ExpectError(http.StatusUnauthorized, "", "unauthenticated")

	// Disabling twice is harmless, and a disabled user can still be edited.
	admin.Post("/api/users/"+jane.ID+"/disable", nil).ExpectStatus(http.StatusOK)
	patchUser(admin, jane.ID, map[string]any{"displayName": "Jane D."}).ExpectStatus(http.StatusOK)

	var enabled userBody
	admin.Post("/api/users/"+jane.ID+"/enable", nil).ExpectStatus(http.StatusOK).JSON(&enabled)
	if enabled.Status != "enabled" {
		t.Fatalf("enabled = %+v", enabled)
	}
	env.Client().LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
}

func TestDisablingOrEnablingAnUnknownUserIsNotFound(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	admin.Post("/api/users/99999/disable", nil).ExpectError(http.StatusNotFound, "", "notFound")
	admin.Post("/api/users/99999/enable", nil).ExpectError(http.StatusNotFound, "", "notFound")
}

func TestThereIsNoWayToDeleteAUser(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)

	// DELETE is not routed (404 for the administrator, 401 when logged out)...
	admin.Delete("/api/users/"+jane.ID).ExpectError(http.StatusNotFound, "", "notFound")
	env.Client().Delete("/api/users/"+jane.ID).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	// ...and the user is untouched.
	if got := findUser(t, listUsers(t, admin), "jdoe"); got.ID != jane.ID {
		t.Fatalf("user changed: %+v", got)
	}
}

func TestEditingAnUnknownUserIsNotFound(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	patchUser(admin, "99999", map[string]any{"displayName": "x"}).ExpectError(http.StatusNotFound, "", "notFound")
	patchUser(admin, "abc", map[string]any{"displayName": "x"}).ExpectError(http.StatusNotFound, "", "notFound")
}
