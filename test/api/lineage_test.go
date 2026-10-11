package api

import (
	"net/http"
	"reflect"
	"testing"
)

// The payments-core lineage scenario from the prototype seed (docs: seed scenarios
// of spec #2): four levels of nesting, one version forked into two lines, a line
// without versions, irregular version numbers, all three statuses. Later tickets
// (lifecycle #10, graph column #11) reuse seedPaymentsCore.

type paymentsCore struct {
	versionFixture
	// line maps a Git branch to its line ID; version maps a version number to its ID.
	line    map[string]string
	version map[string]string
}

// newMainlineFixture is a versionFixture whose mainline branch is main (the version
// fixture's own mainline is develop).
func newMainlineFixture(t *testing.T) versionFixture {
	t.Helper()
	f := newAppFixture(t)
	f.post(nil).ExpectStatus(http.StatusCreated)
	owner := f.env.Client()
	owner.LoginLocal("elarsen", "initial-pw-1").ExpectStatus(http.StatusOK)
	var lines struct {
		Items []lineBody `json:"items"`
	}
	owner.Get(linesPath("payments-core")).ExpectStatus(http.StatusOK).JSON(&lines)
	return versionFixture{appFixture: f, elarsen: owner, mainline: lines.Items[0]}
}

type seedVersion struct {
	number, releasedAt, refKind, refValue, note string
}

func (p *paymentsCore) register(t *testing.T, lineID string, v seedVersion) {
	t.Helper()
	if v.refKind == "" {
		v.refKind, v.refValue = "tag", "v"+v.number
	}
	var out versionBody
	p.elarsen.Post(versionsPath("payments-core", lineID), newVersion(map[string]any{
		"number": v.number, "releasedAt": v.releasedAt, "note": v.note, "ref": map[string]any{"kind": v.refKind, "value": v.refValue},
	})).ExpectStatus(http.StatusCreated).JSON(&out)
	p.version[v.number] = out.ID
}

func (p *paymentsCore) forkFrom(t *testing.T, number, name, branch, reason string, over map[string]any) {
	t.Helper()
	body := map[string]any{"name": name, "branch": branch, "reason": reason}
	for k, v := range over {
		body[k] = v
	}
	var out lineDetailBody
	p.fork(p.version[number], body).ExpectStatus(http.StatusCreated).JSON(&out)
	p.line[branch] = out.ID
}

