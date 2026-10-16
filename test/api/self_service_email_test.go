package api

import (
	"net/http"
	"regexp"
	"strings"
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

func TestAnAddressAnotherUserLogsInWithIsRefusedWhenAskingAndWhenConfirming(t *testing.T) {
	t.Parallel()
	env, admin, emma := emmaWithMail(t)

	// Taken from the start.
	createUserRaw(t, admin, map[string]any{"username": "bob", "loginEmail": "Bob@Example.com"})
	requestBindCode(emma, "bob@example.com").ExpectError(http.StatusUnprocessableEntity, "email", "loginEmailTaken")

	// Taken between the mail and the confirmation.
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	createUserRaw(t, admin, map[string]any{"username": "eve", "loginEmail": emmaEmail})
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "loginName", "loginEmailTaken")
	if me := getMe(t, emma); len(me.Identities) != 1 {
		t.Fatalf("identities = %+v", me.Identities)
	}
}

func TestAUserWhoAlreadyHasAnEmailIdentityCannotAddAnother(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	bob := env.Client()
	bob.LoginAs("bob", "initial-pw-1").ExpectStatus(http.StatusOK)
	requestBindCode(bob, "bob2@example.com").ExpectError(http.StatusUnprocessableEntity, "kind", "identityKindBound")
}

func TestAMalformedAddressOrAMissingCodeOrPasswordIsRefused(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, "not-an-address").ExpectError(http.StatusUnprocessableEntity, "email", "invalid")
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	bindEmail(emma, emmaEmail, "", "emma-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "code", "required")
	bindEmail(emma, emmaEmail, code, "short").ExpectError(http.StatusUnprocessableEntity, "password", "tooShort")
	// A weak password did not spend the code.
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectStatus(http.StatusCreated)
}

func TestTheCodeWorksOnlyOnce(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectStatus(http.StatusCreated)

	unbindOwn(emma, "email").ExpectStatus(http.StatusOK)
	bindEmail(emma, emmaEmail, code, "emma-email-pw-2").ExpectError(http.StatusUnprocessableEntity, "code", "tokenUsed")
}

func TestTheCodeExpiresAfterFifteenMinutes(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	env.Clock.Advance(16 * time.Minute)
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "code", "tokenExpired")
}

