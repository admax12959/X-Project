package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/domain"
)

// ---------------------------------------------------------------- helpers

func patchMe(c *apitest.Client, fields map[string]any) *apitest.Response {
	return c.Do(http.MethodPatch, "/api/me", fields)
}

// ---------------------------------------------------------------- language preference

func TestLanguagePreferenceIsSavedOnTheUserAndFollowsThemToAnotherDevice(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	createUser(t, env.AdminClient(), map[string]any{"username": "jdoe", "password": "jane-pw-123"})

	var first struct {
		User struct {
			Language string `json:"language"`
		} `json:"user"`
	}
	laptop := env.Client()
	laptop.LoginLocal("jdoe", "jane-pw-123").ExpectStatus(http.StatusOK).JSON(&first)
	if first.User.Language != "" {
		t.Fatalf("language before any choice = %q, want empty (not chosen yet)", first.User.Language)
	}

	patchMe(laptop, map[string]any{"language": "en"}).ExpectStatus(http.StatusOK)

	// Another device: a fresh login and the session both report the saved language.
	var second struct {
		User struct {
			Language string `json:"language"`
		} `json:"user"`
	}
	phone := env.Client()
	phone.LoginLocal("jdoe", "jane-pw-123").ExpectStatus(http.StatusOK).JSON(&second)
	if second.User.Language != "en" {
		t.Fatalf("language after login on another device = %q, want en", second.User.Language)
	}

	patchMe(phone, map[string]any{"language": "zh"}).ExpectStatus(http.StatusOK)
	var third struct {
		User struct {
			Language string `json:"language"`
		} `json:"user"`
	}
	laptop.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&third)
	if third.User.Language != "zh" {
		t.Fatalf("session language = %q, want zh", third.User.Language)
	}
}

func TestLanguageMustBeZhOrEn(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	patchMe(admin, map[string]any{"language": "fr"}).ExpectError(http.StatusUnprocessableEntity, "language", "invalid")
	patchMe(admin, map[string]any{"language": ""}).ExpectError(http.StatusUnprocessableEntity, "language", "invalid")
}

func TestMeEndpointsNeedASession(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	patchMe(env.Client(), map[string]any{"language": "en"}).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	env.Client().Get("/api/me").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}

// ---------------------------------------------------------------- profile and identities

// meBody mirrors GET /api/me.
type meBody struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Department  string `json:"department"`
	Language    string `json:"language"`
	IsAdmin     bool   `json:"isAdmin"`
	Identities  []struct {
		Kind      string `json:"kind"`
		LoginName string `json:"loginName"`
		BoundAt   string `json:"boundAt"`
		BoundVia  string `json:"boundVia"`
	} `json:"identities"`
}

func getMe(t *testing.T, c *apitest.Client) meBody {
	t.Helper()
	var m meBody
	c.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&m)
	return m
}

func TestUserWithOnlyALocalIdentityEditsOwnDisplayNameEmailDepartmentAndPhone(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	createUser(t, env.AdminClient(), map[string]any{"username": "jdoe", "password": "jane-pw-123"})
	jane := env.Client()
	jane.LoginLocal("jdoe", "jane-pw-123").ExpectStatus(http.StatusOK)

	var got meBody
	patchMe(jane, map[string]any{
		"displayName": "Jane D. Doe", "email": "jane.doe@example.com", "department": "Treasury", "phone": "555-0199",
	}).ExpectStatus(http.StatusOK).JSON(&got)
	if got.DisplayName != "Jane D. Doe" || got.Email != "jane.doe@example.com" || got.Department != "Treasury" || got.Phone != "555-0199" {
		t.Fatalf("PATCH /api/me answered %+v", got)
	}
	if again := getMe(t, jane); again.DisplayName != got.DisplayName || again.Email != got.Email ||
		again.Department != got.Department || again.Phone != got.Phone {
		t.Fatalf("GET /api/me = %+v, want the saved values %+v", again, got)
	}
	// The administrator sees the same data.
	if listed := findUser(t, listUsers(t, env.AdminClient()), "jdoe"); listed.DisplayName != "Jane D. Doe" || listed.Phone != "555-0199" {
		t.Fatalf("admin view = %+v", listed)
	}
}

