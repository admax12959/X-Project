package api

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// maintFixture is a payments-core application with an Owner (elarsen), a PE
// (praman), a plain user (bob) and the administrator.
type maintFixture struct {
	appFixture
	pe      userBody
	bob     userBody
	ownerC  *apitest.Client
	peC     *apitest.Client
	bobC    *apitest.Client
	appPath string
}

func newMaintFixture(t *testing.T) maintFixture {
	t.Helper()
	f := newAppFixture(t)
	pe := createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	bob := createUser(t, f.admin, map[string]any{"username": "bob", "displayName": "Bob Plain"})
	f.admin.Post("/api/applications", newApp(f.pid, []string{f.owner.ID}, map[string]any{"pes": []string{pe.ID}})).
		ExpectStatus(http.StatusCreated)
	login := func(username string) *apitest.Client {
		c := f.env.Client()
		c.LoginLocal(username, "initial-pw-1").ExpectStatus(http.StatusOK)
		return c
	}
	return maintFixture{
		appFixture: f, pe: pe, bob: bob, appPath: "/api/applications/payments-core",
		ownerC: login("elarsen"), peC: login("praman"), bobC: login("bob"),
	}
}

func (f maintFixture) app(t *testing.T) appBody {
	t.Helper()
	var a appBody
	f.admin.Get(f.appPath).ExpectStatus(http.StatusOK).JSON(&a)
	return a
}

func (f maintFixture) history(t *testing.T) []historyEventBody {
	t.Helper()
	var h historyBody
	f.admin.Get(f.appPath + "/history").ExpectStatus(http.StatusOK).JSON(&h)
	return h.Items
}

// memberEvents summarises the member events of the history in chronological order,
// e.g. "memberRemoved owner elarsen by admin".
func (f maintFixture) memberEvents(t *testing.T) []string {
	t.Helper()
	var raw struct {
		Items []struct {
			Type  string `json:"type"`
			Actor struct {
				Username string `json:"username"`
			} `json:"actor"`
			Params map[string]any `json:"params"`
		} `json:"items"`
	}
	f.admin.Get(f.appPath + "/history").ExpectStatus(http.StatusOK).JSON(&raw)
	var out []string
	for i := len(raw.Items) - 1; i >= 0; i-- {
		e := raw.Items[i]
		if e.Type != "memberAdded" && e.Type != "memberRemoved" {
			continue
		}
		user, _ := e.Params["user"].(map[string]any)
		out = append(out, fmt.Sprintf("%s %v %v by %s", e.Type, e.Params["role"], user["username"], e.Actor.Username))
	}
	return out
}

func (f maintFixture) putMembers(c *apitest.Client, owners, pes []string) *apitest.Response {
	return c.Do(http.MethodPut, f.appPath+"/members", map[string]any{"owners": owners, "pes": pes})
}

func TestAdministratorTransfersOwnershipAndEachChangeIsRecordedWithTheRealActor(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)

	// ADR-0003: the administrator is not an Owner and still may replace the Owner.
	var updated appBody
	f.putMembers(f.admin, []string{f.bob.ID}, []string{f.pe.ID}).ExpectStatus(http.StatusOK).JSON(&updated)
	if len(updated.Owners) != 1 || updated.Owners[0].Username != "bob" || len(updated.PEs) != 1 {
		t.Fatalf("updated = %+v", updated)
	}
	got := f.memberEvents(t)
	want := []string{"memberRemoved owner elarsen by admin", "memberAdded owner bob by admin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %q, want %q", got, want)
	}

	// The former Owner lost the Owner rights, the new one has them.
	f.ownerC.Do(http.MethodPatch, f.appPath, map[string]any{"name": "Nope"}).ExpectError(http.StatusForbidden, "", "forbidden")
	f.bobC.Do(http.MethodPatch, f.appPath, map[string]any{"name": "Mine now"}).ExpectStatus(http.StatusOK)
}

