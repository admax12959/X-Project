package api

import (
	"net/http"
	"slices"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

type roleBody struct {
	ID          string   `json:"id"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

type roleList struct {
	Items []roleBody `json:"items"`
}

func listRoles(t *testing.T, c *apitest.Client) []roleBody {
	t.Helper()
	var l roleList
	c.Get("/api/roles").ExpectStatus(http.StatusOK).JSON(&l)
	return l.Items
}

func roleByID(t *testing.T, roles []roleBody, id string) roleBody {
	t.Helper()
	for _, r := range roles {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("role %q not listed: %+v", id, roles)
	return roleBody{}
}

func TestAdminSeesTheThreeBuiltInRolesWithTheirScope(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)

	roles := listRoles(t, env.AdminClient())

	var ids []string
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	if !slices.Equal(ids, []string{"administrator", "owner", "pe"}) {
		t.Fatalf("role ids = %v, want [administrator owner pe]", ids)
	}
	if got := roleByID(t, roles, "administrator").Scope; got != "global" {
		t.Errorf("administrator scope = %q, want global", got)
	}
	for _, id := range []string{"owner", "pe"} {
		if got := roleByID(t, roles, id).Scope; got != "application" {
			t.Errorf("%s scope = %q, want application", id, got)
		}
	}
}

// The permission matrix of spec #2 (unchanged by the refactor): who may do what.
func TestRolesListTheirPermissionPoints(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)

	roles := listRoles(t, env.AdminClient())

	want := map[string][]string{
		"administrator": {"app.create", "app.delete", "app.members.manage", "audit.view", "platform.manage", "repo.verify", "role.view", "user.manage"},
		"owner":         {"app.deploy.edit", "app.edit", "app.members.manage", "line.edit", "line.fork", "line.status.set", "version.register"},
		"pe":            {"app.deploy.edit"},
	}
	for id, perms := range want {
		got := slices.Clone(roleByID(t, roles, id).Permissions)
		slices.Sort(got)
		if !slices.Equal(got, perms) {
			t.Errorf("%s permissions = %v, want %v", id, got, perms)
		}
	}
}

func TestOnlyAdministratorsMayReadTheRoles(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)

	plainUserClient(t, env, "jdoe").Get("/api/roles").ExpectStatus(http.StatusForbidden)
	env.Client().Get("/api/roles").ExpectStatus(http.StatusUnauthorized)
}

func TestRolesCannotBeChanged(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, path := range []string{"/api/roles", "/api/roles/owner"} {
			res := admin.Do(method, path, map[string]any{"id": "custom", "permissions": []string{"app.edit"}})
			if res.Status < 400 || res.Status >= 500 {
				t.Errorf("%s %s = %d, want a 4xx", method, path, res.Status)
			}
		}
	}
	if got := len(listRoles(t, admin)); got != 3 {
		t.Fatalf("%d roles after attempts, want 3", got)
	}
}
