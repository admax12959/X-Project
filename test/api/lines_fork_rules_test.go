package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

func (f versionFixture) listLines(c *apitest.Client) []lineDetailBody {
	var l struct {
		Items []lineDetailBody `json:"items"`
	}
	c.Get(linesPath("payments-core")).ExpectStatus(http.StatusOK).JSON(&l)
	return l.Items
}

func TestForkingNeedsSourceVersionNameBranchAndReason(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	src := f.registerMain(t, "2.1.0", "2025-02-18")
	invalid := http.StatusUnprocessableEntity

	f.fork("", nil).ExpectError(invalid, "sourceVersionId", "required")
	f.fork("abc", nil).ExpectError(invalid, "sourceVersionId", "invalid")
	f.fork(src.ID, map[string]any{"name": "  "}).ExpectError(invalid, "name", "required")
	f.fork(src.ID, map[string]any{"branch": ""}).ExpectError(invalid, "branch", "required")
	f.fork(src.ID, map[string]any{"reason": "\n"}).ExpectError(invalid, "reason", "required")
	for _, bad := range []string{"has space", "a..b", "-lead", "trail/", "x.lock"} {
		f.fork(src.ID, map[string]any{"branch": bad}).ExpectError(invalid, "branch", "invalid")
	}
	f.fork(src.ID, map[string]any{"plannedEnd": "2027-02-30"}).ExpectError(invalid, "plannedEnd", "invalid")
	f.fork(src.ID, map[string]any{"name": strings.Repeat("n", 201)}).ExpectError(invalid, "name", "tooLong")
	f.fork(src.ID, map[string]any{"reason": strings.Repeat("r", 2001)}).ExpectError(invalid, "reason", "tooLong")
	f.fork(src.ID, map[string]any{"note": strings.Repeat("n", 2001)}).ExpectError(invalid, "note", "tooLong")

	// All problems are reported together.
	res := f.elarsen.Post(linesPath("payments-core"), map[string]any{})
	res.ExpectStatus(invalid)
	if got := len(res.Errors()); got != 4 {
		t.Fatalf("empty body: %d errors, want 4: %s", got, res.Body)
	}
	if got := f.listLines(f.elarsen); len(got) != 1 {
		t.Fatalf("rejected requests created lines: %+v", got)
	}

	// Planned end and note are optional; the lines then carry neither.
	var line lineDetailBody
	f.fork(src.ID, nil).ExpectStatus(http.StatusCreated).JSON(&line)
	if line.PlannedEnd != "" || line.Note != "" {
		t.Fatalf("line = %+v", line)
	}
}

func TestOnlyAnOwnerOfTheApplicationCanFork(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	src := f.registerMain(t, "2.1.0", "2025-02-18")
	createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	dbrooks := createUser(t, f.admin, map[string]any{"username": "dbrooks", "displayName": "Daniel Brooks"})
	// dbrooks owns another application, where elarsen is only a PE.
	f.secondApp(t, dbrooks.ID, map[string]any{"pes": []string{f.owner.ID}})

	forbidden := http.StatusForbidden
	bob := plainUserClient(t, f.env, "bob")
	body := newFork(src.ID, nil)
	path := linesPath("payments-core")
	f.admin.Post(path, body).ExpectError(forbidden, "", "forbidden") // an administrator who is not an Owner
	f.login("praman").Post(path, body).ExpectError(forbidden, "", "forbidden")
	f.login("dbrooks").Post(path, body).ExpectError(forbidden, "", "forbidden") // Owner of another application
	bob.Post(path, body).ExpectError(forbidden, "", "forbidden")
	f.admin.Post(path, map[string]any{}).ExpectError(forbidden, "", "forbidden") // permission is checked before the fields
	f.env.Client().Post(path, body).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	f.elarsen.Post(linesPath("nope"), body).ExpectError(http.StatusNotFound, "", "notFound")
	if got := f.listLines(f.admin); len(got) != 1 {
		t.Fatalf("forbidden requests created lines: %+v", got)
	}
}

func TestAPEOfTheApplicationCannotFork(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	pe := createUser(t, f.admin, map[string]any{"username": "pat", "displayName": "Pat PE"})
	f.post(map[string]any{"pes": []string{pe.ID}}).ExpectStatus(http.StatusCreated)
	owner := f.env.Client()
	owner.LoginAs("elarsen", "initial-pw-1").ExpectStatus(http.StatusOK)
	var v versionBody
	var lines struct {
		Items []lineBody `json:"items"`
	}
	owner.Get(linesPath("payments-core")).ExpectStatus(http.StatusOK).JSON(&lines)
	owner.Post(versionsPath("payments-core", lines.Items[0].ID), newVersion(nil)).ExpectStatus(http.StatusCreated).JSON(&v)

	peClient := f.env.Client()
	peClient.LoginAs("pat", "initial-pw-1").ExpectStatus(http.StatusOK)
	peClient.Post(linesPath("payments-core"), newFork(v.ID, nil)).ExpectError(http.StatusForbidden, "", "forbidden")
}

