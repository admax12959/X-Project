package api

import (
	"bytes"
	"database/sql"
	"encoding/base32"
	"net/http"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/httpapi"
)

const secondFactorLoginPath = "/api/session/second-factor"

type pendingBody struct {
	SecondFactorRequired bool   `json:"secondFactorRequired"`
	ExpiresAt            string `json:"expiresAt"`
}

func cookieNamed(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// tinaWithTOTP creates tina with the second factor on and the clock one step later (the step
// of the confirming code is used up). It returns her secret.
func tinaWithTOTP(t *testing.T, env *apitest.Env) string {
	t.Helper()
	c := plainUserClient(t, env, "tina")
	secret, _ := enableTOTP(t, c, "initial-pw-1")
	env.Clock.Advance(2 * totpStep) // the confirming code's step and the one after it are behind us
	return secret
}

// firstFactor logs tina in with her password and expects to be asked for a code.
func firstFactor(t *testing.T, env *apitest.Env) (*apitest.Client, *apitest.Response) {
	t.Helper()
	c := env.Client()
	r := c.LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK)
	var b pendingBody
	r.JSON(&b)
	if !b.SecondFactorRequired || b.ExpiresAt == "" {
		t.Fatalf("login answer = %s, want secondFactorRequired", r.Body)
	}
	return c, r
}

func TestAUserWithoutASecondFactorLogsInWithThePasswordAlone(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	plainUserClient(t, env, "tina")
	c := env.Client()
	r := c.LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK)
	if cookieNamed(r.Cookies, httpapi.SessionCookie) == nil || (cookieNamed(r.Cookies, httpapi.SecondFactorCookie) != nil && cookieNamed(r.Cookies, httpapi.SecondFactorCookie).MaxAge >= 0) {
		t.Fatalf("cookies = %v, want a session and no second-factor token", r.Cookies)
	}
}

func TestWithASecondFactorThePasswordOnlyGetsAShortTokenAndTheCodeOpensTheSession(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	at := frozenMidStep(env)
	secret := tinaWithTOTP(t, env)
	at = env.Clock.Now()

	c, r := firstFactor(t, env)
	if cookieNamed(r.Cookies, httpapi.SessionCookie) != nil {
		t.Fatalf("a session was opened before the code: %v", r.Cookies)
	}
	pending := cookieNamed(r.Cookies, httpapi.SecondFactorCookie)
	if pending == nil || !pending.HttpOnly || pending.Expires.Before(at) || pending.Expires.After(at.Add(10*time.Minute)) {
		t.Fatalf("second-factor cookie = %+v, want a short-lived HttpOnly token", pending)
	}
	c.Get("/api/session").ExpectStatus(http.StatusUnauthorized)

	var s sessionBody
	ok := c.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at)}).ExpectStatus(http.StatusOK)
	ok.JSON(&s)
	if s.User.Username != "tina" {
		t.Fatalf("user = %+v", s.User)
	}
	sess := cookieNamed(ok.Cookies, httpapi.SessionCookie)
	if sess == nil || sess.Value == "" || sess.Value == pending.Value {
		t.Fatalf("session cookie = %+v, want a new token, not the short one", sess)
	}
	c.Get("/api/session").ExpectStatus(http.StatusOK)

	// The short token is spent.
	again := c.WithCookies([]*http.Cookie{pending})
	again.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at.Add(totpStep))}).ExpectStatus(http.StatusUnauthorized)
}

