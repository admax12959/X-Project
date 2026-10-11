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

// -- tests below are appended as the lifecycle grows --
