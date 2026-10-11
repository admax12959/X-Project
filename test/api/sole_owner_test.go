package api

import (
	"net/http"
	"reflect"
	"testing"
)

// soleOwnerApps reads the blocking application keys out of a soleOwner error.
func soleOwnerApps(t *testing.T, params map[string]any) []string {
	t.Helper()
	list, _ := params["applications"].([]any)
	keys := []string{}
	for _, item := range list {
		m := item.(map[string]any)
		if m["name"] == "" || m["name"] == nil {
			t.Fatalf("blocking application without a name: %v", m)
		}
		keys = append(keys, m["key"].(string))
	}
	return keys
}

func TestDisablingTheOnlyOwnerOfAnApplicationIsBlockedAndNamesTheApplications(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	f.env.CodePlatforms.AddRepo(glURL, "platform/order-service", "552")
	f.env.CodePlatforms.AddRepo(glURL, "platform/shared", "553")
	mk := func(key, path string, owners []string) {
		t.Helper()
		f.admin.Post("/api/applications", newApp(f.pid, owners, map[string]any{"key": key, "repo": map[string]any{"platformId": f.pid, "path": path}})).
			ExpectStatus(http.StatusCreated)
	}
	mk("order-service", "platform/order-service", []string{f.owner.ID})
	mk("shared-app", "platform/shared", []string{f.owner.ID, f.bob.ID}) // two Owners: not blocking

	e := f.admin.Post("/api/users/"+f.owner.ID+"/disable", nil).ExpectError(http.StatusUnprocessableEntity, "", "soleOwner")
	if got := soleOwnerApps(t, e.Params); !reflect.DeepEqual(got, []string{"order-service", "payments-core"}) {
		t.Fatalf("blocking applications = %v", got)
	}
	f.ownerC.Get("/api/session").ExpectStatus(http.StatusOK) // still enabled, still logged in
	if got := findUser(t, listUsers(t, f.admin), "elarsen").Status; got != "enabled" {
		t.Fatalf("status = %q", got)
	}

	// Hand both applications over (administrator, ADR-0003): then the disable goes through.
	f.putMembers(f.admin, []string{f.bob.ID}, []string{f.pe.ID}).ExpectStatus(http.StatusOK)
	e = f.admin.Post("/api/users/"+f.owner.ID+"/disable", nil).ExpectError(http.StatusUnprocessableEntity, "", "soleOwner")
	if got := soleOwnerApps(t, e.Params); !reflect.DeepEqual(got, []string{"order-service"}) {
		t.Fatalf("blocking applications = %v", got)
	}
	f.admin.Do(http.MethodPut, "/api/applications/order-service/members", map[string]any{"owners": []string{f.bob.ID}, "pes": []string{}}).ExpectStatus(http.StatusOK)
	f.admin.Post("/api/users/"+f.owner.ID+"/disable", nil).ExpectStatus(http.StatusOK)

	// Disabling again is harmless, and a PE-only user is never blocked.
	f.admin.Post("/api/users/"+f.owner.ID+"/disable", nil).ExpectStatus(http.StatusOK)
	f.admin.Post("/api/users/"+f.pe.ID+"/disable", nil).ExpectStatus(http.StatusOK)
}

func TestAnOwnerIsStillTheOnlyOwnerWhenTheOtherOwnersAreDisabled(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	f.putMembers(f.admin, []string{f.owner.ID, f.bob.ID}, []string{}).ExpectStatus(http.StatusOK)

	f.admin.Post("/api/users/"+f.bob.ID+"/disable", nil).ExpectStatus(http.StatusOK) // elarsen remains
	e := f.admin.Post("/api/users/"+f.owner.ID+"/disable", nil).ExpectError(http.StatusUnprocessableEntity, "", "soleOwner")
	if got := soleOwnerApps(t, e.Params); !reflect.DeepEqual(got, []string{"payments-core"}) {
		t.Fatalf("blocking applications = %v", got)
	}

	// Enabling the other Owner again unblocks it.
	f.admin.Post("/api/users/"+f.bob.ID+"/enable", nil).ExpectStatus(http.StatusOK)
	f.admin.Post("/api/users/"+f.owner.ID+"/disable", nil).ExpectStatus(http.StatusOK)
}