func TestTheCodeBelongsToTheAddressItWasSentTo(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	bindEmail(emma, "other@example.com", code, "emma-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "code", "tokenInvalid")
	bindEmail(emma, emmaEmail, "000000", "emma-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "code", "tokenInvalid")
}

func TestACodeIsUselessToAnotherUser(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	env.Directory.AddAccount("jane.doe", "jane-ldap-pw", domainEntry("Jane", "jane.doe@corp.example"))
	jane := env.Client()
	loginLDAP(jane, "jane.doe", "jane-ldap-pw").ExpectStatus(http.StatusOK)

	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	bindEmail(jane, emmaEmail, code, "jane-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "code", "tokenInvalid")
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectStatus(http.StatusCreated)
}

func TestAskingAgainReplacesTheEarlierCode(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	first := codeFromMail(t, env, emmaEmail, 1)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	second := codeFromMail(t, env, emmaEmail, 2)
	if first == second {
		t.Skip("the two random codes happen to be equal")
	}
	bindEmail(emma, emmaEmail, first, "emma-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "code", "tokenUsed")
	bindEmail(emma, emmaEmail, second, "emma-email-pw-1").ExpectStatus(http.StatusCreated)
}

func TestSelfServiceEmailNeedsASession(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	anon := env.Client()
	requestBindCode(anon, emmaEmail).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	bindEmail(anon, emmaEmail, "123456", "emma-email-pw-1").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}

func TestSendingCodesIsLimitedPerUser(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	for i := 0; i < 5; i++ {
		requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	}
	requestBindCode(emma, emmaEmail).ExpectError(http.StatusTooManyRequests, "", "rateLimited")
	if n := len(env.Mailer.SentTo(emmaEmail)); n > 5 {
		t.Fatalf("%d mails sent", n)
	}

	env.Clock.Advance(61 * time.Minute)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
}

func TestSendingCodesIsLimitedPerClientAddress(t *testing.T) {
	t.Parallel()
	env, _ := configuredMail(t)
	var users []*apitest.Client
	for _, name := range []string{"u1", "u2", "u3", "u4", "u5"} {
		env.Directory.AddAccount(name, "ldap-pw", domainEntry(name, name+"@corp.example"))
		c := env.Client()
		loginLDAP(c, name, "ldap-pw").ExpectStatus(http.StatusOK)
		users = append(users, c)
	}
	for _, c := range users[:4] {
		for i := 0; i < 5; i++ {
			requestBindCode(c, emmaEmail).ExpectStatus(http.StatusAccepted)
		}
	}
	// The fifth user is nowhere near their own limit, but the address has used up its.
	requestBindCode(users[4], emmaEmail).ExpectError(http.StatusTooManyRequests, "", "rateLimited")
}

func TestTooManyWrongCodesLockGuessingForAWhile(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	wrong := "000000"
	if code == wrong {
		wrong = "000001"
	}
	for i := 0; i < 5; i++ {
		bindEmail(emma, emmaEmail, wrong, "emma-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "code", "tokenInvalid")
	}
	// Even the right code is refused now.
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectError(http.StatusTooManyRequests, "", "rateLimited")
	env.Clock.Advance(16 * time.Minute)
	// The code itself has expired by then; the lock is over.
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectError(http.StatusUnprocessableEntity, "code", "tokenExpired")
}

// bobWithMail is an email-identity user whose password is set, in an environment whose mail works.
func bobWithMail(t *testing.T) (*apitest.Env, *apitest.Client, *apitest.Client, userBody) {
	t.Helper()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	c := env.Client()
	c.LoginAs("bob", "initial-pw-1").ExpectStatus(http.StatusOK)
	return env, admin, c, bob
}

func TestEmailUserChangesTheLoginEmailAfterVerifyingTheNewAddress(t *testing.T) {
	t.Parallel()
	env, _, bobC, bob := bobWithMail(t)
	const next = "bob.new@example.com"

	requestChangeCode(bobC, " Bob.New@Example.com ").ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, next, 1)
	// Nothing changes until the code is confirmed.
	env.Client().LoginEmail(bob.Email, "initial-pw-1").ExpectStatus(http.StatusOK)

	var me meBody
	changeLoginEmail(bobC, next, code).ExpectStatus(http.StatusOK).JSON(&me)
	if len(me.Identities) != 1 || me.Identities[0].LoginName != next {
		t.Fatalf("identities = %+v", me.Identities)
	}
	// The contact email follows the login email for a user without LDAP.
	if me.Email != next {
		t.Errorf("contact email = %q, want %q", me.Email, next)
	}
	env.Client().LoginEmail(next, "initial-pw-1").ExpectStatus(http.StatusOK)
	env.Client().LoginEmail(bob.Email, "initial-pw-1").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
}

func TestChangingTheLoginEmailKeepsTheDirectorysContactEmailOfAnLDAPUser(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	bindEmail(emma, emmaEmail, codeFromMail(t, env, emmaEmail, 1), "emma-email-pw-1").ExpectStatus(http.StatusCreated)

	requestChangeCode(emma, "emma.new@example.com").ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, "emma.new@example.com", 1)
	var me meBody
	changeLoginEmail(emma, "emma.new@example.com", code).ExpectStatus(http.StatusOK).JSON(&me)
	if me.Email != "emma.larsen@corp.example" || me.Identities[1].LoginName != "emma.new@example.com" {
		t.Fatalf("me = %+v, want the directory's contact email and the new login email", me)
	}
}

func TestTheOldAddressIsToldAboutTheChange(t *testing.T) {
	t.Parallel()
	env, _, bobC, bob := bobWithMail(t)
	requestChangeCode(bobC, "bob.new@example.com").ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, "bob.new@example.com", 1)
	changeLoginEmail(bobC, "bob.new@example.com", code).ExpectStatus(http.StatusOK)

	eventually(t, "the notice to the old address", func() bool { return len(env.Mailer.SentTo(bob.Email)) == 1 })
	m := env.Mailer.SentTo(bob.Email)[0].Message
	if !strings.Contains(m.Text, "bob.new@example.com") || !strings.Contains(m.HTML, "bob.new@example.com") {
		t.Errorf("the notice does not name the new address:\n%s", m.Text)
	}
	if verificationCode.MatchString(m.Text) {
		t.Errorf("the notice carries a code:\n%s", m.Text)
	}
}

func TestChangingTheLoginEmailToATakenOrUnchangedAddressIsRefused(t *testing.T) {
	t.Parallel()
	env, admin, bobC, bob := bobWithMail(t)
	createUserRaw(t, admin, map[string]any{"username": "eve", "loginEmail": "eve@example.com"})

	requestChangeCode(bobC, "eve@example.com").ExpectError(http.StatusUnprocessableEntity, "email", "loginEmailTaken")
	requestChangeCode(bobC, bob.Email).ExpectError(http.StatusUnprocessableEntity, "email", "loginEmailUnchanged")

	// Taken between the mail and the confirmation.
	requestChangeCode(bobC, "later@example.com").ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, "later@example.com", 1)
	createUserRaw(t, admin, map[string]any{"username": "late", "loginEmail": "later@example.com"})
	changeLoginEmail(bobC, "later@example.com", code).ExpectError(http.StatusUnprocessableEntity, "email", "loginEmailTaken")
	env.Client().LoginEmail(bob.Email, "initial-pw-1").ExpectStatus(http.StatusOK)
}

func TestChangingTheLoginEmailNeedsAnEmailIdentityAndMail(t *testing.T) {
	t.Parallel()
	_, _, emma := emmaWithMail(t)
	requestChangeCode(emma, emmaEmail).ExpectError(http.StatusUnprocessableEntity, "email", "noEmailIdentity")
	changeLoginEmail(emma, emmaEmail, "123456").ExpectError(http.StatusUnprocessableEntity, "email", "noEmailIdentity")

	env := apitest.New(t)
	c := env.AdminClient()
	requestChangeCode(c, "x@example.com").ExpectError(http.StatusServiceUnavailable, "", "mailNotConfigured")
	changeLoginEmail(c, "x@example.com", "123456").ExpectError(http.StatusServiceUnavailable, "", "mailNotConfigured")
}

func TestABindCodeDoesNotChangeALoginEmailAndTheReverse(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectStatus(http.StatusCreated)
	changeLoginEmail(emma, emmaEmail, code).ExpectError(http.StatusUnprocessableEntity, "email", "loginEmailUnchanged")
	changeLoginEmail(emma, "emma.new@example.com", code).ExpectError(http.StatusUnprocessableEntity, "code", "tokenInvalid")
}

func TestAnOldPasswordResetLinkDiesWithTheOldLoginEmail(t *testing.T) {
	t.Parallel()
	env, _, bobC, bob := bobWithMail(t)
	requestRecovery(env.Client(), bob.Email).ExpectStatus(http.StatusAccepted)
	token := tokenFromMail(t, env, bob.Email, 1)

	requestChangeCode(bobC, "bob.new@example.com").ExpectStatus(http.StatusAccepted)
	changeLoginEmail(bobC, "bob.new@example.com", codeFromMail(t, env, "bob.new@example.com", 1)).ExpectStatus(http.StatusOK)
	setPassword(env.Client(), token, "brand-new-pw-1").ExpectError(http.StatusUnprocessableEntity, "token", "tokenUsed")
}

const emmaContact = "emma.larsen@corp.example"

// noticesTo returns subject and text of the mails delivered to address, once n of them have arrived.
func noticesTo(t *testing.T, env *apitest.Env, address string, n int) []string {
	t.Helper()
	eventually(t, "mail to "+address, func() bool { return len(env.Mailer.SentTo(address)) >= n })
	var out []string
	for _, m := range env.Mailer.SentTo(address) {
		out = append(out, m.Message.Subject+"\n"+m.Message.Text)
	}
	return out
}

func TestBindingAndUnbindingALoginIdentityNotifiesTheContactEmail(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)

	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	bindEmail(emma, emmaEmail, codeFromMail(t, env, emmaEmail, 1), "emma-email-pw-1").ExpectStatus(http.StatusCreated)
	got := noticesTo(t, env, emmaContact, 1)
	if !strings.Contains(got[0], emmaEmail) {
		t.Errorf("the bind notice does not name the new login:\n%s", got[0])
	}

	unbindOwn(emma, "email").ExpectStatus(http.StatusOK)
	got = noticesTo(t, env, emmaContact, 2)
	if !strings.Contains(got[1], emmaEmail) || got[1] == got[0] {
		t.Errorf("the unbind notice = %q (bind notice %q)", got[1], got[0])
	}
}

