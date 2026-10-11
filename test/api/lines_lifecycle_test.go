package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// Line lifecycle (spec #2, ticket #10), through the HTTP API only. Lifecycle tests
// move lines with the real status API; apitest.SeedLineStatus stays a fixture helper.

// lineFixture is the version fixture plus one maintenance line forked from mainline
// version 2.1.0.
type lineFixture struct {
	versionFixture
	line lineDetailBody
}

func newLineFixture(t *testing.T) lineFixture {
	t.Helper()
	f := newVersionFixture(t)
	src := f.registerMain(t, "2.1.0", "2025-02-18")
	var line lineDetailBody
	f.fork(src.ID, map[string]any{"plannedEnd": "2027-06-30", "note": "Contract renewal being negotiated"}).
		ExpectStatus(http.StatusCreated).JSON(&line)
	return lineFixture{versionFixture: f, line: line}
}

func (f lineFixture) path() string { return linesPath("payments-core") + "/" + f.line.ID }

// setStatus is the Owner's status change on the fixture's line.
func (f lineFixture) setStatus(status string) *apitest.Response {
	return f.elarsen.Do(http.MethodPut, f.path()+"/status", map[string]any{"status": status})
}

func (f lineFixture) statusOf(t *testing.T, lineID string) string {
	t.Helper()
	for _, l := range f.listLines(f.elarsen) {
		if l.ID == lineID {
			return l.Status
		}
	}
	t.Fatalf("line %s not listed", lineID)
	return ""
}

func TestOwnerFreezesAnActiveLine(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)

	var got lineDetailBody
	f.setStatus("frozen").ExpectStatus(http.StatusOK).JSON(&got)

	if got.ID != f.line.ID || got.Status != "frozen" || got.Branch != "cust/northwind" {
		t.Fatalf("response = %+v", got)
	}
	if s := f.statusOf(t, f.line.ID); s != "frozen" {
		t.Fatalf("listed status = %q", s)
	}
}

func TestOwnerReturnsAFrozenLineToActive(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	f.setStatus("frozen").ExpectStatus(http.StatusOK)

	var got lineDetailBody
	f.setStatus("active").ExpectStatus(http.StatusOK).JSON(&got)

	if got.Status != "active" || f.statusOf(t, f.line.ID) != "active" {
		t.Fatalf("response = %+v", got)
	}
}

func TestOwnerRetiresAFrozenLine(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	f.setStatus("frozen").ExpectStatus(http.StatusOK)

	var got lineDetailBody
	f.setStatus("retired").ExpectStatus(http.StatusOK).JSON(&got)

	if got.Status != "retired" || f.statusOf(t, f.line.ID) != "retired" {
		t.Fatalf("response = %+v", got)
	}
}

// path from the freshly forked (active) line to each status, by legal moves only.
var pathTo = map[string][]string{
	"active":  nil,
	"frozen":  {"frozen"},
	"retired": {"frozen", "retired"},
}

func (f lineFixture) moveTo(t *testing.T, status string) {
	t.Helper()
	for _, step := range pathTo[status] {
		f.setStatus(step).ExpectStatus(http.StatusOK)
	}
}

func TestEveryIllegalStatusTransitionIsRejectedAndChangesNothing(t *testing.T) {
	t.Parallel()
	illegal := [][2]string{
		{"active", "retired"}, // must be frozen first
		{"active", "active"},
		{"frozen", "frozen"},
		{"retired", "active"}, // retiring is final
		{"retired", "frozen"},
		{"retired", "retired"},
	}
	for _, c := range illegal {
		from, to := c[0], c[1]
		t.Run(from+" to "+to, func(t *testing.T) {
			t.Parallel()
			f := newLineFixture(t)
			f.moveTo(t, from)

			res := f.setStatus(to)
			res.ExpectError(http.StatusUnprocessableEntity, "status", "invalidTransition")
			params := res.Errors()[0].Params
			if params["from"] != from || params["to"] != to {
				t.Fatalf("params = %+v", params)
			}
			if got := f.statusOf(t, f.line.ID); got != from {
				t.Fatalf("status after the rejected change = %q, want %q", got, from)
			}
		})
	}
}

