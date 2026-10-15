package api

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// Password recovery and invitations (#21), seen the way the user sees them: a request on the
// login page, a mail in the inbox (the fake mailer), a link in that mail, a page that sets the
// password. The token is only ever read out of the delivered mail.

const siteAddress = "https://xp.corp.example"

var setPasswordLink = regexp.MustCompile(regexp.QuoteMeta(siteAddress) + `/set-password/([A-Za-z0-9_-]+)`)

// tokenFromMail waits for the n-th mail to address and returns the token in its link.
func tokenFromMail(t *testing.T, env *apitest.Env, address string, n int) string {
	t.Helper()
	eventually(t, "mail to "+address, func() bool { return len(env.Mailer.SentTo(address)) >= n })
	m := env.Mailer.SentTo(address)[n-1].Message
	got := setPasswordLink.FindStringSubmatch(m.Text)
	if got == nil {
		t.Fatalf("no set-password link in the mail to %s:\n%s", address, m.Text)
	}
	if h := setPasswordLink.FindStringSubmatch(m.HTML); h == nil || h[1] != got[1] {
		t.Fatalf("the HTML alternative does not carry the same link:\n%s", m.HTML)
	}
	return got[1]
}

func requestRecovery(c *apitest.Client, email string) *apitest.Response {
	return c.Post("/api/password-recovery", map[string]string{"email": email})
}

func setPassword(c *apitest.Client, token, password string) *apitest.Response {
	return c.Post("/api/password-setup", map[string]string{"token": token, "password": password})
}

func TestUserRecoversTheirPasswordWithTheLinkInTheMail(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)

	anon := env.Client()
	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	token := tokenFromMail(t, env, bob.Email, 1)

	setPassword(anon, token, "brand-new-pw-1").ExpectStatus(http.StatusNoContent)

	env.Client().LoginAs("bob", "brand-new-pw-1").ExpectStatus(http.StatusOK)
	env.Client().LoginAs("bob", "initial-pw-1").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
}
