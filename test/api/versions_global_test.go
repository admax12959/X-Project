package api

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

type globalVersionBody struct {
	versionBody
	Application struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"application"`
}

type globalVersionListBody struct {
	Items    []globalVersionBody `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
}

// globalFixture is two applications, payments-core (mainline develop) and
// order-service (mainline main), both owned by elarsen who is logged in as elarsen.
type globalFixture struct {
	env     *apitest.Env
	elarsen *apitest.Client
	lines   map[string]string // application key -> mainline ID
}

func newGlobalFixture(t *testing.T) globalFixture {
	t.Helper()
	env := apitest.New(t)
	admin := env.AdminClient()
	pid := registerPlatform(t, admin, "gitlab", glURL)
	env.CodePlatforms.AddRepo(glURL, "payments/payments-core", "1182")
	env.CodePlatforms.AddRepo(glURL, "platform/order-service", "552")
	owner := createUser(t, admin, map[string]any{"username": "elarsen", "displayName": "Emma Larsen"})
	createApp(t, admin, pid, []string{owner.ID}, map[string]any{"mainBranch": "develop"})
	createApp(t, admin, pid, []string{owner.ID}, map[string]any{
		"key": "order-service", "name": "Order Service",
		"repo": map[string]any{"platformId": pid, "path": "platform/order-service"},
	})
	elarsen := env.Client()
	elarsen.LoginLocal("elarsen", "initial-pw-1").ExpectStatus(http.StatusOK)
	f := globalFixture{env: env, elarsen: elarsen, lines: map[string]string{}}
	for _, key := range []string{"payments-core", "order-service"} {
		var lines struct {
			Items []lineBody `json:"items"`
		}
		elarsen.Get("/api/applications/" + key + "/lines").ExpectStatus(http.StatusOK).JSON(&lines)
		f.lines[key] = lines.Items[0].ID
	}
	return f
}

// register adds a mainline version to an application, released on the given date.
func (f globalFixture) register(t *testing.T, appKey, number, releasedAt string) {
	t.Helper()
	f.elarsen.Post(versionsPath(appKey, f.lines[appKey]), newVersion(map[string]any{
		"number": number, "releasedAt": releasedAt, "ref": map[string]any{"kind": "tag", "value": "v" + number},
	})).ExpectStatus(http.StatusCreated)
}

func (f globalFixture) list(t *testing.T, c *apitest.Client, query string) globalVersionListBody {
	t.Helper()
	var l globalVersionListBody
	c.Get("/api/versions" + query).ExpectStatus(http.StatusOK).JSON(&l)
	return l
}

