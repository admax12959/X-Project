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