func TestOwnerAddsAndRemovesPEsAndOwnersOneEventPerPerson(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)

	f.putMembers(f.ownerC, []string{f.owner.ID, f.bob.ID}, []string{}).ExpectStatus(http.StatusOK)
	want := []string{"memberRemoved pe praman by elarsen", "memberAdded owner bob by elarsen"}
	if got := f.memberEvents(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %q, want %q", got, want)
	}

	// Same people in another order, duplicates ignored: the order is kept, nothing is recorded.
	var a appBody
	f.putMembers(f.ownerC, []string{f.bob.ID, f.owner.ID, f.bob.ID}, []string{}).ExpectStatus(http.StatusOK).JSON(&a)
	if len(a.Owners) != 2 || a.Owners[0].Username != "bob" {
		t.Fatalf("owners = %+v", a.Owners)
	}
	if got := f.memberEvents(t); len(got) != 2 {
		t.Fatalf("events = %q", got)
	}
}

func TestOwnerListCanNeverBeEmptiedAndMembersMustBeEnabledUsers(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	for _, c := range []*apitest.Client{f.admin, f.ownerC} {
		f.putMembers(c, []string{}, []string{f.pe.ID}).ExpectError(http.StatusUnprocessableEntity, "owners", "required")
	}
	f.admin.Do(http.MethodPut, f.appPath+"/members", map[string]any{"owners": []string{f.owner.ID}}).ExpectError(http.StatusUnprocessableEntity, "pes", "required")
	f.putMembers(f.admin, []string{"abc"}, []string{}).ExpectError(http.StatusUnprocessableEntity, "owners", "invalid")

	// A disabled or unknown user cannot be added.
	f.admin.Post("/api/users/"+f.bob.ID+"/disable", nil).ExpectStatus(http.StatusOK)
	e := f.putMembers(f.ownerC, []string{f.owner.ID, f.bob.ID}, []string{}).ExpectError(http.StatusUnprocessableEntity, "owners", "userUnavailable")
	if e.Params["userId"] != f.bob.ID {
		t.Fatalf("params = %v", e.Params)
	}
	f.putMembers(f.ownerC, []string{f.owner.ID}, []string{"99999"}).ExpectError(http.StatusUnprocessableEntity, "pes", "userUnavailable")

	got := f.app(t)
	if len(got.Owners) != 1 || got.Owners[0].Username != "elarsen" || len(got.PEs) != 1 {
		t.Fatalf("rejected edits changed the lists: %+v", got)
	}
	if h := f.memberEvents(t); len(h) != 0 {
		t.Fatalf("events = %q", h)
	}
}

func TestAMemberWhoWasDisabledLaterMayStayInTheListButCannotBeAddedAgain(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	f.admin.Post("/api/users/"+f.pe.ID+"/disable", nil).ExpectStatus(http.StatusOK)

	// Only newly added people have to be enabled; the disabled PE can stay while an Owner is added...
	f.putMembers(f.ownerC, []string{f.owner.ID, f.bob.ID}, []string{f.pe.ID}).ExpectStatus(http.StatusOK)
	// ...and be removed, after which they cannot come back.
	f.putMembers(f.ownerC, []string{f.owner.ID, f.bob.ID}, []string{}).ExpectStatus(http.StatusOK)
	f.putMembers(f.ownerC, []string{f.owner.ID, f.bob.ID}, []string{f.pe.ID}).ExpectError(http.StatusUnprocessableEntity, "pes", "userUnavailable")
}

func TestOwnerEditsNameAndDescriptionAndItIsRecordedInTheHistory(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)

	var updated appBody
	f.ownerC.Do(http.MethodPatch, f.appPath, map[string]any{"name": "Payments Core v2", "description": "New text"}).
		ExpectStatus(http.StatusOK).JSON(&updated)
	if updated.Name != "Payments Core v2" || updated.Description != "New text" || updated.Key != "payments-core" {
		t.Fatalf("updated = %+v", updated)
	}
	if got := f.app(t); got.Name != "Payments Core v2" || got.Description != "New text" {
		t.Fatalf("stored = %+v", got)
	}

	h := f.history(t)
	if len(h) != 2 || h[0].Type != "appInfoChanged" || h[0].Actor.Username != "elarsen" {
		t.Fatalf("history = %+v", h)
	}
}

