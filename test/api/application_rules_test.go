package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// appFixture is a backend with a code platform, one known repository and one Owner candidate.
type appFixture struct {
	env   *apitest.Env
	admin *apitest.Client
	pid   string
	owner userBody
}

func newAppFixture(t *testing.T) appFixture {
	t.Helper()
	env := apitest.New(t)
	admin := env.AdminClient()
	pid := registerPlatform(t, admin, "gitlab", glURL)
	env.CodePlatforms.AddRepo(glURL, "payments/payments-core", "1182")
	owner := createUser(t, admin, map[string]any{"username": "elarsen", "displayName": "Emma Larsen"})
	return appFixture{env: env, admin: admin, pid: pid, owner: owner}
}

func (f appFixture) post(over map[string]any) *apitest.Response {
	return f.admin.Post("/api/applications", newApp(f.pid, []string{f.owner.ID}, over))
}

func (f appFixture) expectNoApplications(t *testing.T) {
	t.Helper()
	var list appListBody
	f.admin.Get("/api/applications").ExpectStatus(http.StatusOK).JSON(&list)
	if len(list.Items) != 0 {
		t.Fatalf("a rejected request created applications: %+v", list.Items)
	}
}

func TestApplicationKeyMustBeASlug(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	for _, bad := range []string{"A-upper", "1starts-with-digit", "-dash", "a", "has_underscore", "has space", "a" + strings.Repeat("b", 40)} {
		f.post(map[string]any{"key": bad}).ExpectError(http.StatusUnprocessableEntity, "key", "invalid")
	}
	f.post(map[string]any{"key": " "}).ExpectError(http.StatusUnprocessableEntity, "key", "required")
	f.expectNoApplications(t)

	// Boundaries: 2 and 40 characters are fine.
	f.post(map[string]any{"key": "ab"}).ExpectStatus(http.StatusCreated)
	f.post(map[string]any{"key": "a23456789012345678901234567890123456789b"}).ExpectStatus(http.StatusCreated)
}

func TestApplicationKeyMustBeGloballyUnique(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	f.post(nil).ExpectStatus(http.StatusCreated)
	f.post(map[string]any{"name": "Another"}).ExpectError(http.StatusUnprocessableEntity, "key", "keyTaken")
}

func TestApplicationNameIsRequired(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	f.post(map[string]any{"name": "  "}).ExpectError(http.StatusUnprocessableEntity, "name", "required")
	f.expectNoApplications(t)
}

func TestMainBranchDefaultsToMainAndCanBeChanged(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	var a appBody
	f.post(map[string]any{"key": "default-branch"}).ExpectStatus(http.StatusCreated).JSON(&a)
	if a.MainBranch != "main" {
		t.Fatalf("mainBranch = %q, want main", a.MainBranch)
	}
	f.post(map[string]any{"key": "dev-branch", "mainBranch": "develop"}).ExpectStatus(http.StatusCreated).JSON(&a)
	if a.MainBranch != "develop" {
		t.Fatalf("mainBranch = %q, want develop", a.MainBranch)
	}
	f.post(map[string]any{"key": "bad-branch", "mainBranch": "not a branch"}).ExpectError(http.StatusUnprocessableEntity, "mainBranch", "invalid")
}

func TestAnApplicationNeedsAtLeastOneOwnerButNoPE(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	f.admin.Post("/api/applications", newApp(f.pid, []string{}, nil)).ExpectError(http.StatusUnprocessableEntity, "owners", "required")
	f.admin.Post("/api/applications", newApp(f.pid, nil, nil)).ExpectError(http.StatusUnprocessableEntity, "owners", "required")
	f.expectNoApplications(t)

	var a appBody
	f.post(nil).ExpectStatus(http.StatusCreated).JSON(&a)
	if len(a.PEs) != 0 || len(a.Owners) != 1 {
		t.Fatalf("owners %+v pes %+v", a.Owners, a.PEs)
	}
}