func TestAStatusChangeNeedsAKnownTargetStatus(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	invalid := http.StatusUnprocessableEntity

	f.setStatus("").ExpectError(invalid, "status", "required")
	f.setStatus("  ").ExpectError(invalid, "status", "required")
	f.setStatus("archived").ExpectError(invalid, "status", "invalid")
	f.setStatus("Frozen").ExpectError(invalid, "status", "invalid") // values are lowercase
	if got := f.statusOf(t, f.line.ID); got != "active" {
		t.Fatalf("status = %q", got)
	}
}

func TestOnlyAnOwnerOfTheApplicationCanChangeALinesStatus(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	praman := createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	dbrooks := createUser(t, f.admin, map[string]any{"username": "dbrooks", "displayName": "Daniel Brooks"})
	f.admin.Do(http.MethodPut, "/api/applications/payments-core/members", map[string]any{
		"owners": []string{f.owner.ID}, "pes": []string{praman.ID},
	}).ExpectStatus(http.StatusOK)
	f.secondApp(t, dbrooks.ID, nil) // dbrooks owns another application

	forbidden := http.StatusForbidden
	body := map[string]any{"status": "frozen"}
	// An administrator who is not an Owner, a PE, the Owner of another application, a plain user.
	for _, c := range []*apitest.Client{f.admin, f.login("praman"), f.login("dbrooks"), f.env.SeedUserClient("bob", "bob-password-1")} {
		c.Do(http.MethodPut, f.path()+"/status", body).ExpectError(forbidden, "", "forbidden")
		c.Do(http.MethodPut, f.path()+"/status", map[string]any{}).ExpectError(forbidden, "", "forbidden") // before the fields
	}
	f.env.Client().Do(http.MethodPut, f.path()+"/status", body).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	if got := f.statusOf(t, f.line.ID); got != "active" {
		t.Fatalf("a forbidden request changed the status to %q", got)
	}
}

func (f lineFixture) adminID(t *testing.T) string {
	t.Helper()
	var me struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	f.admin.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&me)
	return me.User.ID
}

func TestAnAdministratorWhoIsAlsoAnOwnerMayChangeALinesStatus(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	f.admin.Do(http.MethodPut, "/api/applications/payments-core/members", map[string]any{
		"owners": []string{f.owner.ID, f.adminID(t)}, "pes": []string{},
	}).ExpectStatus(http.StatusOK)

	f.admin.Do(http.MethodPut, f.path()+"/status", map[string]any{"status": "frozen"}).ExpectStatus(http.StatusOK)
}

func TestTheMainlineHasNoStatusAndNoStatusOperation(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	mainPath := linesPath("payments-core") + "/" + f.mainline.ID

	for _, status := range []string{"active", "frozen", "retired"} {
		f.elarsen.Do(http.MethodPut, mainPath+"/status", map[string]any{"status": status}).
			ExpectError(http.StatusUnprocessableEntity, "", "lineIsMainline")
	}
	for _, l := range f.listLines(f.elarsen) {
		if l.IsMain && l.Status != "" {
			t.Fatalf("the mainline got a status: %+v", l)
		}
	}
}

func TestChangingTheStatusOfAnUnknownLineOrApplicationIsNotFound(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	notFound := http.StatusNotFound
	body := map[string]any{"status": "frozen"}
	put := func(path string) *apitest.Response { return f.elarsen.Do(http.MethodPut, path, body) }

	put("/api/applications/nope/lines/"+f.line.ID+"/status").ExpectError(notFound, "", "notFound")
	put(linesPath("payments-core")+"/999999/status").ExpectError(notFound, "", "notFound")
	put(linesPath("payments-core")+"/main/status").ExpectError(notFound, "", "notFound")
	// A line of another application is not a line of this one.
	other := f.secondApp(t, f.owner.ID, nil)
	put(linesPath("payments-core")+"/"+other.ID+"/status").ExpectError(notFound, "", "notFound")
	if got := f.statusOf(t, f.line.ID); got != "active" {
		t.Fatalf("status = %q", got)
	}
}

// registerOnLine registers a version with a note on the fixture's line.
func (f lineFixture) registerOnLine(number, note string) *apitest.Response {
	return f.elarsen.Post(versionsPath("payments-core", f.line.ID), newVersion(map[string]any{
		"number": number, "releasedAt": "2025-04-03", "note": note,
	}))
}

func (f lineFixture) versionCount(t *testing.T) int {
	t.Helper()
	return len(f.list(f.elarsen))
}

