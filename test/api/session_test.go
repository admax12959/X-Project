package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

type sessionBody struct {
	User struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		IsAdmin     bool   `json:"isAdmin"`
	} `json:"user"`
}

func TestInitialAdministratorCanLogInWithConfiguredPassword(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	c := env.Client()

	var login sessionBody
	c.LoginAs(apitest.AdminUsername, apitest.AdminPassword).ExpectStatus(http.StatusOK).JSON(&login)
	if login.User.Username != apitest.AdminUsername || !login.User.IsAdmin || login.User.DisplayName != "Initial Admin" {
		t.Fatalf("login user = %+v", login.User)
	}

	var me sessionBody
	c.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&me)
	if me.User != login.User {
		t.Fatalf("GET /api/session user = %+v, want %+v", me.User, login.User)
	}
}