// seedPaymentsCore builds the scenario through the API. Lines are created in an order
// that differs from the display order (the LTS line before Northwind), versions are
// registered out of release order on the LTS line, and the two lines forked from 2.1.0
// are created Northwind first. Retired and Frozen are applied last with the test-only
// SeedLineStatus (the lifecycle API is a later ticket).
func seedPaymentsCore(t *testing.T) *paymentsCore {
	t.Helper()
	p := &paymentsCore{versionFixture: newMainlineFixture(t), line: map[string]string{"main": ""}, version: map[string]string{}}
	p.line["main"] = p.mainline.ID

	for _, v := range []seedVersion{
		{number: "1.0.0", releasedAt: "2024-03-12", note: "First general release"},
		{number: "1.1.0", releasedAt: "2024-06-20"},
		{number: "2.0.0", releasedAt: "2024-11-05", note: "New ledger model"},
		{number: "2.1.0", releasedAt: "2025-02-18"},
		{number: "2.2.0", releasedAt: "2025-05-27"},
		{number: "2.3.0", releasedAt: "2025-08-14"},
		{number: "3.0.0", releasedAt: "2026-01-09", note: "Switch to the new settlement engine"},
		{number: "3.1.0", releasedAt: "2026-04-22"},
		{number: "3.2.0", releasedAt: "2026-08-30"},
	} {
		p.register(t, p.mainline.ID, v)
	}

	p.forkFrom(t, "1.1.0", "1.x PCI freeze", "maint/1.x-pci",
		"Held at 1.1 during the 2024 PCI DSS assessment; security fixes only until sign-off.", nil)
	p.register(t, p.line["maint/1.x-pci"], seedVersion{number: "1.1.1", releasedAt: "2024-08-02", refKind: "commit", refValue: "4e1a9c2",
		note: "Fix broken access control on statement queries"})
	p.register(t, p.line["maint/1.x-pci"], seedVersion{number: "1.1.2", releasedAt: "2024-12-11"})

	p.forkFrom(t, "2.3.0", "2.x long-term support", "release/2.x-lts",
		"Customers on 2.x cannot move to 3.x yet. Security fixes continue on 2.x.", map[string]any{"plannedEnd": "2027-03-31"})
	for _, v := range []seedVersion{ // out of release order; the number that looks highest is not the latest
		{number: "v2026.08", releasedAt: "2026-08-12", refKind: "commit", refValue: "9f03be1", note: "Date-style number; the format is not enforced"},
		{number: "2.3.1", releasedAt: "2025-11-03"},
		{number: "2.3.2", releasedAt: "2026-03-17"},
	} {
		p.register(t, p.line["release/2.x-lts"], v)
	}

	p.forkFrom(t, "2.1.0", "Northwind customization", "cust/northwind",
		"The Northwind contract requires a custom statement format and settlement cycle. Mainline will not take these changes.",
		map[string]any{"plannedEnd": "2027-06-30", "note": "Contract renewal being negotiated"})
	for _, v := range []seedVersion{
		{number: "2.1.0-nw.1", releasedAt: "2025-04-03"},
		{number: "2.1.0-nw.2", releasedAt: "2025-07-15"},
		{number: "2.1.0-nw.3", releasedAt: "2026-02-26"},
	} {
		p.register(t, p.line["cust/northwind"], v)
	}
	// The same version forked into a second line, which has no version yet.
	p.forkFrom(t, "2.1.0", "Helvetia Bank edition", "cust/helvetia",
		"Helvetia Bank needs HSM-backed key management and an on-prem deployment.", map[string]any{"note": "First version not yet registered"})

	p.forkFrom(t, "2.1.0-nw.2", "Northwind EMEA", "cust/northwind-emea",
		"The EMEA subsidiary needs SEPA-formatted reports, which conflict with the Northwind HQ customization.", nil)
	p.register(t, p.line["cust/northwind-emea"], seedVersion{number: "emea-1.0", releasedAt: "2025-10-20", refValue: "northwind-emea-1.0"})
	p.register(t, p.line["cust/northwind-emea"], seedVersion{number: "emea-1.1", releasedAt: "2026-01-30", refKind: "commit", refValue: "b71d0e8"})

	p.forkFrom(t, "emea-1.1", "Frankfurt pilot", "cust/northwind-emea-de",
		"The Frankfurt pilot adds a Bundesbank reporting channel, kept apart from EMEA until the pilot proves out.", nil)
	p.register(t, p.line["cust/northwind-emea-de"], seedVersion{number: "emea-1.1-de1", releasedAt: "2026-05-08", refValue: "de-emea-1.1-1"})

	p.env.SeedLineStatus("payments-core", "maint/1.x-pci", "retired") // TEST ONLY: bypasses the state machine
	p.env.SeedLineStatus("payments-core", "cust/northwind-emea", "frozen")
	return p
}

type latestVersionBody struct {
	ID         string `json:"id"`
	Number     string `json:"number"`
	ReleasedAt string `json:"releasedAt"`
}

// lineageNodeBody is a node of GET /lineage: the line itself plus what the tree shows.
type lineageNodeBody struct {
	lineDetailBody
	VersionCount  int                `json:"versionCount"`
	LatestVersion *latestVersionBody `json:"latestVersion"`
	Children      []lineageNodeBody  `json:"children"`
}

func (p *paymentsCore) lineage(t *testing.T) lineageNodeBody {
	t.Helper()
	var root lineageNodeBody
	p.env.SeedUserClient("bob", "bob-password-1").Get("/api/applications/payments-core/lineage").ExpectStatus(http.StatusOK).JSON(&root)
	return root
}