func TestSavingUnchangedValuesWritesNoHistoryAndKeyAndRepoCannotBeEdited(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	before := f.app(t)

	f.ownerC.Do(http.MethodPatch, f.appPath, map[string]any{"name": before.Name, "description": before.Description}).ExpectStatus(http.StatusOK)
	if n := len(f.history(t)); n != 1 {
		t.Fatalf("history has %d events after a no-op save, want 1", n)
	}

	// Key and repository binding have no edit path: they are silently not part of the request.
	f.ownerC.Do(http.MethodPatch, f.appPath, map[string]any{
		"key": "renamed", "repo": map[string]any{"platformId": f.pid, "path": "elsewhere/else"}, "name": "Edited",
	}).ExpectStatus(http.StatusOK)
	after := f.app(t)
	if after.Key != "payments-core" || after.Repo != before.Repo || after.Name != "Edited" {
		t.Fatalf("after = %+v", after)
	}
	f.admin.Get("/api/applications/renamed").ExpectError(http.StatusNotFound, "", "notFound")
}

func TestOwnerChangesTheMainBranchOnTheMainline(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)

	f.ownerC.Do(http.MethodPatch, f.appPath, map[string]any{"mainBranch": "develop"}).ExpectStatus(http.StatusOK)
	if got := f.app(t).MainBranch; got != "develop" {
		t.Fatalf("mainBranch = %q", got)
	}
	var lines struct {
		Items []struct {
			IsMain bool   `json:"isMain"`
			Branch string `json:"branch"`
		} `json:"items"`
	}
	f.admin.Get(f.appPath + "/lines").ExpectStatus(http.StatusOK).JSON(&lines)
	if len(lines.Items) != 1 || !lines.Items[0].IsMain || lines.Items[0].Branch != "develop" {
		t.Fatalf("lines = %+v", lines.Items)
	}
	if h := f.history(t); len(h) != 2 || h[0].Type != "appInfoChanged" {
		t.Fatalf("history = %+v", h)
	}

	f.ownerC.Do(http.MethodPatch, f.appPath, map[string]any{"mainBranch": "has space"}).ExpectError(http.StatusUnprocessableEntity, "mainBranch", "invalid")
	f.ownerC.Do(http.MethodPatch, f.appPath, map[string]any{"mainBranch": ""}).ExpectError(http.StatusUnprocessableEntity, "mainBranch", "required")
	if got := f.app(t).MainBranch; got != "develop" {
		t.Fatalf("a rejected edit changed mainBranch to %q", got)
	}
}

// The permission matrix of spec #2, one row per edit action, one column per kind of user.
func TestPermissionMatrixForMaintainingAnApplication(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	owners, pes := []string{f.owner.ID}, []string{f.pe.ID}
	actions := []struct {
		name   string
		method string
		path   string
		body   map[string]any
		// expected status for admin (not a member), Owner, PE, other user
		admin, owner, pe, other int
	}{
		{"basic info", http.MethodPatch, f.appPath, map[string]any{"name": "Matrix"}, 403, 200, 403, 403},
		{"Owner/PE lists", http.MethodPut, f.appPath + "/members", map[string]any{"owners": owners, "pes": pes}, 200, 200, 403, 403},
		{"deployment info", http.MethodPut, f.appPath + "/deploy-info", map[string]any{"deployInfo": "prod-eu-2"}, 403, 200, 200, 403},
	}
	for _, a := range actions {
		for _, who := range []struct {
			name   string
			client *apitest.Client
			want   int
		}{{"administrator", f.admin, a.admin}, {"Owner", f.ownerC, a.owner}, {"PE", f.peC, a.pe}, {"other user", f.bobC, a.other}} {
			res := who.client.Do(a.method, a.path, a.body)
			if res.Status != who.want {
				t.Errorf("%s as %s: status %d, want %d; body %s", a.name, who.name, res.Status, who.want, res.Body)
			}
			if who.want == http.StatusForbidden {
				res.ExpectError(http.StatusForbidden, "", "forbidden")
			}
		}
		if res := f.env.Client().Do(a.method, a.path, a.body); res.Status != http.StatusUnauthorized {
			t.Errorf("%s logged out: status %d", a.name, res.Status)
		}
	}
}