func TestRegisteringOnAFrozenLineNeedsANote(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	f.setStatus("frozen").ExpectStatus(http.StatusOK)
	before := f.versionCount(t)

	f.registerOnLine("2.1.0-nw.1", "").ExpectError(http.StatusUnprocessableEntity, "note", "noteRequiredOnFrozen")
	f.registerOnLine("2.1.0-nw.1", "   \n").ExpectError(http.StatusUnprocessableEntity, "note", "noteRequiredOnFrozen")
	if got := f.versionCount(t); got != before {
		t.Fatalf("a rejected registration stored a version: %d versions, was %d", got, before)
	}

	var v versionBody
	f.registerOnLine("2.1.0-nw.1", "Hotfix: rounding error in settlement totals").ExpectStatus(http.StatusCreated).JSON(&v)
	if v.LineID != f.line.ID || v.Note != "Hotfix: rounding error in settlement totals" {
		t.Fatalf("version = %+v", v)
	}
}

func TestTheNoteIsOptionalAgainOnceAFrozenLineIsActive(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	f.registerOnLine("2.1.0-nw.0", "").ExpectStatus(http.StatusCreated) // active: note optional
	f.setStatus("frozen").ExpectStatus(http.StatusOK)
	f.setStatus("active").ExpectStatus(http.StatusOK)

	f.registerOnLine("2.1.0-nw.1", "").ExpectStatus(http.StatusCreated)
}

func TestNothingCanBeRegisteredOnARetiredLine(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	f.moveTo(t, "retired")
	before := f.versionCount(t)

	// The error is request-level: the line is in the URL, not in the body.
	f.registerOnLine("2.1.0-nw.1", "").ExpectError(http.StatusUnprocessableEntity, "", "lineRetired")
	f.registerOnLine("2.1.0-nw.1", "With a note, still refused").ExpectError(http.StatusUnprocessableEntity, "", "lineRetired")
	if got := f.versionCount(t); got != before {
		t.Fatalf("a retired line took a version: %d versions, was %d", got, before)
	}
}

func TestTheMainlineAcceptsVersionsWithoutANote(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	f.register(map[string]any{"number": "2.2.0", "note": ""}).ExpectStatus(http.StatusCreated)
}

