package api

import (
	"net/http"
	"strings"
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

func TestRegisteringACodePlatformRequiresKindAddressAndToken(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	res := admin.Post("/api/code-platforms", map[string]string{"kind": "", "baseUrl": " ", "token": ""})
	res.ExpectError(http.StatusUnprocessableEntity, "kind", "required")
	res.ExpectError(http.StatusUnprocessableEntity, "baseUrl", "required")
	res.ExpectError(http.StatusUnprocessableEntity, "token", "required")

	var list platformList
	admin.Get("/api/code-platforms").ExpectStatus(http.StatusOK).JSON(&list)
	if len(list.Items) != 0 {
		t.Fatalf("rejected registration was stored: %+v", list.Items)
	}
}

func TestRegisteringACodePlatformValidatesTypeAndAddress(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	const token = "glpat-abcdefghij1234567890"

	admin.Post("/api/code-platforms", map[string]string{"kind": "bitbucket", "baseUrl": "https://git.example.com", "token": token}).
		ExpectError(http.StatusUnprocessableEntity, "kind", "invalid")
	for _, bad := range []string{
		"gitlab.example.com", "ftp://gitlab.example.com", "https://", "not a url",
		"https://user:pw@gitlab.example.com", "https://gitlab.example.com/?x=1", "https://gitlab.example.com/#frag",
	} {
		admin.Post("/api/code-platforms", map[string]string{"kind": "gitlab", "baseUrl": bad, "token": token}).
			ExpectError(http.StatusUnprocessableEntity, "baseUrl", "invalid")
	}

	// Surrounding blanks and a trailing slash are tidied; a port and a path prefix are kept.
	var created platformBody
	admin.Post("/api/code-platforms", map[string]string{"kind": "github", "baseUrl": "  http://git.example.com:8443/gitlab/  ", "token": token}).
		ExpectStatus(http.StatusCreated).JSON(&created)
	if created.Kind != "github" || created.BaseURL != "http://git.example.com:8443/gitlab" {
		t.Fatalf("created = %+v", created)
	}
}

func TestCodePlatformTokenIsNeverReturnedInFull(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	const token = "glpat-abcdefghij1234567890"

	create := admin.Post("/api/code-platforms", map[string]string{"kind": "gitlab", "baseUrl": "https://gitlab.example.com", "token": token}).
		ExpectStatus(http.StatusCreated)
	list := admin.Get("/api/code-platforms").ExpectStatus(http.StatusOK)
	for name, res := range map[string]*apitest.Response{"create": create, "list": list} {
		if strings.Contains(string(res.Body), token) || strings.Contains(string(res.Body), token[:len(token)-4]) {
			t.Fatalf("%s response leaks the token: %s", name, res.Body)
		}
	}
	var created platformBody
	create.JSON(&created)
	if created.TokenTail != "7890" {
		t.Fatalf("tokenTail = %q, want the last 4 characters", created.TokenTail)
	}
}

func TestShortTokensRevealNoTail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	var created platformBody
	env.AdminClient().Post("/api/code-platforms", map[string]string{"kind": "github", "baseUrl": "https://github.example.com", "token": "abc1234"}).
		ExpectStatus(http.StatusCreated).JSON(&created)
	if created.TokenTail != "" {
		t.Fatalf("tokenTail = %q for a short token", created.TokenTail)
	}
}

func TestOnlyAdministratorsCanUseCodePlatformAPIs(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	body := map[string]string{"kind": "gitlab", "baseUrl": "https://gitlab.example.com", "token": "glpat-abcdefghij1234567890"}

	user := env.SeedUserClient("alice", "alice-password")
	user.Get("/api/code-platforms").ExpectError(http.StatusForbidden, "", "forbidden")
	user.Post("/api/code-platforms", body).ExpectError(http.StatusForbidden, "", "forbidden")

	env.Client().Get("/api/code-platforms").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	env.Client().Post("/api/code-platforms", body).ExpectError(http.StatusUnauthorized, "", "unauthenticated")

	var list platformList
	env.AdminClient().Get("/api/code-platforms").ExpectStatus(http.StatusOK).JSON(&list)
	if len(list.Items) != 0 {
		t.Fatalf("a rejected request registered a platform: %+v", list.Items)
	}
}

func TestCodePlatformsAreListedOldestFirst(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	for _, u := range []string{"https://gitlab-a.example.com", "https://gitlab-b.example.com"} {
		admin.Post("/api/code-platforms", map[string]string{"kind": "gitlab", "baseUrl": u, "token": "glpat-abcdefghij1234567890"}).ExpectStatus(http.StatusCreated)
	}
	var list platformList
	admin.Get("/api/code-platforms").ExpectStatus(http.StatusOK).JSON(&list)
	if len(list.Items) != 2 || list.Items[0].BaseURL != "https://gitlab-a.example.com" || list.Items[1].BaseURL != "https://gitlab-b.example.com" {
		t.Fatalf("list = %+v", list.Items)
	}
}