func TestAnAdministratorsBindIsNotifiedToo(t *testing.T) {
	t.Parallel()
	env, admin, emma := emmaWithMail(t)
	me := getMe(t, emma)
	adminBind(admin, me.ID, map[string]any{"kind": "email", "loginName": emmaEmail, "password": "temp-pw-12345"}).ExpectStatus(http.StatusCreated)
	noticesTo(t, env, emmaContact, 1)
}

func TestChangingTheLoginEmailOfAnLDAPUserAlsoTellsTheContactEmail(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	bindEmail(emma, emmaEmail, codeFromMail(t, env, emmaEmail, 1), "emma-email-pw-1").ExpectStatus(http.StatusCreated)
	requestChangeCode(emma, "emma.new@example.com").ExpectStatus(http.StatusAccepted)
	changeLoginEmail(emma, "emma.new@example.com", codeFromMail(t, env, "emma.new@example.com", 1)).ExpectStatus(http.StatusOK)

	// The old login address (bind code + change notice) and the directory contact (bind + change notice).
	if got := noticesTo(t, env, emmaEmail, 2); !strings.Contains(got[1], "emma.new@example.com") {
		t.Errorf("old address notices = %q", got)
	}
	if got := noticesTo(t, env, emmaContact, 2); !strings.Contains(got[1], "emma.new@example.com") {
		t.Errorf("contact notices = %q", got)
	}
}