// statusChangeBody is one entry of a line's status log.
type statusChangeBody struct {
	From  string `json:"from"`
	To    string `json:"to"`
	At    string `json:"at"`
	Actor struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"actor"`
}

// lineWithLogBody is GET /lines/{id}: the line plus its status log.
type lineWithLogBody struct {
	lineDetailBody
	StatusChanges []statusChangeBody `json:"statusChanges"`
}

func (f lineFixture) detail(t *testing.T, c *apitest.Client) lineWithLogBody {
	t.Helper()
	var l lineWithLogBody
	c.Get(f.path()).ExpectStatus(http.StatusOK).JSON(&l)
	return l
}

func TestEveryStatusChangeIsRecordedWithTimeAndActorAndShownInTheLineDetail(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	praman := createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	f.admin.Do(http.MethodPut, "/api/applications/payments-core/members", map[string]any{
		"owners": []string{f.owner.ID, praman.ID}, "pes": []string{},
	}).ExpectStatus(http.StatusOK)
	bob := f.env.SeedUserClient("bob", "bob-password-1")

	if l := f.detail(t, bob); len(l.StatusChanges) != 0 || l.Status != "active" || l.ForkedAt == "" {
		t.Fatalf("a new line has no status changes yet: %+v", l)
	}
	f.setStatus("frozen").ExpectStatus(http.StatusOK)                                                // elarsen
	f.setStatus("retired").ExpectStatus(http.StatusOK)                                               // elarsen
	f.setStatus("active").ExpectError(http.StatusUnprocessableEntity, "status", "invalidTransition") // rejected: not recorded
	f.login("praman").Do(http.MethodPut, f.path()+"/status", map[string]any{"status": "active"}).ExpectStatus(http.StatusUnprocessableEntity)

	l := f.detail(t, bob) // any logged-in user may read it
	if l.Status != "retired" || l.Branch != "cust/northwind" || l.Reason == "" || l.SourceVersion == nil {
		t.Fatalf("line = %+v", l)
	}
	want := []struct{ from, to, actor string }{{"active", "frozen", "elarsen"}, {"frozen", "retired", "elarsen"}}
	if len(l.StatusChanges) != len(want) {
		t.Fatalf("status changes = %+v", l.StatusChanges)
	}
	prev := ""
	for i, w := range want {
		c := l.StatusChanges[i]
		if c.From != w.from || c.To != w.to || c.Actor.Username != w.actor || c.Actor.DisplayName != "Emma Larsen" || c.At == "" || c.At < prev {
			t.Fatalf("status change %d = %+v, want %+v after %q", i, c, w, prev)
		}
		prev = c.At
	}
}

func TestAnotherOwnersStatusChangeIsRecordedWithTheirName(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	praman := createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	f.admin.Do(http.MethodPut, "/api/applications/payments-core/members", map[string]any{
		"owners": []string{f.owner.ID, praman.ID}, "pes": []string{},
	}).ExpectStatus(http.StatusOK)

	f.setStatus("frozen").ExpectStatus(http.StatusOK)
	f.login("praman").Do(http.MethodPut, f.path()+"/status", map[string]any{"status": "active"}).ExpectStatus(http.StatusOK)

	got := f.detail(t, f.elarsen).StatusChanges
	if len(got) != 2 || got[0].Actor.Username != "elarsen" || got[1].Actor.Username != "praman" || got[1].From != "frozen" || got[1].To != "active" {
		t.Fatalf("status changes = %+v", got)
	}
}

func TestTheDetailOfAnUnknownLineIsNotFoundAndTheMainlineHasNoStatusLog(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	f.elarsen.Get(linesPath("payments-core")+"/999999").ExpectError(http.StatusNotFound, "", "notFound")
	f.elarsen.Get(linesPath("nope")+"/"+f.line.ID).ExpectError(http.StatusNotFound, "", "notFound")
	other := f.secondApp(t, f.owner.ID, nil)
	f.elarsen.Get(linesPath("payments-core")+"/"+other.ID).ExpectError(http.StatusNotFound, "", "notFound")

	var main lineWithLogBody
	f.elarsen.Get(linesPath("payments-core") + "/" + f.mainline.ID).ExpectStatus(http.StatusOK).JSON(&main)
	if !main.IsMain || main.Status != "" || len(main.StatusChanges) != 0 {
		t.Fatalf("mainline = %+v", main)
	}
}

// lifecycleEventBody is a change-history entry of a status change or a line edit.
type lifecycleEventBody struct {
	Type  string `json:"type"`
	Actor struct {
		Username string `json:"username"`
	} `json:"actor"`
	Params struct {
		Line    lineBody                     `json:"line"`
		From    string                       `json:"from"`
		To      string                       `json:"to"`
		Changes map[string]map[string]string `json:"changes"`
	} `json:"params"`
}

func (f lineFixture) events(t *testing.T) []lifecycleEventBody {
	t.Helper()
	var h struct {
		Items []lifecycleEventBody `json:"items"`
	}
	f.elarsen.Get("/api/applications/payments-core/history").ExpectStatus(http.StatusOK).JSON(&h)
	return h.Items
}

func TestStatusChangesAreRecordedInTheChangeHistory(t *testing.T) {
	t.Parallel()
	f := newLineFixture(t)
	base := len(f.events(t)) // app created, version registered, line forked

	f.setStatus("frozen").ExpectStatus(http.StatusOK)
	f.setStatus("active").ExpectStatus(http.StatusOK)
	f.setStatus("frozen").ExpectStatus(http.StatusOK)
	f.setStatus("retired").ExpectStatus(http.StatusOK)
	f.setStatus("active").ExpectStatus(http.StatusUnprocessableEntity) // a rejected change leaves no event

	h := f.events(t) // newest first
	if len(h) != base+4 {
		t.Fatalf("history has %d events, want %d: %+v", len(h), base+4, h)
	}
	want := [][2]string{{"frozen", "retired"}, {"active", "frozen"}, {"frozen", "active"}, {"active", "frozen"}}
	for i, w := range want {
		e := h[i]
		if e.Type != "lineStatusChanged" || e.Actor.Username != "elarsen" || e.Params.From != w[0] || e.Params.To != w[1] ||
			e.Params.Line.ID != f.line.ID || e.Params.Line.Name != "Northwind customization" || e.Params.Line.Branch != "cust/northwind" || e.Params.Line.IsMain {
			t.Fatalf("event %d = %+v, want %v", i, e, w)
		}
	}
}

// -- tests below are appended as the lifecycle grows --
