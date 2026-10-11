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
}
