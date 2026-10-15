package api

import (
	"net/http"
	"slices"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// Reading the audit log: who may, filters, order, paging.

func TestOnlyAdministratorsMayReadTheAuditLog(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)

	plainUserClient(t, env, "jdoe").Get("/api/audit-log").ExpectStatus(http.StatusForbidden)
	env.Client().Get("/api/audit-log").ExpectStatus(http.StatusUnauthorized)
}

func TestTheAuditLogCannotBeChanged(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	createUser(t, admin, map[string]any{"username": "jdoe"})
	before := auditLog(t, admin).Total

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, path := range []string{"/api/audit-log", "/api/audit-log/1"} {
			res := admin.Do(method, path, map[string]any{"action": "forged"})
			if res.Status < 400 || res.Status >= 500 {
				t.Errorf("%s %s = %d, want a 4xx", method, path, res.Status)
			}
		}
	}
	if got := auditLog(t, admin).Total; got != before {
		t.Fatalf("audit entries = %d after attempts, want %d", got, before)
	}
}

// auditHistory makes a log of four entries: the administrator creates alice and bob, then
// changes both login emails.
func auditHistory(t *testing.T) (admin *apitest.Client, alice, bob userBody) {
	t.Helper()
	env := apitest.New(t)
	admin = env.AdminClient()
	alice = createUserRaw(t, admin, map[string]any{"username": "alice"})
	bob = createUserRaw(t, admin, map[string]any{"username": "bob"})
	patchUser(admin, alice.ID, map[string]any{"loginEmail": "alice.new@example.com"}).ExpectStatus(http.StatusOK)
	patchUser(admin, bob.ID, map[string]any{"loginEmail": "bob.new@example.com"}).ExpectStatus(http.StatusOK)
	return admin, alice, bob
}

func TestEntriesAreListedNewestFirstAndFilteredByActionObjectAndOperator(t *testing.T) {
	t.Parallel()
	admin, alice, bob := auditHistory(t)
	adminUser := findUser(t, listUsers(t, admin), apitest.AdminUsername)

	all := auditLog(t, admin)
	if all.Total != 4 || len(all.Items) != 4 {
		t.Fatalf("total = %d, items = %d, want 4", all.Total, len(all.Items))
	}
	var got []string
	for _, e := range all.Items {
		got = append(got, e.Action+":"+e.Target.Username)
	}
	want := []string{"loginEmailChanged:bob", "loginEmailChanged:alice", "userCreated:bob", "userCreated:alice"}
	if !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}

	if p := auditLog(t, admin, "action", "userCreated"); p.Total != 2 {
		t.Errorf("action filter: total = %d, want 2", p.Total)
	}
	p := auditLog(t, admin, "targetType", "user", "targetId", alice.ID)
	if p.Total != 2 || p.Items[0].Target.Username != "alice" {
		t.Errorf("object filter: %+v, want alice's two entries", p)
	}
	if p := auditLog(t, admin, "targetType", "user", "targetId", alice.ID, "action", "userCreated"); p.Total != 1 {
		t.Errorf("object and action filters combine: total = %d, want 1", p.Total)
	}
	if p := auditLog(t, admin, "actor", adminUser.ID); p.Total != 4 {
		t.Errorf("operator filter (admin): total = %d, want 4", p.Total)
	}
	if p := auditLog(t, admin, "actor", bob.ID); p.Total != 0 {
		t.Errorf("operator filter (bob): total = %d, want 0", p.Total)
	}
	if p := auditLog(t, admin, "actor", "cli"); p.Total != 0 {
		t.Errorf("operator filter (cli): total = %d, want 0", p.Total)
	}
	if p := auditLog(t, admin, "targetType", "application"); p.Total != 0 {
		t.Errorf("object type filter: total = %d, want 0", p.Total)
	}
}

func TestEntriesAreFilteredByTimeRangeWithInclusiveBounds(t *testing.T) {
	t.Parallel()
	admin, _, _ := auditHistory(t)
	all := auditLog(t, admin).Items // newest first
	newest, oldest := all[0], all[3]

	if p := auditLog(t, admin, "from", newest.At); p.Total < 1 || p.Items[0].ID != newest.ID {
		t.Errorf("from = newest: %+v, want at least the newest entry", p.Items)
	}
	if p := auditLog(t, admin, "to", oldest.At); p.Total < 1 || p.Items[len(p.Items)-1].ID != oldest.ID {
		t.Errorf("to = oldest: %+v, want at least the oldest entry", p.Items)
	}
	if p := auditLog(t, admin, "from", oldest.At, "to", newest.At); p.Total != 4 {
		t.Errorf("from oldest to newest: total = %d, want all 4", p.Total)
	}
	if p := auditLog(t, admin, "from", "2999-01-01T00:00:00Z"); p.Total != 0 {
		t.Errorf("from the far future: total = %d, want 0", p.Total)
	}
	if p := auditLog(t, admin, "to", "2000-01-01T00:00:00Z"); p.Total != 0 {
		t.Errorf("to the far past: total = %d, want 0", p.Total)
	}
}

func TestAuditLogIsPaged(t *testing.T) {
	t.Parallel()
	admin, _, _ := auditHistory(t)
	all := auditLog(t, admin).Items

	first := auditLog(t, admin, "pageSize", "3")
	if first.Total != 4 || first.Page != 1 || first.PageSize != 3 || len(first.Items) != 3 {
		t.Fatalf("page 1 = total %d page %d size %d items %d, want 4/1/3/3", first.Total, first.Page, first.PageSize, len(first.Items))
	}
	second := auditLog(t, admin, "pageSize", "3", "page", "2")
	if second.Total != 4 || second.Page != 2 || len(second.Items) != 1 || second.Items[0].ID != all[3].ID {
		t.Fatalf("page 2 = %+v, want only the oldest entry", second)
	}
	if past := auditLog(t, admin, "pageSize", "3", "page", "3"); len(past.Items) != 0 || past.Total != 4 {
		t.Fatalf("page past the end = %+v, want empty with total 4", past)
	}
	if def := auditLog(t, admin); def.Page != 1 || def.PageSize != 20 {
		t.Fatalf("defaults = page %d size %d, want 1 and 20", def.Page, def.PageSize)
	}
}

func TestMalformedAuditQueriesAreRejected(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	for _, c := range []struct{ key, value string }{
		{"actor", "nobody"}, {"actor", "-3"}, {"targetType", "planet"},
		{"from", "yesterday"}, {"to", "2026-13-45"}, {"page", "0"}, {"pageSize", "101"}, {"pageSize", "x"},
	} {
		admin.Get(auditPath(c.key, c.value)).ExpectError(http.StatusUnprocessableEntity, c.key, "invalid")
	}
}
