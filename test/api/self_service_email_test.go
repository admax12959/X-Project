package api

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/apitest"
)

// Self-service login identities (#30): an LDAP user adds an email identity, an email-identity
// user changes the login email. Both prove they own the new address with a one-time code that
// arrives by mail; the code is only ever read out of the delivered mail (the fake mailer).

const emmaEmail = "emma@example.com"

var verificationCode = regexp.MustCompile(`\b(\d{6})\b`)

// codeFromMail waits for the n-th mail to address and returns the verification code in it.
func codeFromMail(t *testing.T, env *apitest.Env, address string, n int) string {
	t.Helper()
	eventually(t, "mail to "+address, func() bool { return len(env.Mailer.SentTo(address)) >= n })
	m := env.Mailer.SentTo(address)[n-1].Message
	got := verificationCode.FindStringSubmatch(m.Text)
	if got == nil {
		t.Fatalf("no verification code in the mail to %s:\n%s", address, m.Text)
	}
	if h := verificationCode.FindStringSubmatch(m.HTML); h == nil || h[1] != got[1] {
		t.Fatalf("the HTML alternative does not carry the same code:\n%s", m.HTML)
	}
	return got[1]
}

func requestBindCode(c *apitest.Client, email string) *apitest.Response {
	return c.Post("/api/me/email-identity/code", map[string]string{"email": email})
}

func bindEmail(c *apitest.Client, email, code, password string) *apitest.Response {
	return c.Post("/api/me/email-identity", map[string]string{"email": email, "code": code, "password": password})
}

func requestChangeCode(c *apitest.Client, email string) *apitest.Response {
	return c.Post("/api/me/login-email/code", map[string]string{"email": email})
}

func changeLoginEmail(c *apitest.Client, email, code string) *apitest.Response {
	return c.Put("/api/me/login-email", map[string]string{"email": email, "code": code})
}

// emmaWithMail is an LDAP-only user in an environment whose mail works.
func emmaWithMail(t *testing.T) (*apitest.Env, *apitest.Client, *apitest.Client) {
	t.Helper()
	env, admin := configuredMail(t)
	addEmma(env)
	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	return env, admin, emma
}

func TestLDAPUserAddsAnEmailIdentityWithTheCodeFromTheMail(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)

	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	var me meBody
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectStatus(http.StatusCreated).JSON(&me)

	if len(me.Identities) != 2 || me.Identities[1].Kind != "email" || me.Identities[1].LoginName != emmaEmail || me.Identities[1].BoundVia != "self" {
		t.Fatalf("identities = %+v, want ldap and the new email identity bound by the user", me.Identities)
	}
	// The directory still owns the contact email.
	if me.Email != "emma.larsen@corp.example" {
		t.Errorf("contact email = %q, want the directory's", me.Email)
	}
	// The password they set works at once, with no forced change.
	var s loginState
	env.Client().LoginEmail(emmaEmail, "emma-email-pw-1").ExpectStatus(http.StatusOK).JSON(&s)
	if s.MustChangePassword || s.User.ID != me.ID {
		t.Fatalf("login = %+v, want emma without a forced password change", s)
	}
}

func TestAddingAnEmailIdentityNeedsMail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	emma := env.Client()
	loginLDAP(emma, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)

	requestBindCode(emma, emmaEmail).ExpectError(http.StatusServiceUnavailable, "", "mailNotConfigured")
	bindEmail(emma, emmaEmail, "123456", "emma-email-pw-1").ExpectError(http.StatusServiceUnavailable, "", "mailNotConfigured")
	if me := getMe(t, emma); len(me.Identities) != 1 {
		t.Fatalf("identities = %+v", me.Identities)
	}
}

var _ = time.Second
