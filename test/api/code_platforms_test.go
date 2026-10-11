package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

type platformBody struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	BaseURL   string `json:"baseUrl"`
	TokenTail string `json:"tokenTail"`
	CreatedAt string `json:"createdAt"`
}

type platformList struct {
	Items []platformBody `json:"items"`
}

func TestAdminRegistersACodePlatformAndSeesItInTheList(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	var empty platformList
	admin.Get("/api/code-platforms").ExpectStatus(http.StatusOK).JSON(&empty)
	if len(empty.Items) != 0 {
		t.Fatalf("fresh system lists platforms: %+v", empty.Items)
	}

	var created platformBody
	admin.Post("/api/code-platforms", map[string]string{
		"kind": "gitlab", "baseUrl": "https://gitlab.example.com", "token": "glpat-abcdefghij1234567890",
	}).ExpectStatus(http.StatusCreated).JSON(&created)
	if created.ID == "" || created.Kind != "gitlab" || created.BaseURL != "https://gitlab.example.com" || created.CreatedAt == "" {
		t.Fatalf("created = %+v", created)
	}

	var list platformList
	admin.Get("/api/code-platforms").ExpectStatus(http.StatusOK).JSON(&list)
	if len(list.Items) != 1 || list.Items[0] != created {
		t.Fatalf("list = %+v, want [%+v]", list.Items, created)
	}
}
