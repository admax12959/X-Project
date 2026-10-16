package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

const (
	glURL = "https://gitlab.corp.example"
	ghURL = "https://github.corp.example"
)

// registerPlatform registers a code platform through the API and returns its ID.
func registerPlatform(t *testing.T, admin *apitest.Client, kind, baseURL string) string {
	t.Helper()
	var p platformBody
	admin.Post("/api/code-platforms", map[string]string{"kind": kind, "baseUrl": baseURL, "token": "tok-abcdefghij1234567890"}).
		ExpectStatus(http.StatusCreated).JSON(&p)
	return p.ID
}

// plainUserClient creates an enabled, non-administrator user through the API (display name
// = username, password "initial-pw-1") and returns a client logged in as that user.
func plainUserClient(t *testing.T, env *apitest.Env, username string) *apitest.Client {
	t.Helper()
	createUser(t, env.AdminClient(), map[string]any{"username": username, "displayName": username})
	c := env.Client()
	c.LoginAs(username, "initial-pw-1").ExpectStatus(http.StatusOK)
	return c
}