func TestAGitBranchIsUniqueInTheApplicationMainlineBranchIncluded(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	src := f.registerMain(t, "2.1.0", "2025-02-18")
	invalid := http.StatusUnprocessableEntity

	// The mainline's branch (develop) is taken, and the error names the mainline.
	e := f.fork(src.ID, map[string]any{"branch": "develop"}).ExpectError(invalid, "branch", "branchTaken")
	conflict, _ := e.Params["line"].(map[string]any)
	if conflict["id"] != f.mainline.ID || conflict["isMain"] != true || conflict["branch"] != "develop" {
		t.Fatalf("params = %v", e.Params)
	}

	// So is the branch of a maintenance line, and the error names that line.
	var first lineDetailBody
	f.fork(src.ID, nil).ExpectStatus(http.StatusCreated).JSON(&first)
	e = f.fork(src.ID, map[string]any{"name": "Another"}).ExpectError(invalid, "branch", "branchTaken")
	conflict, _ = e.Params["line"].(map[string]any)
	if conflict["id"] != first.ID || conflict["isMain"] != false || conflict["name"] != "Northwind customization" || conflict["branch"] != "cust/northwind" {
		t.Fatalf("params = %v", e.Params)
	}
	if got := f.listLines(f.elarsen); len(got) != 2 {
		t.Fatalf("a rejected fork was stored: %+v", got)
	}

	// Another application may use the same branch name.
	other := f.secondApp(t, f.owner.ID, map[string]any{"mainBranch": "develop"})
	if other.Branch != "develop" {
		t.Fatalf("second application's mainline = %+v", other)
	}
}

func TestRenamingTheMainlineBranchToAnExistingLineBranchIsBranchTaken(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	src := f.registerMain(t, "2.1.0", "2025-02-18")
	var line lineDetailBody
	f.fork(src.ID, nil).ExpectStatus(http.StatusCreated).JSON(&line)

	e := f.elarsen.Do(http.MethodPatch, "/api/applications/payments-core", map[string]any{"mainBranch": "cust/northwind", "name": "Renamed"}).
		ExpectError(http.StatusUnprocessableEntity, "mainBranch", "branchTaken")
	conflict, _ := e.Params["line"].(map[string]any)
	if conflict["id"] != line.ID || conflict["name"] != "Northwind customization" || conflict["branch"] != "cust/northwind" {
		t.Fatalf("params = %v", e.Params)
	}
	// The rejected edit changed nothing, the name included.
	var app appBody
	f.elarsen.Get("/api/applications/payments-core").ExpectStatus(http.StatusOK).JSON(&app)
	if app.MainBranch != "develop" || app.Name != "Payments Core" {
		t.Fatalf("application = %+v", app)
	}

	// A free branch name still works, and the mainline's old branch name is then free for a fork.
	f.elarsen.Do(http.MethodPatch, "/api/applications/payments-core", map[string]any{"mainBranch": "main"}).ExpectStatus(http.StatusOK)
	f.fork(src.ID, map[string]any{"name": "Old develop", "branch": "develop"}).ExpectStatus(http.StatusCreated)
}

func TestTheSourceVersionMustBelongToTheApplication(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	other := f.secondApp(t, f.owner.ID, nil)
	var foreign versionBody
	f.elarsen.Post(versionsPath("rate-engine", other.ID), newVersion(nil)).ExpectStatus(http.StatusCreated).JSON(&foreign)

	f.fork(foreign.ID, nil).ExpectError(http.StatusUnprocessableEntity, "sourceVersionId", "invalid")
	f.fork("999999", nil).ExpectError(http.StatusUnprocessableEntity, "sourceVersionId", "invalid")
	if got := f.listLines(f.elarsen); len(got) != 1 {
		t.Fatalf("lines = %+v", got)
	}
}

func TestAVersionOnARetiredLineCanStillBeForked(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	main := f.registerMain(t, "1.1.0", "2024-06-20")
	var pci lineDetailBody
	f.fork(main.ID, map[string]any{"name": "1.x PCI freeze", "branch": "maint/1.x-pci"}).ExpectStatus(http.StatusCreated).JSON(&pci)
	var fix versionBody
	f.elarsen.Post(versionsPath("payments-core", pci.ID), newVersion(map[string]any{"number": "1.1.1", "releasedAt": "2024-08-02"})).
		ExpectStatus(http.StatusCreated).JSON(&fix)
	pciPath := linesPath("payments-core") + "/" + pci.ID + "/status"
	f.elarsen.Do(http.MethodPut, pciPath, map[string]any{"status": "frozen"}).ExpectStatus(http.StatusOK)
	f.elarsen.Do(http.MethodPut, pciPath, map[string]any{"status": "retired"}).ExpectStatus(http.StatusOK)

	var child lineDetailBody
	f.fork(fix.ID, map[string]any{"name": "Legacy bank edition", "branch": "cust/legacy-bank"}).ExpectStatus(http.StatusCreated).JSON(&child)
	if child.Status != "active" || child.SourceVersion == nil || child.SourceVersion.Number != "1.1.1" ||
		child.SourceVersion.Line.ID != pci.ID || child.SourceVersion.Line.Branch != "maint/1.x-pci" {
		t.Fatalf("child = %+v", child)
	}

	// The list tells the dialog the status of the line that holds the source version.
	byID := map[string]lineDetailBody{}
	for _, l := range f.listLines(f.elarsen) {
		byID[l.ID] = l
	}
	if byID[pci.ID].Status != "retired" || byID[child.ID].Status != "active" {
		t.Fatalf("lines = %+v", byID)
	}
}