func numbersOf(items []globalVersionBody) []string {
	out := make([]string, 0, len(items))
	for _, v := range items {
		out = append(out, v.Number)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGlobalVersionListShowsEveryApplicationsVersionsNewestReleaseFirst(t *testing.T) {
	t.Parallel()
	f := newGlobalFixture(t)
	// Registration order differs from release order, and string order differs from
	// date order ("10.0" < "9.0" as strings, but 10.0 is newer). "x1" and "x2" share a
	// release date: the one registered later comes first.
	f.register(t, "payments-core", "9.0", "2026-01-01")
	f.register(t, "order-service", "x1", "2026-05-01")
	f.register(t, "payments-core", "10.0", "2026-07-19")
	f.register(t, "order-service", "x2", "2026-05-01")
	f.register(t, "order-service", "v3.4", "2026-02-10")

	l := f.list(t, f.env.SeedUserClient("bob", "bob-password-1"), "")
	if want := []string{"10.0", "x2", "x1", "v3.4", "9.0"}; !equalStrings(numbersOf(l.Items), want) {
		t.Fatalf("order = %v, want %v", numbersOf(l.Items), want)
	}
	if l.Total != 5 {
		t.Fatalf("total = %d, want 5", l.Total)
	}
	first := l.Items[0]
	if first.Application.Key != "payments-core" || first.Application.Name != "Payments Core" ||
		first.Line.Branch != "develop" || !first.Line.IsMain || first.LineID != f.lines["payments-core"] ||
		first.Ref.Kind != "tag" || first.Ref.Value != "v10.0" || first.ReleasedAt != "2026-07-19" || first.Note == "" {
		t.Fatalf("first = %+v", first)
	}
	if got := l.Items[1]; got.Application.Key != "order-service" || got.Line.Branch != "main" {
		t.Fatalf("second = %+v", got)
	}
}

func TestGlobalVersionListCanBeFilteredByApplication(t *testing.T) {
	t.Parallel()
	f := newGlobalFixture(t)
	f.register(t, "payments-core", "1.0", "2026-01-01")
	f.register(t, "order-service", "3.0", "2026-02-01")
	f.register(t, "payments-core", "2.0", "2026-03-01")

	l := f.list(t, f.elarsen, "?app=payments-core")
	if want := []string{"2.0", "1.0"}; !equalStrings(numbersOf(l.Items), want) || l.Total != 2 {
		t.Fatalf("payments-core = %v (total %d), want %v", numbersOf(l.Items), l.Total, want)
	}
	if l := f.list(t, f.elarsen, "?app=no-such-app"); len(l.Items) != 0 || l.Total != 0 {
		t.Fatalf("unknown application = %+v, want empty", l)
	}
}

func TestGlobalVersionListSearchesVersionNumbersIgnoringCaseAndTakingWildcardsLiterally(t *testing.T) {
	t.Parallel()
	f := newGlobalFixture(t)
	f.register(t, "payments-core", "V2.1.0", "2026-01-01")
	f.register(t, "order-service", "2.1.0-nw.1", "2026-02-01")
	f.register(t, "payments-core", "3.0.0", "2026-03-01")
	f.register(t, "order-service", "rc_1", "2026-04-01")
	f.register(t, "order-service", "rcx1", "2026-05-01")

	cases := []struct {
		query string
		want  []string
	}{
		{"?q=v2.1", []string{"V2.1.0"}},
		{"?q=2.1", []string{"2.1.0-nw.1", "V2.1.0"}},
		{"?q=nw", []string{"2.1.0-nw.1"}},
		{"?q=rc_1", []string{"rc_1"}},
		{"?q=%25", nil},
		{"?q=%20nw%20", []string{"2.1.0-nw.1"}},
		{"?q=2.1&app=payments-core", []string{"V2.1.0"}},
		{"?q=", []string{"rcx1", "rc_1", "3.0.0", "2.1.0-nw.1", "V2.1.0"}},
	}
	for _, c := range cases {
		l := f.list(t, f.elarsen, c.query)
		if !equalStrings(numbersOf(l.Items), c.want) && !(len(c.want) == 0 && len(l.Items) == 0) {
			t.Errorf("%s = %v, want %v", c.query, numbersOf(l.Items), c.want)
		}
		if l.Total != len(c.want) {
			t.Errorf("%s total = %d, want %d", c.query, l.Total, len(c.want))
		}
	}
}

func TestGlobalVersionListIsPaginatedAndReportsTheTotal(t *testing.T) {
	t.Parallel()
	f := newGlobalFixture(t)
	for i, n := range []string{"1.0", "2.0", "3.0", "4.0", "5.0"} {
		f.register(t, "payments-core", n, "2026-0"+strconv.Itoa(i+1)+"-01")
	}

	pages := []struct {
		query string
		want  []string
		page  int
	}{
		{"?pageSize=2", []string{"5.0", "4.0"}, 1},
		{"?pageSize=2&page=2", []string{"3.0", "2.0"}, 2},
		{"?pageSize=2&page=3", []string{"1.0"}, 3},
		{"?pageSize=2&page=4", nil, 4},
	}
	for _, p := range pages {
		l := f.list(t, f.elarsen, p.query)
		if (len(l.Items) != 0 || len(p.want) != 0) && !equalStrings(numbersOf(l.Items), p.want) {
			t.Errorf("%s = %v, want %v", p.query, numbersOf(l.Items), p.want)
		}
		if l.Total != 5 || l.Page != p.page || l.PageSize != 2 {
			t.Errorf("%s total/page/pageSize = %d/%d/%d", p.query, l.Total, l.Page, l.PageSize)
		}
	}
	// Defaults: page 1, 20 per page.
	if l := f.list(t, f.elarsen, ""); l.Page != 1 || l.PageSize != 20 || len(l.Items) != 5 {
		t.Fatalf("defaults = page %d size %d items %d", l.Page, l.PageSize, len(l.Items))
	}
}

func TestGlobalVersionListRejectsBadPagingParameters(t *testing.T) {
	t.Parallel()
	f := newGlobalFixture(t)
	for _, c := range []struct{ query, field string }{
		{"?page=0", "page"}, {"?page=abc", "page"}, {"?page=-1", "page"},
		{"?pageSize=0", "pageSize"}, {"?pageSize=x", "pageSize"}, {"?pageSize=101", "pageSize"},
	} {
		f.elarsen.Get("/api/versions"+c.query).ExpectError(http.StatusUnprocessableEntity, c.field, "invalid")
	}
}

func TestGlobalVersionListRequiresASession(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Client().Get("/api/versions").ExpectStatus(http.StatusUnauthorized)
}