func TestTheShortTokenOpensNothingButTheSecondStep(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tinaWithTOTP(t, env)
	_, r := firstFactor(t, env)
	pending := cookieNamed(r.Cookies, httpapi.SecondFactorCookie)

	// Presented as a session, anywhere.
	asSession := env.Client().WithCookies([]*http.Cookie{{Name: httpapi.SessionCookie, Value: pending.Value, Path: "/"}})
	for _, path := range []string{"/api/session", "/api/me", "/api/applications", "/api/me/second-factor"} {
		asSession.Get(path).ExpectStatus(http.StatusUnauthorized)
	}
	asSession.Post(totpSetupPath, map[string]string{"password": "initial-pw-1"}).ExpectStatus(http.StatusUnauthorized)
	asSession.Delete("/api/session").ExpectStatus(http.StatusUnauthorized)

	// A real session is no token for the second step either.
	full := plainUserClient(t, env, "uma")
	sess := cookieNamed(full.Cookies(), httpapi.SessionCookie)
	if sess == nil {
		t.Fatal("no session cookie")
	}
	asToken := env.Client().WithCookies([]*http.Cookie{{Name: httpapi.SecondFactorCookie, Value: sess.Value, Path: secondFactorLoginPath}})
	asToken.Post(secondFactorLoginPath, map[string]string{"code": "123456"}).ExpectStatus(http.StatusUnauthorized)

	// No token at all.
	env.Client().Post(secondFactorLoginPath, map[string]string{"code": "123456"}).ExpectStatus(http.StatusUnauthorized)
}

func TestTheShortTokenExpires(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	at := frozenMidStep(env)
	secret := tinaWithTOTP(t, env)
	at = env.Clock.Now()
	c, _ := firstFactor(t, env)

	env.Clock.Advance(6 * time.Minute)
	c.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at.Add(6*time.Minute))}).ExpectStatus(http.StatusUnauthorized)
}

func TestTheCodeOfThePreviousCurrentAndNextStepAreAccepted(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		shift int
		ok    bool
	}{{"previous", -1, true}, {"current", 0, true}, {"next", 1, true}, {"two before", -2, false}, {"two after", 2, false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := apitest.New(t)
			at := frozenMidStep(env)
			secret := tinaWithTOTP(t, env)
			at = env.Clock.Now()
			c, _ := firstFactor(t, env)
			r := c.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at.Add(time.Duration(tc.shift)*totpStep))})
			if tc.ok {
				r.ExpectStatus(http.StatusOK)
			} else {
				r.ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
			}
		})
	}
}

func TestACodeWorksOnceAndTheSameStepsCodeIsARepeat(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	at := frozenMidStep(env)
	secret := tinaWithTOTP(t, env)
	at = env.Clock.Now()
	code := totpAt(t, secret, at)

	c, _ := firstFactor(t, env)
	c.Post(secondFactorLoginPath, map[string]string{"code": code}).ExpectStatus(http.StatusOK)

	// A second login in the same step with the very same code is refused ...
	c2, _ := firstFactor(t, env)
	c2.Post(secondFactorLoginPath, map[string]string{"code": code}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	// ... and so is the code of the step before it, which is older than the one used.
	c2.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at.Add(-totpStep))}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	// The next step's code is fresh.
	c2.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at.Add(totpStep))}).ExpectStatus(http.StatusOK)
}

func TestTheCodeThatConfirmedTheSetupCannotLogIn(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	c := plainUserClient(t, env, "tina")
	secret, usedAt := enableTOTP(t, c, "initial-pw-1")

	c2, _ := firstFactor(t, env)
	c2.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, usedAt)}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
}

