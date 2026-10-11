package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

type memberBody struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type appRepoBody struct {
	PlatformID string `json:"platformId"`
	Kind       string `json:"kind"`
	BaseURL    string `json:"baseUrl"`
	Path       string `json:"path"`
	ProjectID  string `json:"projectId"`
}

// appBody mirrors the application detail (POST /api/applications, GET /api/applications/{key}).
type appBody struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Repo        appRepoBody  `json:"repo"`
	MainBranch  string       `json:"mainBranch"`
	Owners      []memberBody `json:"owners"`
	PEs         []memberBody `json:"pes"`
	DeployInfo  string       `json:"deployInfo"`
	CreatedAt   string       `json:"createdAt"`
}

type appListItem struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Owners      []memberBody `json:"owners"`
	Repo        appRepoBody  `json:"repo"`
}

type appListBody struct {
	Items []appListItem `json:"items"`
}

// newApp is a valid create-application request for platform pid; tests override single fields.
func newApp(pid string, owners []string, over map[string]any) map[string]any {
	m := map[string]any{
		"key":         "payments-core",
		"name":        "Payments Core",
		"description": "Card and ledger services",
		"repo":        map[string]any{"platformId": pid, "path": "payments/payments-core"},
		"owners":      owners,
		"pes":         []string{},
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

// createApp creates an application as admin and returns it.
func createApp(t *testing.T, admin *apitest.Client, pid string, owners []string, over map[string]any) appBody {
	t.Helper()
	var a appBody
	admin.Post("/api/applications", newApp(pid, owners, over)).ExpectStatus(http.StatusCreated).JSON(&a)
	return a
}

func TestAdminCreatesAnApplicationAndEveryoneSeesItInTheList(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	pid := registerPlatform(t, admin, "gitlab", glURL)
	env.CodePlatforms.AddRepo(glURL, "payments/payments-core", "1182")
	elarsen := createUser(t, admin, map[string]any{"username": "elarsen", "displayName": "Emma Larsen"})
	praman := createUser(t, admin, map[string]any{"username": "praman", "displayName": "Priya Raman"})

	var created appBody
	admin.Post("/api/applications", newApp(pid, []string{elarsen.ID}, map[string]any{"pes": []string{praman.ID}})).
		ExpectStatus(http.StatusCreated).JSON(&created)
	if created.Key != "payments-core" || created.Name != "Payments Core" || created.Description != "Card and ledger services" ||
		created.MainBranch != "main" || created.DeployInfo != "" || created.CreatedAt == "" {
		t.Fatalf("created = %+v", created)
	}
	if created.Repo != (appRepoBody{PlatformID: pid, Kind: "gitlab", BaseURL: glURL, Path: "payments/payments-core", ProjectID: "1182"}) {
		t.Fatalf("repo = %+v", created.Repo)
	}
	if len(created.Owners) != 1 || created.Owners[0].ID != elarsen.ID || created.Owners[0].DisplayName != "Emma Larsen" ||
		len(created.PEs) != 1 || created.PEs[0].Username != "praman" {
		t.Fatalf("owners %+v pes %+v", created.Owners, created.PEs)
	}

	// Any logged-in user sees it in the list.
	var list appListBody
	env.SeedUserClient("bob", "bob-password-1").Get("/api/applications").ExpectStatus(http.StatusOK).JSON(&list)
	if len(list.Items) != 1 {
		t.Fatalf("list = %+v", list.Items)
	}
	got := list.Items[0]
	if got.Key != "payments-core" || got.Name != "Payments Core" || got.Repo.Kind != "gitlab" || got.Repo.Path != "payments/payments-core" ||
		len(got.Owners) != 1 || got.Owners[0].DisplayName != "Emma Larsen" {
		t.Fatalf("list item = %+v", got)
	}
}