func (n lineageNodeBody) child(t *testing.T, branch string) lineageNodeBody {
	t.Helper()
	for _, c := range n.Children {
		if c.Branch == branch {
			return c
		}
	}
	t.Fatalf("%s has no child %s: %+v", n.Branch, branch, n.Children)
	return lineageNodeBody{}
}

func branches(nodes []lineageNodeBody) []string {
	out := []string{}
	for _, n := range nodes {
		out = append(out, n.Branch)
	}
	return out
}

func TestTheLineageNestsLinesUnderTheLineHoldingTheirSourceVersion(t *testing.T) {
	t.Parallel()
	p := seedPaymentsCore(t)
	root := p.lineage(t)

	// Mainline at the top: no status, no fork facts.
	if !root.IsMain || root.Branch != "main" || root.ID != p.mainline.ID || root.Status != "" || root.SourceVersion != nil || root.ForkedAt != "" {
		t.Fatalf("root = %+v", root.lineDetailBody)
	}
	// Children ordered by the release date of the source version, then creation: the LTS line
	// (2.3.0) was created before Northwind but comes last; Northwind before Helvetia (both 2.1.0).
	want := []string{"maint/1.x-pci", "cust/northwind", "cust/helvetia", "release/2.x-lts"}
	if got := branches(root.Children); !reflect.DeepEqual(got, want) {
		t.Fatalf("mainline children = %v, want %v", got, want)
	}

	// Four levels: Mainline > Northwind > Northwind EMEA > Frankfurt pilot.
	northwind := root.child(t, "cust/northwind")
	emea := northwind.child(t, "cust/northwind-emea")
	de := emea.child(t, "cust/northwind-emea-de")
	if len(de.Children) != 0 || len(emea.Children) != 1 || len(northwind.Children) != 1 {
		t.Fatalf("northwind=%v emea=%v de=%v", branches(northwind.Children), branches(emea.Children), branches(de.Children))
	}
	if sv := emea.SourceVersion; sv == nil || sv.Number != "2.1.0-nw.2" || sv.ReleasedAt != "2025-07-15" || sv.Line.ID != northwind.ID || sv.Line.Branch != "cust/northwind" {
		t.Fatalf("emea source = %+v", emea.SourceVersion)
	}
	if sv := de.SourceVersion; sv == nil || sv.Number != "emea-1.1" || sv.Line.ID != emea.ID {
		t.Fatalf("de source = %+v", de.SourceVersion)
	}

	// One version forked into two lines: both hang under the mainline with the same source.
	helvetia := root.child(t, "cust/helvetia")
	if northwind.SourceVersion.ID != p.version["2.1.0"] || helvetia.SourceVersion.ID != p.version["2.1.0"] {
		t.Fatalf("sources = %+v / %+v", northwind.SourceVersion, helvetia.SourceVersion)
	}

	// A line without versions is shown, with nothing latest and no children.
	if helvetia.VersionCount != 0 || helvetia.LatestVersion != nil || len(helvetia.Children) != 0 || helvetia.Status != "active" ||
		helvetia.Note != "First version not yet registered" {
		t.Fatalf("helvetia = %+v", helvetia)
	}
}

