package api

import (
	"net/http"
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
