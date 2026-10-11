package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// Seam (spec #2, Testing Decisions): the HTTP API against a real MySQL. No SQL, no
// internal calls; the one exception is apitest.SeedLineStatus (test-only, see there), used only by
// the seedPaymentsCore fixture. Lifecycle behaviour is tested through the status API.

// sourceVersionBody is the source version of a line: where it was forked from.
type sourceVersionBody struct {
	ID         string   `json:"id"`
	Number     string   `json:"number"`
	ReleasedAt string   `json:"releasedAt"`
	Line       lineBody `json:"line"`
}

// lineDetailBody is a line as GET /lines and POST /lines return it.
type lineDetailBody struct {
	ID            string             `json:"id"`
	IsMain        bool               `json:"isMain"`
	Name          string             `json:"name"`
	Branch        string             `json:"branch"`
	Status        string             `json:"status"`
	SourceVersion *sourceVersionBody `json:"sourceVersion"`
	Reason        string             `json:"reason"`
	PlannedEnd    string             `json:"plannedEnd"`
	Note          string             `json:"note"`
	ForkedAt      string             `json:"forkedAt"`
}

func linesPath(appKey string) string { return "/api/applications/" + appKey + "/lines" }

// newFork is a valid fork request; tests override single fields.
func newFork(sourceVersionID string, over map[string]any) map[string]any {
	m := map[string]any{
		"sourceVersionId": sourceVersionID,
		"name":            "Northwind customization",
		"branch":          "cust/northwind",
		"reason":          "The Northwind contract requires a custom statement format.",
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

// registerMain registers a version on the mainline and returns it.
func (f versionFixture) registerMain(t *testing.T, number, releasedAt string) versionBody {
	t.Helper()
	var v versionBody
	f.register(map[string]any{"number": number, "releasedAt": releasedAt, "ref": map[string]any{"kind": "tag", "value": "v" + number}}).
		ExpectStatus(http.StatusCreated).JSON(&v)
	return v
}

func (f versionFixture) fork(sourceVersionID string, over map[string]any) *apitest.Response {
	return f.elarsen.Post(linesPath("payments-core"), newFork(sourceVersionID, over))
}

func TestOwnerForksAMaintenanceLineFromAMainlineVersion(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	src := f.registerMain(t, "2.1.0", "2025-02-18")

	var line lineDetailBody
	f.fork(src.ID, map[string]any{"plannedEnd": "2027-06-30", "note": "Contract renewal being negotiated"}).
		ExpectStatus(http.StatusCreated).JSON(&line)

	if line.ID == "" || line.IsMain || line.Name != "Northwind customization" || line.Branch != "cust/northwind" ||
		line.Status != "active" || line.Reason != "The Northwind contract requires a custom statement format." ||
		line.PlannedEnd != "2027-06-30" || line.Note != "Contract renewal being negotiated" || line.ForkedAt == "" {
		t.Fatalf("forked line = %+v", line)
	}
	if sv := line.SourceVersion; sv == nil || sv.ID != src.ID || sv.Number != "2.1.0" || sv.ReleasedAt != "2025-02-18" ||
		sv.Line.ID != f.mainline.ID || !sv.Line.IsMain || sv.Line.Branch != "develop" {
		t.Fatalf("source version = %+v", line.SourceVersion)
	}

	// Everyone sees it in the line list, after the mainline; the mainline has no status.
	var list struct {
		Items []lineDetailBody `json:"items"`
	}
	f.env.SeedUserClient("bob", "bob-password-1").Get(linesPath("payments-core")).ExpectStatus(http.StatusOK).JSON(&list)
	if len(list.Items) != 2 || !list.Items[0].IsMain || list.Items[0].Status != "" || list.Items[0].SourceVersion != nil {
		t.Fatalf("lines = %+v", list.Items)
	}
	if got := list.Items[1]; got.ID != line.ID || got.Status != "active" || got.SourceVersion == nil || got.SourceVersion.Number != "2.1.0" {
		t.Fatalf("listed fork = %+v", got)
	}
}
