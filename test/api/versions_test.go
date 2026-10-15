package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

type versionBody struct {
	ID     string   `json:"id"`
	LineID string   `json:"lineId"`
	Line   lineBody `json:"line"`
	Number string   `json:"number"`
	Ref    struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	} `json:"ref"`
	ReleasedAt string `json:"releasedAt"`
	Note       string `json:"note"`
	CreatedAt  string `json:"createdAt"`
}

type versionListBody struct {
	Items []versionBody `json:"items"`
}

// versionFixture is an application "payments-core" (mainline branch develop) whose
// Owner elarsen is logged in as elarsen.
type versionFixture struct {
	appFixture
	elarsen  *apitest.Client
	mainline lineBody
}

func newVersionFixture(t *testing.T) versionFixture {
	t.Helper()
	f := newAppFixture(t)
	f.post(map[string]any{"mainBranch": "develop"}).ExpectStatus(http.StatusCreated)
	owner := f.env.Client()
	owner.LoginLocal("elarsen", "initial-pw-1").ExpectStatus(http.StatusOK)
	var lines struct {
		Items []lineBody `json:"items"`
	}
	owner.Get("/api/applications/payments-core/lines").ExpectStatus(http.StatusOK).JSON(&lines)
	return versionFixture{appFixture: f, elarsen: owner, mainline: lines.Items[0]}
}

func versionsPath(appKey, lineID string) string {
	return "/api/applications/" + appKey + "/lines/" + lineID + "/versions"
}

// newVersion is a valid register-version request; tests override single fields.
func newVersion(over map[string]any) map[string]any {
	m := map[string]any{
		"number":     "1.0.0",
		"ref":        map[string]any{"kind": "tag", "value": "v1.0.0"},
		"releasedAt": "2024-03-12",
		"note":       "First general release",
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

func (f versionFixture) register(over map[string]any) *apitest.Response {
	return f.elarsen.Post(versionsPath("payments-core", f.mainline.ID), newVersion(over))
}

func (f versionFixture) list(c *apitest.Client) []versionBody {
	var l versionListBody
	c.Get("/api/applications/payments-core/versions").ExpectStatus(http.StatusOK).JSON(&l)
	return l.Items
}

func TestOwnerRegistersAVersionOnTheMainlineAndEveryoneSeesItInTheVersionList(t *testing.T) {
	t.Parallel()
	f := newVersionFixture(t)

	var v versionBody
	f.register(nil).ExpectStatus(http.StatusCreated).JSON(&v)
	if v.ID == "" || v.Number != "1.0.0" || v.Ref.Kind != "tag" || v.Ref.Value != "v1.0.0" ||
		v.ReleasedAt != "2024-03-12" || v.Note != "First general release" || v.CreatedAt == "" ||
		v.LineID != f.mainline.ID || !v.Line.IsMain || v.Line.Branch != "develop" {
		t.Fatalf("registered = %+v", v)
	}

	items := f.list(plainUserClient(t, f.env, "bob"))
	if len(items) != 1 || items[0] != v {
		t.Fatalf("list = %+v, want [%+v]", items, v)
	}
}
