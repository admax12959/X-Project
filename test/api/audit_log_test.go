package api

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// auditEntryBody mirrors one item of GET /api/audit-log.
type auditEntryBody struct {
	ID     string `json:"id"`
	At     string `json:"at"`
	Action string `json:"action"`
	Actor  struct {
		Kind        string `json:"kind"`
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"actor"`
	Target struct {
		Type        string `json:"type"`
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"target"`
	Params map[string]any `json:"params"`
}

type auditPageBody struct {
	Items    []auditEntryBody `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

// auditLog reads the audit log with the given query parameters (key, value, ...).
func auditLog(t *testing.T, c *apitest.Client, query ...string) auditPageBody {
	t.Helper()
	var p auditPageBody
	c.Get(auditPath(query...)).ExpectStatus(http.StatusOK).JSON(&p)
	return p
}

func auditPath(query ...string) string {
	q := url.Values{}
	for i := 0; i+1 < len(query); i += 2 {
		q.Set(query[i], query[i+1])
	}
	if len(q) == 0 {
		return "/api/audit-log"
	}
	return "/api/audit-log?" + q.Encode()
}

// auditOf returns the entries of one action, newest first.
func auditOf(t *testing.T, c *apitest.Client, action string) []auditEntryBody {
	t.Helper()
	return auditLog(t, c, "action", action, "pageSize", "100").Items
}

func TestCreatingAUserIsRecordedWithTheOperatorTheUserAndTheLoginEmail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	adminUser := findUser(t, listUsers(t, admin), apitest.AdminUsername)

	jane := createUserRaw(t, admin, map[string]any{"username": "jdoe", "displayName": "Jane Doe"})

	got := auditOf(t, admin, "userCreated")
	if len(got) != 1 {
		t.Fatalf("userCreated entries = %+v, want exactly one", got)
	}
	e := got[0]
	if e.Actor.Kind != "user" || e.Actor.ID != adminUser.ID || e.Actor.Username != apitest.AdminUsername {
		t.Errorf("actor = %+v, want the administrator", e.Actor)
	}
	if e.Target.Type != "user" || e.Target.ID != jane.ID || e.Target.Username != "jdoe" || e.Target.DisplayName != "Jane Doe" {
		t.Errorf("target = %+v, want user jdoe", e.Target)
	}
	if e.Params["loginEmail"] != apitest.Email("jdoe") {
		t.Errorf("params = %+v, want loginEmail %s", e.Params, apitest.Email("jdoe"))
	}
	if e.At == "" || e.ID == "" {
		t.Errorf("entry has no id/at: %+v", e)
	}
}
