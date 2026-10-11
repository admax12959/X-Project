package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
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
