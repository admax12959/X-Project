package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/domain"
)

// ADR-0004: the directory owns the namespace. Renaming a local user is a creation of a
// name, so it follows the same rule as creating a user.
func TestRenamingALocalUserToALoginNameInTheDirectoryIsRefused(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	env.Directory.AddAccount("retired", "pw", domain.DirectoryEntry{DisplayName: "Retired Person"})
	env.Directory.SetDisabled("retired", true)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)

	patchUser(admin, jane.ID, map[string]any{"username": "elarsen"}).ExpectError(http.StatusUnprocessableEntity, "username", "usernameExistsInLdap")
	// A disabled directory account still owns its name.
	patchUser(admin, jane.ID, map[string]any{"username": "retired"}).ExpectError(http.StatusUnprocessableEntity, "username", "usernameExistsInLdap")
	env.Client().LoginLocal("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)

	// A name the directory does not know is fine.
	patchUser(admin, jane.ID, map[string]any{"username": "contractor1"}).ExpectStatus(http.StatusOK)
}

func TestRenamingALocalUserFailsWhenTheDirectoryCannotBeAsked(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, nil)
	env.Directory.SetUnreachable(true)

	patchUser(admin, jane.ID, map[string]any{"username": "contractor1"}).ExpectError(http.StatusServiceUnavailable, "", "ldapUnreachable")
	env.Client().LoginLocal("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	env.Client().LoginLocal("contractor1", "initial-pw-1").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")

	// Edits that are not a rename never need the directory.
	patchUser(admin, jane.ID, map[string]any{"username": "jdoe", "phone": "555-0100"}).ExpectStatus(http.StatusOK)
	patchUser(admin, jane.ID, map[string]any{"department": "Ops"}).ExpectStatus(http.StatusOK)
}