func TestAnAdministratorWhoIsAlsoOwnerMayEditBasicInfoAndDeploymentInfo(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	var me struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	f.admin.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&me)
	f.admin.Do(http.MethodPut, f.appPath+"/members", map[string]any{"owners": []string{f.owner.ID, me.User.ID}, "pes": []string{}}).ExpectStatus(http.StatusOK)

	f.admin.Do(http.MethodPatch, f.appPath, map[string]any{"name": "By admin-owner"}).ExpectStatus(http.StatusOK)
	f.admin.Do(http.MethodPut, f.appPath+"/deploy-info", map[string]any{"deployInfo": "x"}).ExpectStatus(http.StatusOK)
}

func TestOwnerAndPeEditDeploymentInfoAndItIsRecordedInTheHistory(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)

	var updated appBody
	f.peC.Do(http.MethodPut, f.appPath+"/deploy-info", map[string]any{"deployInfo": "prod-eu-2\nwindow: Tue 02:00"}).ExpectStatus(http.StatusOK).JSON(&updated)
	if updated.DeployInfo != "prod-eu-2\nwindow: Tue 02:00" || f.app(t).DeployInfo != updated.DeployInfo {
		t.Fatalf("updated = %+v", updated)
	}
	h := f.history(t)
	if len(h) != 2 || h[0].Type != "deployInfoChanged" || h[0].Actor.Username != "praman" {
		t.Fatalf("history = %+v", h)
	}

	// Saving the same text again records nothing; clearing the text is allowed.
	f.ownerC.Do(http.MethodPut, f.appPath+"/deploy-info", map[string]any{"deployInfo": updated.DeployInfo}).ExpectStatus(http.StatusOK)
	if n := len(f.history(t)); n != 2 {
		t.Fatalf("history has %d events", n)
	}
	f.ownerC.Do(http.MethodPut, f.appPath+"/deploy-info", map[string]any{"deployInfo": ""}).ExpectStatus(http.StatusOK)
	if got := f.app(t).DeployInfo; got != "" {
		t.Fatalf("deployInfo = %q", got)
	}
	if h := f.history(t); len(h) != 3 || h[0].Type != "deployInfoChanged" || h[0].Actor.Username != "elarsen" {
		t.Fatalf("history = %+v", h)
	}

	f.ownerC.Do(http.MethodPut, f.appPath+"/deploy-info", map[string]any{"deployInfo": strings.Repeat("x", 20001)}).ExpectError(http.StatusUnprocessableEntity, "deployInfo", "tooLong")
	f.ownerC.Do(http.MethodPut, f.appPath+"/deploy-info", map[string]any{}).ExpectError(http.StatusUnprocessableEntity, "deployInfo", "required")
}

func TestApplicationInfoIsValidated(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	patch := func(body map[string]any) *apitest.Response { return f.ownerC.Do(http.MethodPatch, f.appPath, body) }

	patch(map[string]any{"name": "  "}).ExpectError(http.StatusUnprocessableEntity, "name", "required")
	patch(map[string]any{"name": strings.Repeat("n", 201)}).ExpectError(http.StatusUnprocessableEntity, "name", "tooLong")
	patch(map[string]any{"description": strings.Repeat("d", 4001)}).ExpectError(http.StatusUnprocessableEntity, "description", "tooLong")
	// All problems are reported at once and nothing is applied.
	res := patch(map[string]any{"name": "", "description": strings.Repeat("d", 4001), "mainBranch": "a..b"}).ExpectStatus(http.StatusUnprocessableEntity)
	if len(res.Errors()) != 3 {
		t.Fatalf("errors = %s", res.Body)
	}
	if got := f.app(t); got.Name != "Payments Core" || got.Description != "Card and ledger services" {
		t.Fatalf("app = %+v", got)
	}
}
