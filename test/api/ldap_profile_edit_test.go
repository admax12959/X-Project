package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/domain"
)

const oddDirectoryEmail = "odd-mail-without-at-sign"

// An LDAP-synced email is written without local validation (the directory owns it), so
// it may not look like an address. That must not block the fields the user does own.
func TestLDAPUserWithAnOddDirectoryEmailStillSavesPhoneAndLanguage(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.AddAccount("odd", "ldap-pw", domain.DirectoryEntry{
		DisplayName: "Odd Mail", Email: oddDirectoryEmail, Department: "Payments Platform",
	})
	odd := env.Client()
	loginLDAP(odd, "odd", "ldap-pw").ExpectStatus(http.StatusOK)

	patchMe(odd, map[string]any{"phone": "555-0177"}).ExpectStatus(http.StatusOK)
	patchMe(odd, map[string]any{"language": "en"}).ExpectStatus(http.StatusOK)
	// The settings form submits the directory's own values back as well.
	patchMe(odd, map[string]any{
		"displayName": "Odd Mail", "email": oddDirectoryEmail, "department": "Payments Platform", "phone": "555-0178",
	}).ExpectStatus(http.StatusOK)
	me := getMe(t, odd)
	if me.Phone != "555-0178" || me.Language != "en" || me.Email != oddDirectoryEmail {
		t.Fatalf("me = %+v, want the phone and language saved and the directory email untouched", me)
	}

	// The fields the user owns are still validated.
	patchMe(odd, map[string]any{"phone": strings.Repeat("9", 65)}).ExpectError(http.StatusUnprocessableEntity, "phone", "tooLong")
	patchMe(odd, map[string]any{"language": "fr"}).ExpectError(http.StatusUnprocessableEntity, "language", "invalid")
}

func TestAdminCanEditThePhoneOfAnLDAPUserWhoseDirectoryEmailIsOdd(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.Directory.AddAccount("odd", "ldap-pw", domain.DirectoryEntry{
		DisplayName: "Odd Mail", Email: oddDirectoryEmail,
	})
	loginLDAP(env.Client(), "odd", "ldap-pw").ExpectStatus(http.StatusOK)
	admin := env.AdminClient()
	odd := findUser(t, listUsers(t, admin), "odd")

	var edited userBody
	patchUser(admin, odd.ID, map[string]any{"phone": "555-0166"}).ExpectStatus(http.StatusOK).JSON(&edited)
	if edited.Phone != "555-0166" || edited.Email != oddDirectoryEmail {
		t.Fatalf("edited = %+v, want the phone saved and the directory email untouched", edited)
	}
}
