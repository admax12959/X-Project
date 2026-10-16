package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

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

func sessionStatus(c *apitest.Client) int { return c.Get("/api/session").Status }

func TestRecoveryLinkWorksOnlyOnce(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	anon := env.Client()

	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	token := tokenFromMail(t, env, bob.Email, 1)
	setPassword(anon, token, "brand-new-pw-1").ExpectStatus(http.StatusNoContent)

	setPassword(anon, token, "another-new-pw-2").ExpectError(http.StatusUnprocessableEntity, "token", "tokenUsed")
	env.Client().LoginAs("bob", "brand-new-pw-1").ExpectStatus(http.StatusOK)
	env.Client().LoginAs("bob", "another-new-pw-2").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
}

func TestRecoveryLinkExpiresAfterAnHour(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	anon := env.Client()

	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	token := tokenFromMail(t, env, bob.Email, 1)
	env.Clock.Advance(61 * time.Minute)

	setPassword(anon, token, "brand-new-pw-1").ExpectError(http.StatusUnprocessableEntity, "token", "tokenExpired")
	env.Client().LoginAs("bob", "initial-pw-1").ExpectStatus(http.StatusOK)
}

func TestAnUnknownLinkIsRefused(t *testing.T) {
	t.Parallel()
	env, _ := mailEnv(t)
	setPassword(env.Client(), "not-a-token-we-issued", "brand-new-pw-1").ExpectError(http.StatusUnprocessableEntity, "token", "tokenInvalid")
}

func TestAWeakPasswordDoesNotSpendTheLink(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	anon := env.Client()

	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	token := tokenFromMail(t, env, bob.Email, 1)
	setPassword(anon, token, "short").ExpectError(http.StatusUnprocessableEntity, "password", "tooShort")
	setPassword(anon, token, "brand-new-pw-1").ExpectStatus(http.StatusNoContent)
}

func TestAskingAgainReplacesTheEarlierRecoveryLink(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	anon := env.Client()

	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	first := tokenFromMail(t, env, bob.Email, 1)
	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	second := tokenFromMail(t, env, bob.Email, 2)

	setPassword(anon, first, "brand-new-pw-1").ExpectError(http.StatusUnprocessableEntity, "token", "tokenUsed")
	setPassword(anon, second, "brand-new-pw-1").ExpectStatus(http.StatusNoContent)
}

func TestResettingThePasswordEndsEveryOtherSessionOfTheUser(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	laptop, phone := env.Client(), env.Client()
	laptop.LoginAs("bob", "initial-pw-1").ExpectStatus(http.StatusOK)
	phone.LoginAs("bob", "initial-pw-1").ExpectStatus(http.StatusOK)

	anon := env.Client()
	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	setPassword(anon, tokenFromMail(t, env, bob.Email, 1), "brand-new-pw-1").ExpectStatus(http.StatusNoContent)

	if got := sessionStatus(laptop); got != http.StatusUnauthorized {
		t.Fatalf("laptop session after the reset: status %d, want 401", got)
	}
	if got := sessionStatus(phone); got != http.StatusUnauthorized {
		t.Fatalf("phone session after the reset: status %d, want 401", got)
	}
	if got := sessionStatus(admin); got != http.StatusOK {
		t.Fatalf("someone else's session was ended: status %d", got)
	}
}

func TestRecoveryAnswersTheSameWhetherOrNotTheAccountExistsAndSendsNothingForOthers(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	gone := createUser(t, admin, map[string]any{"username": "gone"})
	admin.Post("/api/users/"+gone.ID+"/disable", nil).ExpectStatus(http.StatusOK)
	enableMail(t, admin)
	anon := env.Client()

	var bodies []string
	for _, email := range []string{"nobody@example.com", gone.Email, bob.Email} {
		r := requestRecovery(anon, email).ExpectStatus(http.StatusAccepted)
		bodies = append(bodies, string(r.Body))
	}
	if bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Fatalf("the answers differ: %q", bodies)
	}

	// Mail is prepared in the background: once all of it is done and bob's mail is out, check
	// that nothing was queued for the others.
	env.SettleBackground()
	tokenFromMail(t, env, bob.Email, 1)
	if sent := env.Mailer.Sent(); len(sent) != 1 {
		t.Fatalf("%d mails were sent, want only the one to bob", len(sent))
	}
	out, _ := getOutbox(t, admin, "")
	if len(out.Items) != 1 || out.Items[0].To != bob.Email {
		t.Fatalf("outbox = %+v, want only the mail to bob", out.Items)
	}
}

func TestRecoveryIsLimitedPerEmailAddress(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	anon := env.Client()

	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	requestRecovery(anon, strings.ToUpper(bob.Email)).ExpectStatus(http.StatusAccepted)
	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
	requestRecovery(anon, bob.Email).ExpectError(http.StatusTooManyRequests, "", "rateLimited")
	// An address that does not exist is limited the same way: the limit says nothing about accounts.
	for range 3 {
		requestRecovery(anon, "nobody@example.com").ExpectStatus(http.StatusAccepted)
	}
	requestRecovery(anon, "nobody@example.com").ExpectError(http.StatusTooManyRequests, "", "rateLimited")

	env.Clock.Advance(61 * time.Minute)
	requestRecovery(anon, bob.Email).ExpectStatus(http.StatusAccepted)
}

