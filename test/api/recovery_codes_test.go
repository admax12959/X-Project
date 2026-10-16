package api

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/apitest"
)

// Recovery codes (#31, ADR-0006): handed out once when the second factor is turned on, each
// good for one login, stored only as hashes.

const (
	recoveryCodesPath = "/api/me/second-factor/recovery-codes"
	disablePath       = "/api/me/second-factor/disable"
)

var recoveryCodeShape = regexp.MustCompile(`^[2-9A-HJ-NP-Z]{4}-[2-9A-HJ-NP-Z]{4}-[2-9A-HJ-NP-Z]{4}$`)

// enrolled is the answer of confirming the setup: the status plus the codes, shown this once.
type enrolled struct {
	secondFactorStatus
	RecoveryCodes []string `json:"recoveryCodes"`
}

type recoveryStatus struct {
	secondFactorStatus
	RecoveryCodesRemaining int      `json:"recoveryCodesRemaining"`
	RecoveryCodes          []string `json:"recoveryCodes"`
}

// enableWithCodes turns the second factor on and returns the secret and the recovery codes.
func enableWithCodes(t testing.TB, c *apitest.Client, password string) (secret string, codes []string) {
	t.Helper()
	s := startTOTP(t, c, password)
	var e enrolled
	c.Post(totpConfirmPath, map[string]string{"code": totpAt(t, s.Secret, c.Env().Clock.Now())}).ExpectStatus(http.StatusOK).JSON(&e)
	return s.Secret, e.RecoveryCodes
}

func recoveryStatusOf(t testing.TB, c *apitest.Client) recoveryStatus {
	t.Helper()
	var s recoveryStatus
	c.Get("/api/me/second-factor").ExpectStatus(http.StatusOK).JSON(&s)
	return s
}

func TestEnablingTheSecondFactorHandsOutRecoveryCodesOnceAndTheStatusOnlyCountsThem(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	c := plainUserClient(t, env, "tina")

	if st := recoveryStatusOf(t, c); st.RecoveryCodesRemaining != 0 {
		t.Fatalf("remaining before enabling = %d, want 0", st.RecoveryCodesRemaining)
	}
	_, codes := enableWithCodes(t, c, "initial-pw-1")

	if len(codes) != 10 {
		t.Fatalf("recovery codes = %v, want 10", codes)
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if !recoveryCodeShape.MatchString(code) || seen[code] {
			t.Fatalf("code %q is malformed or repeated in %v", code, codes)
		}
		seen[code] = true
	}
	st := recoveryStatusOf(t, c)
	if st.RecoveryCodesRemaining != 10 || len(st.RecoveryCodes) != 0 {
		t.Fatalf("status = %+v, want a count of 10 and no codes", st)
	}
}

// tinaWithCodes creates tina with the second factor on and the clock two steps later, as
// tinaWithTOTP does, and returns her secret and recovery codes.
func tinaWithCodes(t *testing.T, env *apitest.Env) (secret string, codes []string, tina *apitest.Client) {
	t.Helper()
	tina = plainUserClient(t, env, "tina")
	secret, codes = enableWithCodes(t, tina, "initial-pw-1")
	env.Clock.Advance(2 * totpStep)
	return secret, codes, tina
}

