package api

import (
	"net/http"
	"testing"
)

// The home page (#12): the application list with its latest version and maintenance
// line count, the "mine" filter, and the recent-activity list.

type homeVersionBody struct {
	ID         string   `json:"id"`
	Number     string   `json:"number"`
	ReleasedAt string   `json:"releasedAt"`
	Line       lineBody `json:"line"`
}

type homeAppBody struct {
	Key           string           `json:"key"`
	LineCount     int              `json:"lineCount"`
	LatestVersion *homeVersionBody `json:"latestVersion"`
}

type homeListBody struct {
	Items []homeAppBody `json:"items"`
}

func (p *paymentsCore) homeApp(t *testing.T, path string) homeAppBody {
	t.Helper()
	var l homeListBody
	p.env.SeedUserClient("bob", "bob-password-1").Get(path).ExpectStatus(http.StatusOK).JSON(&l)
	for _, a := range l.Items {
		if a.Key == "payments-core" {
			return a
		}
	}
	t.Fatalf("payments-core is not in %+v", l.Items)
	return homeAppBody{}
}

func TestApplicationListShowsTheNumberOfMaintenanceLinesInEveryStatus(t *testing.T) {
	t.Parallel()
	p := seedPaymentsCore(t) // six maintenance lines: Active, Frozen and Retired
	if got := p.homeApp(t, "/api/applications").LineCount; got != 6 {
		t.Fatalf("lineCount = %d, want 6 (the mainline is not counted, Retired and Frozen are)", got)
	}
}

func TestApplicationWithoutForksHasNoMaintenanceLinesAndNoLatestVersion(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	f.post(nil).ExpectStatus(http.StatusCreated)
	var l homeListBody
	f.env.SeedUserClient("bob", "bob-password-1").Get("/api/applications").ExpectStatus(http.StatusOK).JSON(&l)
	if len(l.Items) != 1 || l.Items[0].LineCount != 0 || l.Items[0].LatestVersion != nil {
		t.Fatalf("list = %+v", l.Items)
	}
}
