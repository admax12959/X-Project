package api

import (
	"net/http"
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
