package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// userBody mirrors the administrator view of a user (GET /api/users items).
type userBody struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Department  string `json:"department"`
	Status      string `json:"status"`
	IsAdmin     bool   `json:"isAdmin"`
	Identities  []struct {
		Kind     string `json:"kind"`
		BoundVia string `json:"boundVia"`
	} `json:"identities"`
}

type userListBody struct {
	Items []userBody `json:"items"`
}

// newUser is a valid create-user request; tests override single fields.
func newUser(over map[string]any) map[string]any {
	m := map[string]any{
		"username":    "jdoe",
		"displayName": "Jane Doe",
		"email":       "jane@example.com",
		"department":  "Payments",
		"password":    "initial-pw-1",
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

func createUser(t *testing.T, c *apitest.Client, over map[string]any) userBody {
	t.Helper()
	var u userBody
	c.Post("/api/users", newUser(over)).ExpectStatus(http.StatusCreated).JSON(&u)
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

func TestAdminCreatesALocalUserWhoCanThenLogIn(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	created := createUser(t, admin, nil)
	if created.ID == "" || created.Username != "jdoe" || created.DisplayName != "Jane Doe" ||
		created.Email != "jane@example.com" || created.Department != "Payments" ||
		created.Status != "enabled" || created.IsAdmin {
		t.Fatalf("created = %+v", created)
	}

	env.Client().LoginLocal("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	env.Client().LoginLocal("jdoe", "wrong-password").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
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
	if jane.DisplayName != "Jane Doe" || jane.Email != "jane@example.com" || jane.Department != "Payments" || jane.Status != "enabled" {
		t.Fatalf("jane = %+v", jane)
	}
	if len(jane.Identities) != 1 || jane.Identities[0].Kind != "local" {
		t.Fatalf("identities = %+v", jane.Identities)
	}
	root := findUser(t, users, apitest.AdminUsername)
	if !root.IsAdmin || len(root.Identities) != 1 || root.Identities[0].Kind != "local" {
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

func TestOnlyAdministratorsMayManageUsers(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	createUser(t, admin, nil)

	jane := env.Client()
	jane.LoginLocal("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	jane.Get("/api/users").ExpectError(http.StatusForbidden, "", "forbidden")
	jane.Post("/api/users", newUser(map[string]any{"username": "other"})).ExpectError(http.StatusForbidden, "", "forbidden")

	if got := len(listUsers(t, admin)); got != 2 {
		t.Fatalf("a forbidden create changed the user list: %d users", got)
	}
}