func TestTheLineageCarriesStatusForkFactsAndVersionSummaries(t *testing.T) {
	t.Parallel()
	p := seedPaymentsCore(t)
	root := p.lineage(t)
	pci := root.child(t, "maint/1.x-pci")
	northwind := root.child(t, "cust/northwind")
	emea := northwind.child(t, "cust/northwind-emea")
	de := emea.child(t, "cust/northwind-emea-de")
	lts := root.child(t, "release/2.x-lts")

	// All three statuses, lowercase; a retired line's own versions still count.
	if pci.Status != "retired" || emea.Status != "frozen" || northwind.Status != "active" || de.Status != "active" {
		t.Fatalf("statuses: %q %q %q %q", pci.Status, emea.Status, northwind.Status, de.Status)
	}
	if pci.Name != "1.x PCI freeze" || pci.Reason != "Held at 1.1 during the 2024 PCI DSS assessment; security fixes only until sign-off." ||
		pci.SourceVersion.Number != "1.1.0" || pci.ForkedAt == "" || pci.PlannedEnd != "" {
		t.Fatalf("pci = %+v", pci.lineDetailBody)
	}
	// Planned end is an informational date; it exists on two lines only.
	if northwind.PlannedEnd != "2027-06-30" || lts.PlannedEnd != "2027-03-31" || northwind.Note != "Contract renewal being negotiated" {
		t.Fatalf("northwind = %+v lts = %+v", northwind.lineDetailBody, lts.lineDetailBody)
	}

	// Latest follows the release date, never the number: v2026.08 beats 2.3.2.
	// Counts: main 9, pci 2, northwind 3, emea 2, de 1, helvetia 0, lts 3 (20 versions in all).
	for _, c := range []struct {
		node   lineageNodeBody
		count  int
		latest string
	}{
		{root, 9, "3.2.0"}, {pci, 2, "1.1.2"}, {northwind, 3, "2.1.0-nw.3"}, {emea, 2, "emea-1.1"}, {de, 1, "emea-1.1-de1"}, {lts, 3, "v2026.08"},
	} {
		if c.node.VersionCount != c.count || c.node.LatestVersion == nil || c.node.LatestVersion.Number != c.latest {
			t.Errorf("%s: count %d latest %+v, want %d / %s", c.node.Branch, c.node.VersionCount, c.node.LatestVersion, c.count, c.latest)
		}
	}
	if lv := lts.LatestVersion; lv.ReleasedAt != "2026-08-12" || lv.ID != p.version["v2026.08"] {
		t.Errorf("lts latest = %+v", lv)
	}

	// A retired line's versions are all still listed, and the flat list agrees with the tree.
	if got := p.listLines(p.elarsen); len(got) != 7 {
		t.Fatalf("lines = %d, want 7", len(got))
	}
	if got := p.list(p.elarsen); len(got) != 20 {
		t.Fatalf("versions = %d, want 20", len(got))
	}
}

// A line references a version (its source) and a version references its line, with no key from
// the line back to the version. Deleting the application must still remove all of it.
func TestDeletingAnApplicationRemovesItsWholeLineage(t *testing.T) {
	t.Parallel()
	p := seedPaymentsCore(t)
	p.admin.Do(http.MethodDelete, "/api/applications/payments-core", map[string]any{"confirmKey": "payments-core"}).ExpectStatus(http.StatusNoContent)
	p.admin.Get("/api/applications/payments-core/lineage").ExpectError(http.StatusNotFound, "", "notFound")
	p.admin.Get("/api/applications/payments-core/lines").ExpectError(http.StatusNotFound, "", "notFound")

	// The key is free again and the new application starts with a mainline only.
	p.env.CodePlatforms.AddRepo(glURL, "payments/payments-core", "1182")
	p.post(nil).ExpectStatus(http.StatusCreated)
	if got := p.listLines(p.admin); len(got) != 1 || !got[0].IsMain {
		t.Fatalf("lines of the new application = %+v", got)
	}
}

func TestTheLineageOfAnApplicationWithoutMaintenanceLinesIsJustTheMainline(t *testing.T) {
	t.Parallel()
	f := newMainlineFixture(t)
	var root lineageNodeBody
	f.elarsen.Get("/api/applications/payments-core/lineage").ExpectStatus(http.StatusOK).JSON(&root)
	if !root.IsMain || root.Children == nil || len(root.Children) != 0 || root.VersionCount != 0 || root.LatestVersion != nil {
		t.Fatalf("root = %+v", root)
	}
	f.elarsen.Get("/api/applications/nope/lineage").ExpectError(http.StatusNotFound, "", "notFound")
	f.env.Client().Get("/api/applications/payments-core/lineage").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}
