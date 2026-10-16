package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// directoryDeptBody reads the directory department wherever a user is shown.
type directoryDeptBody struct {
	DirectoryDepartment string `json:"directoryDepartment"`
	Department          string `json:"department"`
}

// The department text synced from LDAP is the "directory department": read-only text under
// that name in every user view, and unrelated to the organization's departments (#25).
func TestDirectoryDepartmentIsReadOnlyLDAPTextUnderItsOwnName(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	admin := env.AdminClient()
	emma := env.Client()
	var session struct {
		User directoryDeptBody `json:"user"`
	}
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK).JSON(&session)
	if session.User.DirectoryDepartment != "Payments Platform" || session.User.Department != "" {
		t.Fatalf("session user = %+v", session.User)
	}
	var me directoryDeptBody
	emma.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&me)
	if me.DirectoryDepartment != "Payments Platform" || me.Department != "" {
		t.Fatalf("me = %+v", me)
	}
	var list struct {
		Items []directoryDeptBody `json:"items"`
	}
	admin.Get("/api/users").ExpectStatus(http.StatusOK).JSON(&list)
	found := false
	for _, u := range list.Items {
		found = found || u.DirectoryDepartment == "Payments Platform"
		if u.Department != "" {
			t.Fatalf("user list still has a department key: %+v", u)
		}
	}
	if !found {
		t.Fatalf("user list = %+v", list.Items)
	}

	// Nobody can write it: not at creation, not by an administrator's edit, not by the user.
	var created directoryDeptBody
	admin.Post("/api/users", newUser(map[string]any{"username": "kim", "department": "Payments", "directoryDepartment": "Payments"})).
		ExpectStatus(http.StatusCreated).JSON(&created)
	if created.DirectoryDepartment != "" {
		t.Fatalf("created = %+v", created)
	}
	var edited directoryDeptBody
	jane := createUser(t, admin, map[string]any{"username": "jdoe"})
	patchUser(admin, jane.ID, map[string]any{"department": "Ops", "directoryDepartment": "Ops"}).ExpectStatus(http.StatusOK).JSON(&edited)
	if edited.DirectoryDepartment != "" {
		t.Fatalf("edited = %+v", edited)
	}
	janeC := env.Client()
	janeC.LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	var self directoryDeptBody
	patchMe(janeC, map[string]any{"department": "Ops", "directoryDepartment": "Ops"}).ExpectStatus(http.StatusOK).JSON(&self)
	if self.DirectoryDepartment != "" {
		t.Fatalf("self = %+v", self)
	}
	// An LDAP user sending the old key is not an error any more: the key is simply gone.
	patchMe(emma, map[string]any{"department": "Something Else"}).ExpectStatus(http.StatusOK)
	emma.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&me)
	if me.DirectoryDepartment != "Payments Platform" {
		t.Fatalf("me after = %+v", me)
	}
}
