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
