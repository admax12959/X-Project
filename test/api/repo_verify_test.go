package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

type verifiedRepoBody struct {
	Repo struct {
		PlatformID string `json:"platformId"`
		Path       string `json:"path"`
		ProjectID  string `json:"projectId"`
	} `json:"repo"`
}

func verifyBody(platformID, path string) map[string]any {
	return map[string]any{"repo": map[string]any{"platformId": platformID, "path": path}}
}

func TestAdminVerifiesARepositoryAndGetsItsProjectID(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	pid := registerPlatform(t, admin, "gitlab", glURL)
	env.CodePlatforms.AddRepo(glURL, "payments/payments-core", "1182")

	var got verifiedRepoBody
	admin.Post("/api/repo-verifications", verifyBody(pid, "payments/payments-core")).ExpectStatus(http.StatusOK).JSON(&got)
	if got.Repo.PlatformID != pid || got.Repo.Path != "payments/payments-core" || got.Repo.ProjectID != "1182" {
		t.Fatalf("verified = %+v", got)
	}
}

func TestRepositoryVerificationFailuresAreDistinguishable(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	pid := registerPlatform(t, admin, "gitlab", glURL)
	env.CodePlatforms.SetForbidden(glURL, "secret/vault")

	admin.Post("/api/repo-verifications", verifyBody(pid, "payments/missing")).
		ExpectError(http.StatusUnprocessableEntity, "repo.path", "repoNotFound")
	admin.Post("/api/repo-verifications", verifyBody(pid, "secret/vault")).
		ExpectError(http.StatusUnprocessableEntity, "repo.path", "repoForbidden")

	env.CodePlatforms.SetUnreachable(glURL, true)
	admin.Post("/api/repo-verifications", verifyBody(pid, "payments/payments-core")).
		ExpectError(http.StatusServiceUnavailable, "repo.platformId", "platformUnreachable")
}

func TestRepositoryVerificationChecksItsInput(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	pid := registerPlatform(t, admin, "gitlab", glURL)

	admin.Post("/api/repo-verifications", verifyBody("", " ")).
		ExpectError(http.StatusUnprocessableEntity, "repo.platformId", "required")
	admin.Post("/api/repo-verifications", verifyBody(pid, " ")).
		ExpectError(http.StatusUnprocessableEntity, "repo.path", "required")
	admin.Post("/api/repo-verifications", verifyBody(pid, "no spaces/allowed here")).
		ExpectError(http.StatusUnprocessableEntity, "repo.path", "invalid")
	admin.Post("/api/repo-verifications", verifyBody("99999", "a/b")).
		ExpectError(http.StatusUnprocessableEntity, "repo.platformId", "invalid")
	admin.Post("/api/repo-verifications", verifyBody("abc", "a/b")).
		ExpectError(http.StatusUnprocessableEntity, "repo.platformId", "invalid")
}

func TestOnlyAdministratorsCanVerifyRepositories(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	pid := registerPlatform(t, env.AdminClient(), "gitlab", glURL)
	env.CodePlatforms.AddRepo(glURL, "a/b", "1")

	env.SeedUserClient("alice", "alice-password").Post("/api/repo-verifications", verifyBody(pid, "a/b")).
		ExpectError(http.StatusForbidden, "", "forbidden")
	env.Client().Post("/api/repo-verifications", verifyBody(pid, "a/b")).
		ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}
