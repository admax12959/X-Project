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

func TestFirstLDAPLoginCreatesTheUserFromDirectoryAttributes(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	c := env.Client()

	var s ldapSession
	loginLDAP(c, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK).JSON(&s)
	want := ldapUser{Username: "elarsen", DisplayName: "Emma Larsen", Email: "emma.larsen@corp.example", Department: "Payments Platform"}
	got := s.User
	got.ID = ""
	if got != want || s.User.ID == "" {
		t.Fatalf("user = %+v, want %+v", s.User, want)
	}

	var me ldapSession
	c.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&me)
	if me.User != s.User {
		t.Fatalf("GET /api/session = %+v, want %+v", me.User, s.User)
	}
}

var _ = app.DefaultConfig