func TestSelfServiceChangesAreRecordedWithTheUserAsOperator(t *testing.T) {
	t.Parallel()
	env, admin, emma := emmaWithMail(t)
	me := getMe(t, emma)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	bindEmail(emma, emmaEmail, codeFromMail(t, env, emmaEmail, 1), "emma-email-pw-1").ExpectStatus(http.StatusCreated)
	requestChangeCode(emma, "emma.new@example.com").ExpectStatus(http.StatusAccepted)
	changeLoginEmail(emma, "emma.new@example.com", codeFromMail(t, env, "emma.new@example.com", 1)).ExpectStatus(http.StatusOK)
	unbindOwn(emma, "email").ExpectStatus(http.StatusOK)
	// Codes and passwords never reach the log; a refused attempt leaves no trace.
	bindEmail(emma, emmaEmail, "000000", "emma-email-pw-1").ExpectStatus(http.StatusUnprocessableEntity)

	bound := auditOf(t, admin, "identityBound")
	if len(bound) != 1 || bound[0].Actor.Username != "elarsen" || bound[0].Target.ID != me.ID ||
		bound[0].Params["kind"] != "email" || bound[0].Params["loginName"] != emmaEmail || bound[0].Params["via"] != "self" {
		t.Errorf("identityBound = %+v", bound)
	}
	changed := auditOf(t, admin, "loginEmailChanged")
	if len(changed) != 1 || changed[0].Actor.Username != "elarsen" || changed[0].Target.ID != me.ID ||
		changed[0].Params["from"] != emmaEmail || changed[0].Params["to"] != "emma.new@example.com" {
		t.Errorf("loginEmailChanged = %+v", changed)
	}
	if unbound := auditOf(t, admin, "identityUnbound"); len(unbound) != 1 || unbound[0].Params["loginName"] != "emma.new@example.com" {
		t.Errorf("identityUnbound = %+v", unbound)
	}
}

func TestAnAdministratorChangingTheLoginEmailTellsTheOldAddressAndTheContactEmail(t *testing.T) {
	t.Parallel()
	env, admin, emma := emmaWithMail(t)
	me := getMe(t, emma)
	adminBind(admin, me.ID, map[string]any{"kind": "email", "loginName": emmaEmail, "password": "temp-pw-12345"}).ExpectStatus(http.StatusCreated)
	noticesTo(t, env, emmaContact, 1)
	before := len(env.Mailer.SentTo(emmaEmail))

	patchUser(admin, me.ID, map[string]any{"loginEmail": "emma.new@example.com"}).ExpectStatus(http.StatusOK)

	old := noticesTo(t, env, emmaEmail, before+1)
	if got := old[len(old)-1]; !strings.Contains(got, "emma.new@example.com") || !strings.Contains(got, emmaEmail) {
		t.Errorf("old address notice = %q, want both addresses named", got)
	}
	contact := noticesTo(t, env, emmaContact, 2)
	if got := contact[1]; !strings.Contains(got, "emma.new@example.com") {
		t.Errorf("contact notice = %q", got)
	}
	for _, m := range append(env.Mailer.SentTo(emmaEmail), env.Mailer.SentTo(emmaContact)...) {
		if strings.Contains(m.Message.Text+m.Message.HTML, "temp-pw-12345") {
			t.Errorf("a notice carries the password: %q", m.Message.Text)
		}
	}
}

func TestAnAdministratorChangingTheLoginEmailOfALocalUserSendsOneNotice(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	jane := createUser(t, admin, nil)
	enableMail(t, admin)
	patchUser(admin, jane.ID, map[string]any{"loginEmail": "jane.new@example.com"}).ExpectStatus(http.StatusOK)

	got := noticesTo(t, env, "jdoe@example.com", 1)
	if !strings.Contains(got[len(got)-1], "jane.new@example.com") {
		t.Errorf("notice = %q", got)
	}
	env.SettleBackground()
	if n := len(env.Mailer.SentTo("jdoe@example.com")); n != 1 {
		t.Errorf("mails to the old address = %d, want exactly one (login and contact email are the same)", n)
	}
	if n := len(env.Mailer.SentTo("jane.new@example.com")); n != 0 {
		t.Errorf("mails to the new address = %d, want none", n)
	}
}