func TestRecoveryIsLimitedPerClientAddress(t *testing.T) {
	t.Parallel()
	env, _ := mailEnv(t)
	enableMail(t, env.AdminClient())
	anon := env.Client()

	for i := range 10 {
		requestRecovery(anon, fmt.Sprintf("guess%d@example.com", i)).ExpectStatus(http.StatusAccepted)
	}
	requestRecovery(anon, "one-more@example.com").ExpectError(http.StatusTooManyRequests, "", "rateLimited")

	env.Clock.Advance(61 * time.Minute)
	requestRecovery(anon, "one-more@example.com").ExpectStatus(http.StatusAccepted)
}

func TestRecoveryIsUnavailableWithoutMail(t *testing.T) {
	t.Parallel()
	env, _ := mailEnv(t)
	anon := env.Client()

	var methods struct {
		PasswordRecovery bool `json:"passwordRecovery"`
	}
	anon.Get("/api/login-methods").ExpectStatus(http.StatusOK).JSON(&methods)
	if methods.PasswordRecovery {
		t.Fatal("login-methods offers password recovery without mail")
	}
	requestRecovery(anon, "bob@example.com").ExpectError(http.StatusServiceUnavailable, "", "mailNotConfigured")

	enableMail(t, env.AdminClient())
	anon.Get("/api/login-methods").ExpectStatus(http.StatusOK).JSON(&methods)
	if !methods.PasswordRecovery {
		t.Fatal("login-methods does not offer password recovery with mail configured")
	}
}

func TestRecoveryMailIsInTheRecipientsLanguage(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	ellen := createUser(t, admin, map[string]any{"username": "ellen"})
	enableMail(t, admin)
	c := env.Client()
	c.LoginAs("ellen", "initial-pw-1").ExpectStatus(http.StatusOK)
	patchMe(c, map[string]any{"language": "en"}).ExpectStatus(http.StatusOK)

	requestRecovery(env.Client(), ellen.Email).ExpectStatus(http.StatusAccepted)
	tokenFromMail(t, env, ellen.Email, 1)
	if m := env.Mailer.SentTo(ellen.Email)[0].Message; regexp.MustCompile(`\p{Han}`).MatchString(m.Subject + m.Text) {
		t.Fatalf("an English-speaking recipient got a Chinese mail:\n%s\n%s", m.Subject, m.Text)
	}
}

func TestLinksUseTheConfiguredSiteAddressNotTheHostHeader(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)

	req, _ := http.NewRequest(http.MethodPost, env.URL()+"/api/password-recovery", strings.NewReader(`{"email":"`+bob.Email+`"}`))
	req.Host = "evil.example"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	tokenFromMail(t, env, bob.Email, 1) // fails unless the link starts with the configured address
	if m := env.Mailer.SentTo(bob.Email)[0].Message; strings.Contains(m.Text+m.HTML, "evil.example") {
		t.Fatalf("the mail mentions the request's Host header:\n%s", m.Text)
	}
}

// ---------------------------------------------------------------- invitations

func TestAdministratorInvitesAUserByMailAndTheyChooseTheirPassword(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	enableMail(t, admin)

	// Whatever password the administrator typed is not used: the user has none until the link is used.
	u := createUserRaw(t, admin, map[string]any{"username": "carol", "password": "typed-by-admin-1"})
	if len(u.Identities) != 1 || !u.Identities[0].InvitePending {
		t.Fatalf("identities = %+v, want one email identity with an invitation pending", u.Identities)
	}
	env.Client().LoginEmail(u.Email, "typed-by-admin-1").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	env.Client().LoginEmail(u.Email, "").ExpectStatus(http.StatusUnprocessableEntity)

	token := tokenFromMail(t, env, u.Email, 1)
	anon := env.Client()
	setPassword(anon, token, "carols-own-pw-1").ExpectStatus(http.StatusNoContent)

	// No forced change afterwards: the password is the user's own already.
	c := env.Client()
	var login struct {
		MustChangePassword bool `json:"mustChangePassword"`
	}
	c.LoginEmail(u.Email, "carols-own-pw-1").ExpectStatus(http.StatusOK).JSON(&login)
	if login.MustChangePassword {
		t.Fatal("an invited user who chose their password is made to change it")
	}
	if got := c.Get("/api/applications").Status; got != http.StatusOK {
		t.Fatalf("the invited user cannot use the API: %d", got)
	}
	setPassword(anon, token, "carols-own-pw-2").ExpectError(http.StatusUnprocessableEntity, "token", "tokenUsed")
}

func TestAnInvitationNeedsNoPasswordFromTheAdministrator(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	enableMail(t, admin)
	body := newUser(map[string]any{"username": "carol"})
	delete(body, "password")
	admin.Post("/api/users", body).ExpectStatus(http.StatusCreated)
	tokenFromMail(t, env, apitest.Email("carol"), 1)
}