func TestABlankCodeIsRequiredAndAWrongOneIsInvalid(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tinaWithTOTP(t, env)
	c, _ := firstFactor(t, env)
	c.Post(secondFactorLoginPath, map[string]string{"code": ""}).ExpectError(http.StatusUnprocessableEntity, "code", "required")
	c.Post(secondFactorLoginPath, map[string]string{"code": "abcdef"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	c.Get("/api/session").ExpectStatus(http.StatusUnauthorized)
}

func TestAnLDAPLoginNeedsTheCodeToo(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	at := frozenMidStep(env)
	addEmma(env)
	c := env.Client()
	loginLDAP(c, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	secret, _ := enableTOTP(t, c, "ldap-pw")
	env.Clock.Advance(totpStep)
	at = env.Clock.Now()

	c2 := env.Client()
	var b pendingBody
	r := loginLDAP(c2, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	r.JSON(&b)
	if !b.SecondFactorRequired || cookieNamed(r.Cookies, httpapi.SessionCookie) != nil {
		t.Fatalf("LDAP login answer = %s, cookies %v, want a pending login", r.Body, r.Cookies)
	}
	c2.Get("/api/session").ExpectStatus(http.StatusUnauthorized)
	c2.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at)}).ExpectStatus(http.StatusOK)
	c2.Get("/api/session").ExpectStatus(http.StatusOK)

	// A wrong directory password never gets as far as the code.
	loginLDAP(env.Client(), "elarsen", "nope").ExpectStatus(http.StatusUnauthorized)
}

func TestTheEmailIdentityOfAUserWithTheSecondFactorAsksForTheCodeToo(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tinaWithTOTP(t, env)
	// A wrong password is still just invalidCredentials, nothing about the second factor.
	env.Client().LoginAs("tina", "wrong").ExpectError(http.StatusUnauthorized, "", "invalidCredentials")
}

func TestFiveWrongCodesLockTheSecondStepForTheUserAndItUnlocksByItself(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	at := frozenMidStep(env)
	secret := tinaWithTOTP(t, env)
	at = env.Clock.Now()

	c, _ := firstFactor(t, env)
	for i := 0; i < 4; i++ {
		c.Post(secondFactorLoginPath, map[string]string{"code": "000000"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	}
	// The fifth wrong code locks.
	e := c.Post(secondFactorLoginPath, map[string]string{"code": "000000"}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")
	if e.Params["lockedUntil"] == nil {
		t.Errorf("totpLocked params = %v, want lockedUntil", e.Params)
	}
	// Now even the right code is refused, on this login and on a new one; the password step
	// itself still works.
	good := totpAt(t, secret, at)
	c.Post(secondFactorLoginPath, map[string]string{"code": good}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")
	c2, _ := firstFactor(t, env)
	c2.Post(secondFactorLoginPath, map[string]string{"code": good}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")

	// Another user is not affected.
	plainUserClient(t, env, "uma")

	// Still locked a little before the end, free after it.
	env.Clock.Advance(14 * time.Minute)
	c3, _ := firstFactor(t, env)
	c3.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, env.Clock.Now())}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")
	env.Clock.Advance(2 * time.Minute)
	c4, _ := firstFactor(t, env)
	c4.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, env.Clock.Now())}).ExpectStatus(http.StatusOK)
}

func TestARightCodeStartsTheCountOfWrongCodesAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret := tinaWithTOTP(t, env)

	for round := 0; round < 3; round++ {
		c, _ := firstFactor(t, env)
		for i := 0; i < 4; i++ {
			c.Post(secondFactorLoginPath, map[string]string{"code": "000000"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
		}
		c.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, env.Clock.Now())}).ExpectStatus(http.StatusOK)
		env.Clock.Advance(totpStep)
	}
}

func TestTheFailuresAreCountedPerUserAcrossLogins(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tinaWithTOTP(t, env)
	for i := 0; i < 4; i++ {
		c, _ := firstFactor(t, env) // a new login each time
		c.Post(secondFactorLoginPath, map[string]string{"code": "000000"}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	}
	c, _ := firstFactor(t, env)
	c.Post(secondFactorLoginPath, map[string]string{"code": "000000"}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")
}

func TestTheSecretIsStoredEncryptedWithTheMasterKey(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	c := plainUserClient(t, env, "tina")
	s := startTOTP(t, c, "initial-pw-1")

	db, err := sql.Open("mysql", env.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var stored []byte
	var version int
	if err := db.QueryRow("SELECT secret_enc, key_version FROM user_authenticators LIMIT 1").Scan(&stored, &version); err != nil {
		t.Fatal(err)
	}
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s.Secret)
	if len(stored) <= len(key) || bytes.Contains(stored, key) || bytes.Contains(stored, []byte(s.Secret)) {
		t.Fatalf("stored secret (%d bytes) is not sealed", len(stored))
	}
	if version != 1 {
		t.Errorf("key version = %d, want the master key version 1", version)
	}
}

func TestWhenTheMasterKeyChangedTheCodeCannotBeCheckedAndSaysSo(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret := tinaWithTOTP(t, env)
	at := env.Clock.Now()

	env.Restart(func(c *app.Config) { c.SecretKey = "another-master-key-entirely" })
	c := env.Client()
	c.LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK)
	c.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at)}).ExpectError(http.StatusServiceUnavailable, "", "secondFactorUnreadable")
	c.Get("/api/session").ExpectStatus(http.StatusUnauthorized)
}