func TestOwnersAndPEsMustBeExistingEnabledUsers(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	gone := createUser(t, f.admin, map[string]any{"username": "srossi", "displayName": "Sofia Rossi"})
	f.admin.Post("/api/users/"+gone.ID+"/disable", nil).ExpectStatus(http.StatusOK)
	unknown := strconv.Itoa(987654)

	for _, tc := range []struct{ name, id string }{{"disabled", gone.ID}, {"unknown", unknown}} {
		e := f.post(map[string]any{"owners": []string{f.owner.ID, tc.id}}).
			ExpectError(http.StatusUnprocessableEntity, "owners", "userUnavailable")
		if e.Params["userId"] != tc.id {
			t.Fatalf("%s owner: params = %v", tc.name, e.Params)
		}
		f.post(map[string]any{"pes": []string{tc.id}}).ExpectError(http.StatusUnprocessableEntity, "pes", "userUnavailable")
	}
	f.post(map[string]any{"owners": []string{"not-an-id"}}).ExpectError(http.StatusUnprocessableEntity, "owners", "invalid")
	f.expectNoApplications(t)
}

func TestAnApplicationCannotBeCreatedWithoutAPassingRepositoryVerification(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	f.env.CodePlatforms.SetForbidden(glURL, "secret/vault")

	f.post(map[string]any{"repo": map[string]any{"platformId": f.pid, "path": "payments/missing"}}).
		ExpectError(http.StatusUnprocessableEntity, "repo.path", "repoNotFound")
	f.post(map[string]any{"repo": map[string]any{"platformId": f.pid, "path": "secret/vault"}}).
		ExpectError(http.StatusUnprocessableEntity, "repo.path", "repoForbidden")
	f.post(map[string]any{"repo": map[string]any{"platformId": "424242", "path": "payments/payments-core"}}).
		ExpectError(http.StatusUnprocessableEntity, "repo.platformId", "invalid")
	f.post(map[string]any{"repo": map[string]any{"platformId": f.pid, "path": ""}}).
		ExpectError(http.StatusUnprocessableEntity, "repo.path", "required")
	f.env.CodePlatforms.SetUnreachable(glURL, true)
	f.post(nil).ExpectError(http.StatusServiceUnavailable, "repo.platformId", "platformUnreachable")
	f.expectNoApplications(t)
}

func TestTheProjectIDComesFromThePlatformNotTheClient(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	body := newApp(f.pid, []string{f.owner.ID}, nil)
	body["repo"] = map[string]any{"platformId": f.pid, "path": "payments/payments-core", "projectId": "666"}
	var a appBody
	f.admin.Post("/api/applications", body).ExpectStatus(http.StatusCreated).JSON(&a)
	if a.Repo.ProjectID != "1182" {
		t.Fatalf("projectId = %q, want the platform's 1182", a.Repo.ProjectID)
	}
}

func TestOnlyAdministratorsCreateApplications(t *testing.T) {
	t.Parallel()
	f := newAppFixture(t)
	body := newApp(f.pid, []string{f.owner.ID}, nil)

	plainUserClient(t, f.env, "alice").Post("/api/applications", body).ExpectError(http.StatusForbidden, "", "forbidden")
	f.env.Client().Post("/api/applications", body).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	// An Owner is not an administrator either.
	f.admin.Post("/api/applications", body).ExpectStatus(http.StatusCreated)
	ownerClient := f.env.Client()
	ownerClient.LoginAs("elarsen", "initial-pw-1").ExpectStatus(http.StatusOK)
	other := newApp(f.pid, []string{f.owner.ID}, map[string]any{"key": "other-app"})
	ownerClient.Post("/api/applications", other).ExpectError(http.StatusForbidden, "", "forbidden")

	var list appListBody
	f.admin.Get("/api/applications").ExpectStatus(http.StatusOK).JSON(&list)
	if len(list.Items) != 1 {
		t.Fatalf("list = %+v", list.Items)
	}
}