func TestAnInvitationExpiresAfterAWeek(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	enableMail(t, admin)
	u := createUserRaw(t, admin, map[string]any{"username": "carol"})
	token := tokenFromMail(t, env, u.Email, 1)

	env.Clock.Advance(8 * 24 * time.Hour)
	setPassword(env.Client(), token, "carols-own-pw-1").ExpectError(http.StatusUnprocessableEntity, "token", "tokenExpired")
}

func TestAdministratorCanSendTheInvitationAgainAndTheOldLinkStopsWorking(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	enableMail(t, admin)
	u := createUserRaw(t, admin, map[string]any{"username": "carol"})
	first := tokenFromMail(t, env, u.Email, 1)

	admin.Post("/api/users/"+u.ID+"/invitation", nil).ExpectStatus(http.StatusAccepted)
	second := tokenFromMail(t, env, u.Email, 2)
	setPassword(env.Client(), first, "carols-own-pw-1").ExpectError(http.StatusUnprocessableEntity, "token", "tokenUsed")
	setPassword(env.Client(), second, "carols-own-pw-1").ExpectStatus(http.StatusNoContent)

	// Nothing left to invite once the password is set.
	admin.Post("/api/users/"+u.ID+"/invitation", nil).ExpectError(http.StatusUnprocessableEntity, "", "inviteNotPending")
}

func TestOnlyAdministratorsSendInvitations(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	plain := plainUserClient(t, env, "dave")
	enableMail(t, admin)
	u := createUserRaw(t, admin, map[string]any{"username": "carol"})
	plain.Post("/api/users/"+u.ID+"/invitation", nil).ExpectStatus(http.StatusForbidden)
}

func TestWithoutMailTheAdministratorsTemporaryPasswordStillWorks(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	u := createUserRaw(t, admin, map[string]any{"username": "carol", "password": "temp-password-1"})
	if u.Identities[0].InvitePending {
		t.Fatal("an invitation is pending although no mail is configured")
	}
	var login struct {
		MustChangePassword bool `json:"mustChangePassword"`
	}
	env.Client().LoginEmail(u.Email, "temp-password-1").ExpectStatus(http.StatusOK).JSON(&login)
	if !login.MustChangePassword {
		t.Fatal("a temporary password does not force a change at the first login")
	}
	if len(env.Mailer.Sent()) != 0 {
		t.Fatal("a mail was sent without mail being configured")
	}
	// A password is required, as before.
	body := newUser(map[string]any{"username": "erin"})
	delete(body, "password")
	admin.Post("/api/users", body).ExpectError(http.StatusUnprocessableEntity, "password", "required")
}

func pendingInList(t *testing.T, admin *apitest.Client, username string) bool {
	t.Helper()
	for _, u := range listUsers(t, admin) {
		if u.Username == username {
			return len(u.Identities) == 1 && u.Identities[0].InvitePending
		}
	}
	t.Fatalf("user %s is not in the list", username)
	return false
}

func TestTheUserListShowsWhoseInvitationIsStillPending(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	temp := createUserRaw(t, admin, map[string]any{"username": "tempo", "password": "temp-password-1"})
	enableMail(t, admin)
	carol := createUserRaw(t, admin, map[string]any{"username": "carol"})
	_ = temp

	if pendingInList(t, admin, "tempo") {
		t.Fatal("a user with a temporary password is listed as invited")
	}
	if !pendingInList(t, admin, "carol") {
		t.Fatal("an invited user is not listed as pending")
	}
	setPassword(env.Client(), tokenFromMail(t, env, carol.Email, 1), "carols-own-pw-1").ExpectStatus(http.StatusNoContent)
	if pendingInList(t, admin, "carol") {
		t.Fatal("a user who chose their password is still listed as pending")
	}
}

func TestChangingTheLoginEmailKillsLinksSentToTheOldAddress(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	enableMail(t, admin)
	carol := createUserRaw(t, admin, map[string]any{"username": "carol"})
	token := tokenFromMail(t, env, carol.Email, 1)

	admin.Do(http.MethodPatch, "/api/users/"+carol.ID, map[string]any{"loginEmail": "carol.new@example.com"}).ExpectStatus(http.StatusOK)

	setPassword(env.Client(), token, "carols-own-pw-1").ExpectError(http.StatusUnprocessableEntity, "token", "tokenUsed")
	admin.Post("/api/users/"+carol.ID+"/invitation", nil).ExpectStatus(http.StatusAccepted)
	tokenFromMail(t, env, "carol.new@example.com", 1)
}

func TestAnInvitedUserCannotLogInWithTheDummyPassword(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	enableMail(t, admin)
	u := createUserRaw(t, admin, map[string]any{"username": "carol"})
	c := env.Client()
	c.LoginEmail(u.Email, "x-project-dummy").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
	if got := c.Get("/api/session").Status; got != http.StatusUnauthorized {
		t.Fatalf("a session was opened for an invited user: %d", got)
	}
}
