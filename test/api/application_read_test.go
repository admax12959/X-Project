package api

import (
	"net/http"
	"testing"
)

type lineBody struct {
	ID     string `json:"id"`
	IsMain bool   `json:"isMain"`
	Name   string `json:"name"`
	Branch string `json:"branch"`
}

func TestCreatingAnApplicationCreatesItsMainlineWithoutAStatus(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	f.post(map[string]any{"mainBranch": "develop"}).ExpectStatus(http.StatusCreated)

	res := f.admin.Get("/api/applications/payments-core/lines").ExpectStatus(http.StatusOK)
	var lines struct {
		Items []lineBody `json:"items"`
	}
	res.JSON(&lines)
	if len(lines.Items) != 1 || !lines.Items[0].IsMain || lines.Items[0].Branch != "develop" || lines.Items[0].ID == "" {
		t.Fatalf("lines = %+v", lines.Items)
	}
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	res.JSON(&raw)
	if _, has := raw.Items[0]["status"]; has {
		t.Fatalf("the mainline has no status, got %v", raw.Items[0])
	}
	f.admin.Get("/api/applications/nope/lines").ExpectError(http.StatusNotFound, "", "notFound")
}

func TestAnyLoggedInUserOpensTheApplicationOverview(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	pe := createUser(t, f.admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})
	f.post(map[string]any{"pes": []string{pe.ID}, "mainBranch": "develop"}).ExpectStatus(http.StatusCreated)

	var a appBody
	f.env.SeedUserClient("bob", "bob-password-1").Get("/api/applications/payments-core").ExpectStatus(http.StatusOK).JSON(&a)
	if a.Key != "payments-core" || a.Name != "Payments Core" || a.Description != "Card and ledger services" ||
		a.MainBranch != "develop" || a.DeployInfo != "" || a.Repo.ProjectID != "1182" || a.Repo.Path != "payments/payments-core" ||
		len(a.Owners) != 1 || a.Owners[0].DisplayName != "Emma Larsen" || len(a.PEs) != 1 || a.PEs[0].DisplayName != "Priya Raman" {
		t.Fatalf("overview = %+v", a)
	}

	f.admin.Get("/api/applications/nope").ExpectError(http.StatusNotFound, "", "notFound")
	f.env.Client().Get("/api/applications/payments-core").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}