func TestOwnProfileEditsAreValidatedLikeTheAdminsAndCannotChangeUsernameOrRole(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	createUser(t, env.AdminClient(), map[string]any{"username": "jdoe", "password": "jane-pw-123"})
	jane := env.Client()
	jane.LoginLocal("jdoe", "jane-pw-123").ExpectStatus(http.StatusOK)

	patchMe(jane, map[string]any{"displayName": "  "}).ExpectError(http.StatusUnprocessableEntity, "displayName", "required")
	patchMe(jane, map[string]any{"email": "not-an-address"}).ExpectError(http.StatusUnprocessableEntity, "email", "invalid")

	// Username and role are not editable here: the fields are simply ignored.
	patchMe(jane, map[string]any{"username": "someoneelse", "isAdmin": true, "phone": "555-0111"}).ExpectStatus(http.StatusOK)
	if me := getMe(t, jane); me.Username != "jdoe" || me.IsAdmin || me.Phone != "555-0111" {
		t.Fatalf("me = %+v, want username and role untouched and the phone saved", me)
	}
}

func TestLDAPBoundUserCannotEditDisplayNameEmailDepartmentButCanEditPhone(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)

	for _, field := range []string{"displayName", "email", "department"} {
		patchMe(emma, map[string]any{field: "Something Else"}).ExpectError(http.StatusUnprocessableEntity, field, "ldapManaged")
	}
	// All three at once are all reported.
	res := patchMe(emma, map[string]any{"displayName": "A", "email": "a@corp.example", "department": "B"})
	res.ExpectError(http.StatusUnprocessableEntity, "displayName", "ldapManaged")
	res.ExpectError(http.StatusUnprocessableEntity, "email", "ldapManaged")
	res.ExpectError(http.StatusUnprocessableEntity, "department", "ldapManaged")

	// Sending the directory's own values back (a form that submits every field) is fine, and so is the phone.
	patchMe(emma, map[string]any{
		"displayName": "Emma Larsen", "email": "emma.larsen@corp.example", "department": "Payments Platform", "phone": "555-0142",
	}).ExpectStatus(http.StatusOK)
	me := getMe(t, emma)
	if me.DisplayName != "Emma Larsen" || me.Email != "emma.larsen@corp.example" || me.Department != "Payments Platform" || me.Phone != "555-0142" {
		t.Fatalf("me = %+v", me)
	}
}

// ---------------------------------------------------------------- binding an identity yourself

func addJane(env *apitest.Env) {
	env.Directory.AddAccount("jane.doe", "jane-ldap-pw", domain.DirectoryEntry{
		DisplayName: "Jane Doe (Corp)", Email: "jane.doe@corp.example", Department: "Treasury",
	})
}

// localJane creates the local user jdoe and returns a client logged in as them.
func localJane(t *testing.T, env *apitest.Env) (*apitest.Client, userBody) {
	t.Helper()
	u := createUser(t, env.AdminClient(), map[string]any{"username": "jdoe", "password": "jane-pw-123"})
	c := env.Client()
	c.LoginLocal("jdoe", "jane-pw-123").ExpectStatus(http.StatusOK)
	return c, u
}

func bindOwn(c *apitest.Client, fields map[string]any) *apitest.Response {
	return c.Post("/api/me/identities", fields)
}

func bindLDAP(login, password string) map[string]any {
	return map[string]any{"kind": "ldap", "loginName": login, "password": password}
}

func TestUserBindsAnLDAPIdentityAfterAuthenticatingWithItAndThenLogsInWithEither(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addJane(env)
	jane, created := localJane(t, env)

	var me meBody
	bindOwn(jane, bindLDAP("jane.doe", "jane-ldap-pw")).ExpectStatus(http.StatusCreated).JSON(&me)
	if len(me.Identities) != 2 || me.Identities[1].Kind != "ldap" || me.Identities[1].LoginName != "jane.doe" ||
		me.Identities[1].BoundVia != "self" || me.Identities[1].BoundAt == "" {
		t.Fatalf("identities after binding = %+v, want local (admin) then ldap (self)", me.Identities)
	}
	if got := getMe(t, jane); len(got.Identities) != 2 {
		t.Fatalf("GET /api/me identities = %+v", got.Identities)
	}

	// The LDAP login now enters the same user, not a new one.
	ldap := env.Client()
	if u := userOf(t, loginLDAP(ldap, "jane.doe", "jane-ldap-pw")); u.ID != created.ID || u.Username != "jdoe" {
		t.Fatalf("LDAP login opened %+v, want user %s (jdoe)", u, created.ID)
	}
	env.Client().LoginLocal("jdoe", "jane-pw-123").ExpectStatus(http.StatusOK)
}

func TestBindingAnLDAPIdentityMakesTheDirectoryOwnDisplayNameEmailAndDepartment(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addJane(env)
	jane, _ := localJane(t, env)
	bindOwn(jane, bindLDAP("jane.doe", "jane-ldap-pw")).ExpectStatus(http.StatusCreated)

	me := getMe(t, jane)
	if me.DisplayName != "Jane Doe (Corp)" || me.Email != "jane.doe@corp.example" || me.Department != "Treasury" {
		t.Fatalf("profile after binding = %+v, want the directory's values", me)
	}
	patchMe(jane, map[string]any{"displayName": "Mine"}).ExpectError(http.StatusUnprocessableEntity, "displayName", "ldapManaged")
}

