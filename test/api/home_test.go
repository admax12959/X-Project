package api

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
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

func TestApplicationListShowsTheVersionReleasedLastNotTheHighestNumber(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)
	bob := f.env.SeedUserClient("bob", "bob-password-1")
	latest := func() homeAppBody {
		t.Helper()
		var l homeListBody
		bob.Get("/api/applications").ExpectStatus(http.StatusOK).JSON(&l)
		if len(l.Items) != 1 || l.Items[0].LatestVersion == nil {
			t.Fatalf("list = %+v", l.Items)
		}
		return l.Items[0]
	}
	register := func(lineID, number, releasedAt string) {
		t.Helper()
		f.elarsen.Post(versionsPath("payments-core", lineID), newVersion(map[string]any{
			"number": number, "releasedAt": releasedAt, "ref": map[string]any{"kind": "tag", "value": "t-" + number},
		})).ExpectStatus(http.StatusCreated)
	}

	// The number that looks highest was released first.
	register(f.mainline.ID, "10.0.0", "2026-01-01")
	register(f.mainline.ID, "2.0.0", "2026-03-01")
	if got := latest().LatestVersion; got.Number != "2.0.0" || got.ReleasedAt != "2026-03-01" || !got.Line.IsMain || got.Line.Branch != "develop" {
		t.Fatalf("latest = %+v, want mainline 2.0.0", got)
	}

	// A maintenance line may hold the newest release; the answer says which line.
	var fork lineDetailBody
	f.elarsen.Post("/api/applications/payments-core/lines", map[string]any{
		"sourceVersionId": f.versionID(t, "10.0.0"), "name": "LTS", "branch": "release/lts", "reason": "Long-term support",
	}).ExpectStatus(http.StatusCreated).JSON(&fork)
	register(fork.ID, "v2026.08", "2026-08-12")
	register(fork.ID, "11.9.9", "2026-05-01")
	if got := latest(); got.LineCount != 1 || got.LatestVersion.Number != "v2026.08" || got.LatestVersion.Line.Branch != "release/lts" ||
		got.LatestVersion.Line.Name != "LTS" || got.LatestVersion.ID == "" {
		t.Fatalf("latest = %+v / %+v, want v2026.08 on release/lts", got, got.LatestVersion)
	}

	// Same release date: the version registered last wins.
	register(f.mainline.ID, "tie-first", "2026-12-01")
	register(fork.ID, "tie-second", "2026-12-01")
	if got := latest().LatestVersion; got.Number != "tie-second" {
		t.Fatalf("latest = %+v, want tie-second", got)
	}
}

func TestMineFilterKeepsApplicationsWhereTheUserIsOwnerOrPE(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t) // f.owner is elarsen
	brooks := createUser(t, f.admin, map[string]any{"username": "dbrooks", "displayName": "Daniel Brooks"})
	praman := createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	create := func(key string, owners, pes []string) {
		t.Helper()
		createApp(t, f.admin, f.pid, owners, map[string]any{"key": key, "name": key + " name", "pes": pes})
	}
	create("order-service", []string{brooks.ID}, []string{f.owner.ID})              // elarsen is PE
	create("payments-core", []string{f.owner.ID, brooks.ID}, nil)                   // elarsen is Owner
	create("settlement-batch", []string{f.owner.ID}, []string{f.owner.ID})          // both roles: listed once
	create("fraud-engine", []string{praman.ID}, nil)                                // not on it
	create("payments-gateway", []string{praman.ID}, []string{brooks.ID, praman.ID}) // not on it

	keys := func(c *apitest.Client, path string) []string {
		t.Helper()
		var l homeListBody
		c.Get(path).ExpectStatus(http.StatusOK).JSON(&l)
		out := []string{}
		for _, a := range l.Items {
			out = append(out, a.Key)
		}
		return out
	}
	elarsen := f.env.Client()
	elarsen.LoginLocal("elarsen", "initial-pw-1").ExpectStatus(http.StatusOK)
	for _, tc := range []struct {
		why    string
		client *apitest.Client
		path   string
		want   []string
	}{
		{"Owner or PE, each application once", elarsen, "/api/applications?mine=true", []string{"order-service", "payments-core", "settlement-batch"}},
		{"without the filter everything is listed", elarsen, "/api/applications", []string{"fraud-engine", "order-service", "payments-core", "payments-gateway", "settlement-batch"}},
		{"mine=false is the same as no filter", elarsen, "/api/applications?mine=false", []string{"fraud-engine", "order-service", "payments-core", "payments-gateway", "settlement-batch"}},
		{"combined with search (AND)", elarsen, "/api/applications?mine=true&q=brooks", []string{"order-service", "payments-core"}},
		{"search matches nothing of mine", elarsen, "/api/applications?mine=true&q=fraud", []string{}},
		{"an administrator who is not listed owns nothing", f.admin, "/api/applications?mine=true", []string{}},
	} {
		if got := keys(tc.client, tc.path); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %s got %v, want %v", tc.why, tc.path, got, tc.want)
		}
	}
	elarsen.Get("/api/applications?mine=maybe").ExpectError(http.StatusUnprocessableEntity, "mine", "invalid")
}

// versionID finds a version of payments-core by number through the version list.
func (f versionFixture) versionID(t *testing.T, number string) string {
	t.Helper()
	for _, v := range f.list(f.elarsen) {
		if v.Number == number {
			return v.ID
		}
	}
	t.Fatalf("no version %s", number)
	return ""
}
