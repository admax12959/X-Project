package api

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// secondApp creates "rate-engine", owned by owner, and returns its mainline.
func (f versionFixture) secondApp(t *testing.T, ownerID string, over map[string]any) lineBody {
	t.Helper()
	f.env.CodePlatforms.AddRepo(glURL, "misc/rates", "7")
	m := map[string]any{"key": "rate-engine", "repo": map[string]any{"platformId": f.pid, "path": "misc/rates"}}
	for k, v := range over {
		m[k] = v
	}
	f.admin.Post("/api/applications", newApp(f.pid, []string{ownerID}, m)).ExpectStatus(http.StatusCreated)
	var lines struct {
		Items []lineBody `json:"items"`
	}
	f.admin.Get("/api/applications/rate-engine/lines").ExpectStatus(http.StatusOK).JSON(&lines)
	return lines.Items[0]
}

func (f versionFixture) login(username string) *apitest.Client {
	c := f.env.Client()
	c.LoginLocal(username, "initial-pw-1").ExpectStatus(http.StatusOK)
	return c
}

func TestRegisteringAVersionNeedsNumberRefAndReleaseDate(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	invalid := http.StatusUnprocessableEntity

	f.register(map[string]any{"number": "  "}).ExpectError(invalid, "number", "required")
	f.register(map[string]any{"ref": map[string]any{"kind": "", "value": "v1"}}).ExpectError(invalid, "ref.kind", "required")
	f.register(map[string]any{"ref": map[string]any{"kind": "branch", "value": "v1"}}).ExpectError(invalid, "ref.kind", "invalid")
	f.register(map[string]any{"ref": map[string]any{"kind": "tag", "value": " "}}).ExpectError(invalid, "ref.value", "required")
	f.register(map[string]any{"releasedAt": ""}).ExpectError(invalid, "releasedAt", "required")
	for _, bad := range []string{"2026-02-30", "12/03/2024", "2024-3-12", "yesterday"} {
		f.register(map[string]any{"releasedAt": bad}).ExpectError(invalid, "releasedAt", "invalid")
	}
	// All problems are reported together.
	res := f.elarsen.Post(versionsPath("payments-core", f.mainline.ID), map[string]any{})
	res.ExpectStatus(invalid)
	if got := len(res.Errors()); got != 4 {
		t.Fatalf("empty body: %d errors, want 4: %s", got, res.Body)
	}
	if got := f.list(f.elarsen); len(got) != 0 {
		t.Fatalf("rejected requests stored versions: %+v", got)
	}

	// The note is optional, either kind of reference works, and the number format is free.
	var v versionBody
	f.register(map[string]any{"number": "emea-1.1", "note": "", "ref": map[string]any{"kind": "commit", "value": "b71d0e8"}}).
		ExpectStatus(http.StatusCreated).JSON(&v)
	if v.Ref.Kind != "commit" || v.Ref.Value != "b71d0e8" || v.Note != "" {
		t.Fatalf("version = %+v", v)
	}
	f.register(map[string]any{"number": "v2026.08"}).ExpectStatus(http.StatusCreated)
	f.register(map[string]any{"number": "2.1.0-nw.2"}).ExpectStatus(http.StatusCreated)
}

func TestVersionFieldsHaveLengthLimits(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	invalid := http.StatusUnprocessableEntity
	f.register(map[string]any{"number": strings.Repeat("9", 101)}).ExpectError(invalid, "number", "tooLong")
	f.register(map[string]any{"ref": map[string]any{"kind": "tag", "value": strings.Repeat("t", 256)}}).ExpectError(invalid, "ref.value", "tooLong")
	f.register(map[string]any{"note": strings.Repeat("n", 2001)}).ExpectError(invalid, "note", "tooLong")
	f.register(map[string]any{"number": strings.Repeat("9", 100), "note": strings.Repeat("n", 2000)}).ExpectStatus(http.StatusCreated)
}

func TestOnlyAnOwnerOfTheApplicationCanRegisterVersions(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	dbrooks := createUser(t, f.admin, map[string]any{"username": "dbrooks", "displayName": "Daniel Brooks"})
	// dbrooks owns another application, where elarsen is only a PE.
	other := f.secondApp(t, dbrooks.ID, map[string]any{"pes": []string{f.owner.ID}})

	path := versionsPath("payments-core", f.mainline.ID)
	bob := plainUserClient(t, f.env, "bob")
	forbidden := http.StatusForbidden
	f.admin.Post(path, newVersion(nil)).ExpectError(forbidden, "", "forbidden") // an administrator who is not an Owner
	f.login("praman").Post(path, newVersion(nil)).ExpectError(forbidden, "", "forbidden")
	f.login("dbrooks").Post(path, newVersion(nil)).ExpectError(forbidden, "", "forbidden") // Owner of another application
	bob.Post(path, newVersion(nil)).ExpectError(forbidden, "", "forbidden")
	f.admin.Post(path, map[string]any{}).ExpectError(forbidden, "", "forbidden") // permission is checked before the fields
	f.elarsen.Post(versionsPath("rate-engine", other.ID), newVersion(nil)).ExpectError(forbidden, "", "forbidden")
	f.env.Client().Post(path, newVersion(nil)).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	if got := f.list(f.admin); len(got) != 0 {
		t.Fatalf("forbidden requests stored versions: %+v", got)
	}

	// Everyone logged in can read them.
	f.register(nil).ExpectStatus(http.StatusCreated)
	for _, c := range []*apitest.Client{f.admin, f.login("praman"), f.login("dbrooks"), bob} {
		if got := f.list(c); len(got) != 1 {
			t.Fatalf("list = %+v", got)
		}
	}
	f.env.Client().Get("/api/applications/payments-core/versions").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}

