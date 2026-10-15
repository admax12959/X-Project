package api

import (
	"net/http"
	"testing"
)

// Branch names are case-sensitive (Cust/X and cust/x are two branches), like version
// numbers; the exact name decides what conflicts.
func TestBranchNamesAreCaseSensitive(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	src := f.registerMain(t, "2.1.0", "2025-02-18")
	invalid := http.StatusUnprocessableEntity

	// A different case of the mainline's branch and of a maintenance line's branch is a different branch.
	var upperMain, upperLine lineDetailBody
	f.fork(src.ID, map[string]any{"name": "Upper mainline", "branch": "Develop"}).ExpectStatus(http.StatusCreated).JSON(&upperMain)
	f.fork(src.ID, nil).ExpectStatus(http.StatusCreated)
	f.fork(src.ID, map[string]any{"name": "Upper line", "branch": "Cust/Northwind"}).ExpectStatus(http.StatusCreated).JSON(&upperLine)

	// The exact name still conflicts, and the error names the line with that exact name.
	e := f.fork(src.ID, map[string]any{"name": "Again", "branch": "Cust/Northwind"}).ExpectError(invalid, "branch", "branchTaken")
	conflict, _ := e.Params["line"].(map[string]any)
	if conflict["id"] != upperLine.ID || conflict["branch"] != "Cust/Northwind" {
		t.Fatalf("params = %v, want the line on Cust/Northwind", e.Params)
	}

	// The same holds when the mainline branch is renamed.
	f.elarsen.Do(http.MethodPatch, "/api/applications/payments-core", map[string]any{"mainBranch": "CUST/NORTHWIND"}).ExpectStatus(http.StatusOK)
	e = f.elarsen.Do(http.MethodPatch, "/api/applications/payments-core", map[string]any{"mainBranch": "Develop"}).
		ExpectError(invalid, "mainBranch", "branchTaken")
	conflict, _ = e.Params["line"].(map[string]any)
	if conflict["id"] != upperMain.ID {
		t.Fatalf("params = %v, want the line on Develop", e.Params)
	}
}
