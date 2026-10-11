package api

import (
	"net/http"
	"sync"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// A conflict error must always name the line that holds the number or the branch: the UI
// puts it in the sentence. These races make the losers hit the unique key while the
// winner's row is still being committed.

// race runs n copies of do at the same moment and returns their responses.
func race(n int, do func() *apitest.Response) []*apitest.Response {
	out := make([]*apitest.Response, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			out[i] = do()
		}()
	}
	close(start)
	wg.Wait()
	return out
}

// expectOneWinnerAndNamedLosers checks one 201 and, for every other response, a 422 with
// the given code whose params name a line (the one wantLineID, when given).
func expectOneWinnerAndNamedLosers(t *testing.T, rs []*apitest.Response, code, wantLineID string) {
	t.Helper()
	created := 0
	for _, r := range rs {
		if r.Status == http.StatusCreated {
			created++
			continue
		}
		errs := r.Errors()
		if r.Status != http.StatusUnprocessableEntity || len(errs) != 1 || errs[0].Code != code {
			t.Fatalf("loser answered %d %s, want 422 %s", r.Status, r.Body, code)
		}
		line, _ := errs[0].Params["line"].(map[string]any)
		if line == nil || line["id"] == nil || (wantLineID != "" && line["id"] != wantLineID) {
			t.Fatalf("conflict params = %v, want the line %s", errs[0].Params, wantLineID)
		}
	}
	if created != 1 {
		t.Fatalf("%d requests succeeded, want exactly 1", created)
	}
}

func TestConcurrentRegistrationsOfOneNumberAllNameTheLineThatHoldsIt(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	rs := race(8, func() *apitest.Response { return f.register(map[string]any{"number": "9.9.9"}) })
	expectOneWinnerAndNamedLosers(t, rs, "numberTaken", f.mainline.ID)
}

func TestConcurrentForksOfOneBranchAllNameTheLineThatHoldsIt(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	src := f.registerMain(t, "1.0.0", "2024-03-12")
	rs := race(8, func() *apitest.Response { return f.fork(src.ID, nil) })
	expectOneWinnerAndNamedLosers(t, rs, "branchTaken", "")
}
