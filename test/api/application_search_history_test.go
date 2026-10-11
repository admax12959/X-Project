package api

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestApplicationListSearch(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	f.env.CodePlatforms.AddRepo(glURL, "platform/order-service", "552")
	f.env.CodePlatforms.AddRepo(glURL, "misc/rates", "7")
	brooks := createUser(t, f.admin, map[string]any{"username": "dbrooks", "displayName": "Daniel Brooks"})
	f.post(nil).ExpectStatus(http.StatusCreated)
	other := func(key, name, path string) {
		t.Helper()
		f.admin.Post("/api/applications", newApp(f.pid, []string{brooks.ID}, map[string]any{
			"key": key, "name": name, "repo": map[string]any{"platformId": f.pid, "path": path},
		})).ExpectStatus(http.StatusCreated)
	}
	other("order-service", "Order Service", "platform/order-service")
	other("rate-a", "Rate 100% done", "misc/rates")
	other("rate-b", "Rate 1000 done a_b", "misc/rates")
	other("rate-c", "Rate 1000 done axb", "misc/rates")

	reader := f.env.SeedUserClient("reader", "reader-password-1")
	search := func(q string) []string {
		t.Helper()
		var list appListBody
		reader.Get("/api/applications?q=" + url.QueryEscape(q)).ExpectStatus(http.StatusOK).JSON(&list)
		keys := []string{}
		for _, it := range list.Items {
			keys = append(keys, it.Key)
		}
		return keys
	}
	for _, tc := range []struct {
		why, q string
		want   []string
	}{
		{"empty query lists all", "", []string{"order-service", "payments-core", "rate-a", "rate-b", "rate-c"}},
		{"blank query lists all", "   ", []string{"order-service", "payments-core", "rate-a", "rate-b", "rate-c"}},
		{"by key", "order-ser", []string{"order-service"}},
		{"by name, case-insensitive", "PAYMENTS CORE", []string{"payments-core"}},
		{"by repository path", "platform/order", []string{"order-service"}},
		{"by Owner display name", "brooks", []string{"order-service", "rate-a", "rate-b", "rate-c"}},
		{"by Owner username", "elarsen", []string{"payments-core"}},
		{"percent is literal", "100%", []string{"rate-a"}},
		{"underscore is literal", "a_b", []string{"rate-b"}},
		{"no match", "zzz", []string{}},
	} {
		if got := search(tc.q); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: q=%q got %v, want %v", tc.why, tc.q, got, tc.want)
		}
	}
}

type historyEventBody struct {
	ID    string `json:"id"`
	At    string `json:"at"`
	Actor struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"actor"`
	Type   string `json:"type"`
	Params struct {
		Owners []struct {
			UserID      string `json:"userId"`
			Username    string `json:"username"`
			DisplayName string `json:"displayName"`
		} `json:"owners"`
		PEs []struct {
			UserID      string `json:"userId"`
			Username    string `json:"username"`
			DisplayName string `json:"displayName"`
		} `json:"pes"`
	} `json:"params"`
}

type historyBody struct {
	Items []historyEventBody `json:"items"`
}

func TestChangeHistoryStartsWithTheCreationEventAndKeepsUserInputVerbatim(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	odd := createUser(t, f.admin, map[string]any{"username": "zoe", "displayName": `Zoë <b>O'Neil</b> 王 &amp;`})
	f.post(map[string]any{"owners": []string{f.owner.ID, odd.ID}}).ExpectStatus(http.StatusCreated)

	var h historyBody
	f.env.SeedUserClient("reader", "reader-password-1").Get("/api/applications/payments-core/history").ExpectStatus(http.StatusOK).JSON(&h)
	if len(h.Items) != 1 {
		t.Fatalf("history = %+v", h.Items)
	}
	e := h.Items[0]
	if e.Type != "appCreated" || e.ID == "" || e.At == "" || e.Actor.Username != "admin" || e.Actor.DisplayName != "Initial Admin" {
		t.Fatalf("event = %+v", e)
	}
	if len(e.Params.Owners) != 2 || e.Params.Owners[0].DisplayName != "Emma Larsen" || e.Params.Owners[1].DisplayName != `Zoë <b>O'Neil</b> 王 &amp;` ||
		e.Params.Owners[0].UserID != f.owner.ID {
		t.Fatalf("owners = %+v", e.Params.Owners)
	}

	// Names are resolved when the history is read, so a profile change shows up.
	f.admin.Do(http.MethodPatch, "/api/users/"+f.owner.ID, map[string]any{"displayName": "Emma Larsen-Berg"}).ExpectStatus(http.StatusOK)
	f.admin.Get("/api/applications/payments-core/history").ExpectStatus(http.StatusOK).JSON(&h)
	if h.Items[0].Params.Owners[0].DisplayName != "Emma Larsen-Berg" {
		t.Fatalf("owners = %+v", h.Items[0].Params.Owners)
	}
}

func TestCreationEventRecordsThePEsAsWellAsTheOwners(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	pe1 := createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	pe2 := createUser(t, f.admin, map[string]any{"username": "bob", "displayName": "Bob Plain"})
	f.post(map[string]any{"pes": []string{pe1.ID, pe2.ID}}).ExpectStatus(http.StatusCreated)

	var h historyBody
	f.admin.Get("/api/applications/payments-core/history").ExpectStatus(http.StatusOK).JSON(&h)
	if len(h.Items) != 1 || h.Items[0].Type != "appCreated" {
		t.Fatalf("history = %+v", h.Items)
	}
	pes := h.Items[0].Params.PEs
	if len(pes) != 2 || pes[0].UserID != pe1.ID || pes[0].DisplayName != "Priya Raman" || pes[1].UserID != pe2.ID || pes[1].Username != "bob" {
		t.Fatalf("pes = %+v, want Priya Raman then Bob Plain", pes)
	}
	if owners := h.Items[0].Params.Owners; len(owners) != 1 || owners[0].UserID != f.owner.ID {
		t.Fatalf("owners = %+v", owners)
	}
}

func TestChangeHistoryCannotBeChangedThroughTheAPI(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	f.post(nil).ExpectStatus(http.StatusCreated)
	var h historyBody
	f.admin.Get("/api/applications/payments-core/history").ExpectStatus(http.StatusOK).JSON(&h)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, path := range []string{"/api/applications/payments-core/history", "/api/applications/payments-core/history/" + h.Items[0].ID} {
			if res := f.admin.Do(method, path, map[string]any{"type": "x"}); res.Status < 400 {
				t.Fatalf("%s %s = %d, want an error", method, path, res.Status)
			}
		}
	}
	var after historyBody
	f.admin.Get("/api/applications/payments-core/history").ExpectStatus(http.StatusOK).JSON(&after)
	if !reflect.DeepEqual(after, h) {
		t.Fatalf("history changed: %+v -> %+v", h, after)
	}
	f.admin.Get("/api/applications/nope/history").ExpectError(http.StatusNotFound, "", "notFound")
}