func TestRegisteringOnAnUnknownApplicationOrLineIsNotFound(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	notFound := http.StatusNotFound
	f.elarsen.Post(versionsPath("nope", f.mainline.ID), newVersion(nil)).ExpectError(notFound, "", "notFound")
	f.elarsen.Get("/api/applications/nope/versions").ExpectError(notFound, "", "notFound")
	f.elarsen.Post(versionsPath("payments-core", "999999"), newVersion(nil)).ExpectError(notFound, "", "notFound")
	f.elarsen.Post(versionsPath("payments-core", "main"), newVersion(nil)).ExpectError(notFound, "", "notFound")

	// A line of another application is not a line of this one.
	other := f.secondApp(t, f.owner.ID, nil)
	f.elarsen.Post(versionsPath("payments-core", other.ID), newVersion(nil)).ExpectError(notFound, "", "notFound")
	if got := f.list(f.elarsen); len(got) != 0 {
		t.Fatalf("versions = %+v", got)
	}
}

func TestAVersionNumberIsUniqueWithinTheApplicationAndTheConflictSaysWhere(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	f.register(map[string]any{"number": "2.1.0", "releasedAt": "2025-02-18"}).ExpectStatus(http.StatusCreated)

	e := f.register(map[string]any{"number": " 2.1.0 ", "releasedAt": "2026-01-01", "ref": map[string]any{"kind": "commit", "value": "abc"}}).
		ExpectError(http.StatusUnprocessableEntity, "number", "numberTaken")
	line, _ := e.Params["line"].(map[string]any)
	if e.Params["releasedAt"] != "2025-02-18" || line["id"] != f.mainline.ID || line["isMain"] != true || line["branch"] != "develop" {
		t.Fatalf("params = %v", e.Params)
	}
	if got := f.list(f.elarsen); len(got) != 1 {
		t.Fatalf("a rejected duplicate was stored: %+v", got)
	}

	// Numbers are compared exactly: the format is not restricted and tags are case-sensitive.
	f.register(map[string]any{"number": "V2.1.0"}).ExpectStatus(http.StatusCreated)

	// Another application may use the same number.
	other := f.secondApp(t, f.owner.ID, nil)
	f.elarsen.Post(versionsPath("rate-engine", other.ID), newVersion(map[string]any{"number": "2.1.0"})).ExpectStatus(http.StatusCreated)
}

func TestVersionsAreListedByReleaseDateNotByNumberOrRegistrationOrder(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	// The number that looks highest is not the latest.
	for _, v := range []struct{ number, date string }{
		{"2.3.2", "2026-03-17"}, {"v2026.08", "2026-08-12"}, {"2.3.1", "2025-11-03"}, {"9.9.9", "2025-11-03"},
	} {
		f.register(map[string]any{"number": v.number, "releasedAt": v.date}).ExpectStatus(http.StatusCreated)
	}
	var got []string
	for _, v := range f.list(f.elarsen) {
		got = append(got, v.Number)
	}
	// On the same date the one registered later comes first.
	if want := []string{"v2026.08", "2.3.2", "9.9.9", "2.3.1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestRegisteringAVersionIsRecordedInTheChangeHistory(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	f.register(map[string]any{"number": "3.2.0"}).ExpectStatus(http.StatusCreated)
	f.register(map[string]any{"number": "3.2.0"}).ExpectStatus(http.StatusUnprocessableEntity) // a rejected registration leaves no event

	var h struct {
		Items []struct {
			Type  string `json:"type"`
			Actor struct {
				Username string `json:"username"`
			} `json:"actor"`
			Params struct {
				Number string   `json:"number"`
				Line   lineBody `json:"line"`
			} `json:"params"`
		} `json:"items"`
	}
	plainUserClient(t, f.env, "bob").Get("/api/applications/payments-core/history").ExpectStatus(http.StatusOK).JSON(&h)
	if len(h.Items) != 2 {
		t.Fatalf("history = %+v", h.Items)
	}
	e := h.Items[0]
	if e.Type != "versionRegistered" || e.Actor.Username != "elarsen" || e.Params.Number != "3.2.0" ||
		!e.Params.Line.IsMain || e.Params.Line.ID != f.mainline.ID || e.Params.Line.Branch != "develop" {
		t.Fatalf("event = %+v", e)
	}
	if h.Items[1].Type != "appCreated" {
		t.Fatalf("older event = %+v", h.Items[1])
	}
}