func TestARecoveryCodeLogsInInPlaceOfTheAuthenticatorCodeAndWorksOnlyOnce(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	_, codes, _ := tinaWithCodes(t, env)

	c, _ := firstFactor(t, env)
	c.Post(secondFactorLoginPath, map[string]string{"code": codes[0]}).ExpectStatus(http.StatusOK)
	c.Get("/api/session").ExpectStatus(http.StatusOK)

	// Spent: the same code does not open another login.
	c2, _ := firstFactor(t, env)
	c2.Post(secondFactorLoginPath, map[string]string{"code": codes[0]}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	c2.Get("/api/session").ExpectStatus(http.StatusUnauthorized)

	// Another code still works, typed the way people type them: lower case, spaces.
	c3, _ := firstFactor(t, env)
	typed := strings.ToLower(strings.ReplaceAll(codes[1], "-", " "))
	c3.Post(secondFactorLoginPath, map[string]string{"code": typed}).ExpectStatus(http.StatusOK)

	if got := recoveryStatusOf(t, c3).RecoveryCodesRemaining; got != 8 {
		t.Fatalf("remaining = %d, want 8", got)
	}
}

// The login page shows a recovery code as XXXX-XXXX-XXXX and the field takes it as typed.
func TestARecoveryCodeIsAcceptedWithItsDashesInLowerCase(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	_, codes, _ := tinaWithCodes(t, env)

	c, _ := firstFactor(t, env)
	c.Post(secondFactorLoginPath, map[string]string{"code": " " + strings.ToLower(codes[2]) + " "}).ExpectStatus(http.StatusOK)
	c.Get("/api/session").ExpectStatus(http.StatusOK)
}

func TestWrongRecoveryCodesAndWrongAuthenticatorCodesFillTheSameCounterAndALockedStepRefusesRecoveryCodes(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	_, codes, _ := tinaWithCodes(t, env)

	c, _ := firstFactor(t, env)
	c.Post(secondFactorLoginPath, map[string]string{"code": "AAAA-AAAA-AAAA"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	c.Post(secondFactorLoginPath, map[string]string{"code": "000000"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	c.Post(secondFactorLoginPath, map[string]string{"code": "BBBB-BBBB-BBBB"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	c.Post(secondFactorLoginPath, map[string]string{"code": "111111"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	// The fifth wrong try, of either kind, locks.
	c.Post(secondFactorLoginPath, map[string]string{"code": "CCCC-CCCC-CCCC"}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")

	c.Post(secondFactorLoginPath, map[string]string{"code": codes[0]}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")
	c2, _ := firstFactor(t, env)
	c2.Post(secondFactorLoginPath, map[string]string{"code": codes[0]}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")

	// After the lock a right recovery code works, and was not spent by the refused tries.
	env.Clock.Advance(16 * time.Minute)
	c3, _ := firstFactor(t, env)
	c3.Post(secondFactorLoginPath, map[string]string{"code": codes[0]}).ExpectStatus(http.StatusOK)
}

// ---------------------------------------------------------------- regenerate and disable

func regenerate(c *apitest.Client, code string) *apitest.Response {
	return c.Post(recoveryCodesPath, map[string]string{"code": code})
}

func disable(c *apitest.Client, code string) *apitest.Response {
	return c.Post(disablePath, map[string]string{"code": code})
}

// nowCode is the authenticator code of the clock's current step.
func nowCode(t *testing.T, env *apitest.Env, secret string) string {
	t.Helper()
	return totpAt(t, secret, env.Clock.Now())
}

func TestRegeneratingRecoveryCodesNeedsACodeAndVoidsTheOldSetAtOnce(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret, old, tina := tinaWithCodes(t, env)

	regenerate(tina, "").ExpectError(http.StatusUnprocessableEntity, "code", "required")
	regenerate(tina, "000000").ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	// A recovery code is a good proof too; it is spent like at a login, and the whole old set is void anyway.
	var fresh recoveryStatus
	regenerate(tina, old[0]).ExpectStatus(http.StatusOK).JSON(&fresh)
	if len(fresh.RecoveryCodes) != 10 || fresh.RecoveryCodesRemaining != 10 || !fresh.Enabled {
		t.Fatalf("answer = %+v, want enabled, ten new codes", fresh)
	}
	for _, c := range fresh.RecoveryCodes {
		if !recoveryCodeShape.MatchString(c) || slicesContain(old, c) {
			t.Fatalf("new code %q is malformed or was in the old set", c)
		}
	}
	if st := recoveryStatusOf(t, tina); st.RecoveryCodesRemaining != 10 || len(st.RecoveryCodes) != 0 {
		t.Fatalf("status = %+v, want a count of 10 and no codes", st)
	}

	// Every old code is void, the new ones work.
	for _, o := range old[1:3] {
		c, _ := firstFactor(t, env)
		c.Post(secondFactorLoginPath, map[string]string{"code": o}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	}
	c, _ := firstFactor(t, env)
	c.Post(secondFactorLoginPath, map[string]string{"code": fresh.RecoveryCodes[0]}).ExpectStatus(http.StatusOK)

	// The authenticator proves it as well (a new step: the earlier use is spent).
	env.Clock.Advance(totpStep)
	regenerate(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
}

func slicesContain(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestRegeneratingOrDisablingWithoutASecondFactorIsNotFoundAndCountsNoFailure(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tina := plainUserClient(t, env, "tina")
	for i := 0; i < 6; i++ {
		regenerate(tina, "000000").ExpectError(http.StatusNotFound, "", "notFound")
		disable(tina, "000000").ExpectError(http.StatusNotFound, "", "notFound")
	}
	// Not locked out for when she does turn it on.
	secret, _ := enableTOTP(t, tina, "initial-pw-1")
	env.Clock.Advance(2 * totpStep)
	regenerate(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
}

func TestDisablingTheSecondFactorNeedsACodeAndLoginsStopAskingForOne(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret, codes, tina := tinaWithCodes(t, env)

	disable(tina, "").ExpectError(http.StatusUnprocessableEntity, "code", "required")
	disable(tina, "999999").ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	if !secondFactorOf(t, tina).Enabled {
		t.Fatal("a wrong code turned the second factor off")
	}
	disable(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)

	st := recoveryStatusOf(t, tina)
	if st.Enabled || len(st.Authenticators) != 0 || st.RecoveryCodesRemaining != 0 {
		t.Fatalf("status = %+v, want off, no authenticator, no recovery codes", st)
	}
	// The password alone logs in again, and the recovery codes are gone.
	var s loginState
	env.Client().LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK).JSON(&s)
	if s.User.Username != "tina" {
		t.Fatalf("login = %+v, want a session", s)
	}

	// She can turn it on again, with a new secret and new codes.
	env.Clock.Advance(totpStep)
	_, again := enableWithCodes(t, tina, "initial-pw-1")
	if len(again) != 10 || slicesContain(codes, again[0]) {
		t.Fatalf("codes after enabling again = %v", again)
	}
}

func TestOneLockoutCounterCoversLoginRegenerateAndDisable(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret, _, tina := tinaWithCodes(t, env)

	login, _ := firstFactor(t, env)
	login.Post(secondFactorLoginPath, map[string]string{"code": "000000"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	regenerate(tina, "AAAA-AAAA-AAAA").ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	disable(tina, "111111").ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	regenerate(tina, "000000").ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	disable(tina, "BBBB-BBBB-BBBB").ExpectError(http.StatusTooManyRequests, "", "totpLocked")

	// Locked for all three, even with the right code.
	regenerate(tina, nowCode(t, env, secret)).ExpectError(http.StatusTooManyRequests, "", "totpLocked")
	disable(tina, nowCode(t, env, secret)).ExpectError(http.StatusTooManyRequests, "", "totpLocked")
	login.Post(secondFactorLoginPath, map[string]string{"code": nowCode(t, env, secret)}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")

	env.Clock.Advance(16 * time.Minute)
	regenerate(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
}

// ---------------------------------------------------------------- audit

func TestEnablingRegeneratingAndDisablingAreAudited(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret, _, tina := tinaWithCodes(t, env)
	admin := env.AdminClient()

	regenerate(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
	env.Clock.Advance(totpStep)
	disable(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)

	for _, action := range []string{"secondFactorEnabled", "recoveryCodesRegenerated", "secondFactorDisabled"} {
		got := auditOf(t, admin, action)
		if len(got) != 1 {
			t.Fatalf("%s entries = %+v, want exactly one", action, got)
		}
		e := got[0]
		if e.Actor.Username != "tina" || e.Target.Type != "user" || e.Target.Username != "tina" {
			t.Errorf("%s entry = %+v, want tina acting on herself", action, e)
		}
		raw := string(mustJSON(t, e))
		if strings.Contains(raw, secret) || recoveryCodeShape.FindString(raw) != "" {
			t.Errorf("%s entry leaks a secret or a code: %s", action, raw)
		}
	}
	// A refused attempt leaves nothing.
	if got := auditOf(t, admin, "secondFactorDisabled"); len(got) != 1 {
		t.Fatalf("disabled entries = %d", len(got))
	}
}

// ---------------------------------------------------------------- notifications

// tinaOnMail is tina with the second factor off, in an environment whose mail was configured
// after she was created (so no invitation is queued for her).
func tinaOnMail(t *testing.T) (env *apitest.Env, tina *apitest.Client) {
	t.Helper()
	env = apitest.New(t)
	frozenMidStep(env)
	tina = plainUserClient(t, env, "tina")
	enableMail(t, env.AdminClient())
	return env, tina
}

func mailsTo(env *apitest.Env, address string) []string {
	var subjects []string
	for _, m := range env.Mailer.SentTo(address) {
		subjects = append(subjects, m.Message.Subject)
	}
	return subjects
}

func TestEnablingRegeneratingAndDisablingMailTheContactEmailWithoutAnyCode(t *testing.T) {
	t.Parallel()
	env, tina := tinaOnMail(t)
	addr := apitest.Email("tina")

	secret, codes := enableWithCodes(t, tina, "initial-pw-1")
	eventually(t, "the enable notice", func() bool { return len(mailsTo(env, addr)) == 1 })
	env.Clock.Advance(2 * totpStep)
	regenerate(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
	eventually(t, "the regenerate notice", func() bool { return len(mailsTo(env, addr)) == 2 })
	env.Clock.Advance(totpStep)
	disable(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
	eventually(t, "the disable notice", func() bool { return len(mailsTo(env, addr)) == 3 })

	subjects := mailsTo(env, addr)
	if subjects[0] == subjects[1] || subjects[1] == subjects[2] || subjects[0] == subjects[2] {
		t.Errorf("subjects = %q, want three different notices", subjects)
	}
	for _, m := range env.Mailer.SentTo(addr) {
		if strings.Contains(m.Message.Text+m.Message.HTML, secret) || strings.Contains(m.Message.Text, codes[0]) {
			t.Errorf("a notice carries a secret or a recovery code: %q", m.Message.Text)
		}
		if m.Message.Text == "" || m.Message.HTML == "" {
			t.Errorf("notice %q misses its text or HTML part", m.Message.Subject)
		}
	}
}

func TestARefusedRegenerateOrDisableSendsNoMail(t *testing.T) {
	t.Parallel()
	env, tina := tinaOnMail(t)
	addr := apitest.Email("tina")
	enableWithCodes(t, tina, "initial-pw-1")
	eventually(t, "the enable notice", func() bool { return len(mailsTo(env, addr)) == 1 })

	regenerate(tina, "000000").ExpectStatus(http.StatusUnprocessableEntity)
	disable(tina, "000000").ExpectStatus(http.StatusUnprocessableEntity)
	time.Sleep(200 * time.Millisecond)
	if got := mailsTo(env, addr); len(got) != 1 {
		t.Fatalf("mails = %q, want only the enable notice", got)
	}
}

func TestWithoutSMTPTheOperationsWorkAndNoMailIsSent(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tina := plainUserClient(t, env, "tina")
	secret, _ := enableWithCodes(t, tina, "initial-pw-1")
	env.Clock.Advance(2 * totpStep)
	regenerate(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
	env.Clock.Advance(totpStep)
	disable(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
	if got := env.Mailer.Sent(); len(got) != 0 {
		t.Fatalf("mails = %d, want none", len(got))
	}
}