func TestBindingAnLDAPIdentityIsRefusedWhenAuthenticationFails(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addJane(env)
	jane, _ := localJane(t, env)

	// 422, not 401: a 401 would log the browser out.
	bindOwn(jane, bindLDAP("jane.doe", "wrong")).ExpectError(http.StatusUnprocessableEntity, "password", "invalidCredentials")
	bindOwn(jane, bindLDAP("nobody", "x")).ExpectError(http.StatusUnprocessableEntity, "password", "invalidCredentials")
	env.Directory.SetDisabled("jane.doe", true)
	bindOwn(jane, bindLDAP("jane.doe", "jane-ldap-pw")).ExpectError(http.StatusUnprocessableEntity, "password", "invalidCredentials")

	env.Directory.SetDisabled("jane.doe", false)
	env.Directory.SetUnreachable(true)
	bindOwn(jane, bindLDAP("jane.doe", "jane-ldap-pw")).ExpectError(http.StatusServiceUnavailable, "", "ldapUnreachable")

	bindOwn(jane, map[string]any{"kind": "ldap", "loginName": "", "password": ""}).ExpectStatus(http.StatusUnprocessableEntity)
	if me := getMe(t, jane); len(me.Identities) != 1 {
		t.Fatalf("identities = %+v, nothing may be bound by a failed attempt", me.Identities)
	}
}

func TestAUserHasAtMostOneIdentityOfEachType(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addJane(env)
	env.Directory.AddAccount("jane.second", "pw2", domain.DirectoryEntry{DisplayName: "Jane Two"})
	jane, _ := localJane(t, env)
	bindOwn(jane, bindLDAP("jane.doe", "jane-ldap-pw")).ExpectStatus(http.StatusCreated)

	bindOwn(jane, bindLDAP("jane.second", "pw2")).ExpectError(http.StatusUnprocessableEntity, "kind", "identityKindBound")
	if me := getMe(t, jane); len(me.Identities) != 2 {
		t.Fatalf("identities = %+v, want the original two", me.Identities)
	}
}

func TestAnIdentityHeldByAnotherUserCannotBeBound(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	emma := env.Client()
	emmaUser := userOf(t, loginLDAP(emma, "elarsen", "ldap-pw"))
	jane, _ := localJane(t, env)

	bindOwn(jane, bindLDAP("elarsen", "ldap-pw")).ExpectError(http.StatusUnprocessableEntity, "loginName", "identityTaken")
	if me := getMe(t, jane); len(me.Identities) != 1 {
		t.Fatalf("identities = %+v", me.Identities)
	}
	if u := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw")); u.ID != emmaUser.ID {
		t.Fatalf("LDAP login opened user %s, want the original holder %s", u.ID, emmaUser.ID)
	}
}

func TestSettingsShowEachLoginIdentityWithItsTypeLoginNameBindingTimeAndSource(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)

	admin := getMe(t, env.AdminClient())
	if admin.Username != apitest.AdminUsername || !admin.IsAdmin || admin.DisplayName != "Initial Admin" {
		t.Fatalf("me = %+v", admin)
	}
	if len(admin.Identities) != 1 || admin.Identities[0].Kind != "local" || admin.Identities[0].LoginName != apitest.AdminUsername ||
		admin.Identities[0].BoundVia != "bootstrap" || admin.Identities[0].BoundAt == "" {
		t.Fatalf("admin identities = %+v, want the bootstrap local identity", admin.Identities)
	}

	createUser(t, env.AdminClient(), map[string]any{"username": "jdoe", "phone": "555-0100", "password": "jane-pw-123"})
	jane := env.Client()
	jane.LoginLocal("jdoe", "jane-pw-123").ExpectStatus(http.StatusOK)
	me := getMe(t, jane)
	if me.Username != "jdoe" || me.Phone != "555-0100" || me.IsAdmin || len(me.Identities) != 1 || me.Identities[0].BoundVia != "admin" {
		t.Fatalf("jdoe me = %+v, want a local identity bound by admin", me)
	}

	addEmma(env)
	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	if got := getMe(t, emma); len(got.Identities) != 1 || got.Identities[0].Kind != "ldap" ||
		got.Identities[0].LoginName != "elarsen" || got.Identities[0].BoundVia != "firstLogin" {
		t.Fatalf("emma identities = %+v, want the ldap identity bound at first login", got.Identities)
	}
}
