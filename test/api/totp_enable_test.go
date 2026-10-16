package api

import (
	"encoding/base32"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

func TestStartingTheAuthenticatorSetupNeedsThePasswordAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	c := plainUserClient(t, env, "tina")

	c.Post(totpSetupPath, map[string]string{}).ExpectError(http.StatusUnprocessableEntity, "password", "required")
	c.Post(totpSetupPath, map[string]string{"password": "not-my-password"}).ExpectError(http.StatusUnprocessableEntity, "password", "invalidCredentials")
	if secondFactorOf(t, c).Enabled {
		t.Fatal("the second factor is on although nothing was confirmed")
	}
	env.Client().Post(totpSetupPath, map[string]string{"password": "initial-pw-1"}).ExpectStatus(http.StatusUnauthorized)
}

func TestTheSetupGivesA160BitSecretAsAQRCodeAndAsAKeyToTypeIn(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	c := plainUserClient(t, env, "tina")

	s := startTOTP(t, c, "initial-pw-1")

	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s.Secret)
	if err != nil || len(key) != 20 {
		t.Fatalf("secret %q: %d bytes (err %v), want 20 bytes (160 bit) in base32", s.Secret, len(key), err)
	}
	uri, err := url.Parse(s.OtpauthURI)
	if err != nil || uri.Scheme != "otpauth" || uri.Host != "totp" {
		t.Fatalf("otpauth uri = %q (err %v)", s.OtpauthURI, err)
	}
	q := queryOf(t, s.OtpauthURI)
	if q.Get("secret") != s.Secret || q.Get("algorithm") != "SHA1" || q.Get("digits") != "6" || q.Get("period") != "30" || q.Get("issuer") == "" {
		t.Fatalf("otpauth parameters = %v, want the secret, SHA1, 6 digits, 30 seconds and an issuer", q)
	}
	if !strings.Contains(uri.Path, apitest.Email("tina")) {
		t.Fatalf("otpauth label %q does not name the account", uri.Path)
	}
	if !strings.HasPrefix(s.QrSvg, "<svg") {
		t.Fatalf("qrSvg = %.40q, want an SVG image", s.QrSvg)
	}
	// Asking again gives a fresh secret; the first one is never valid.
	again := startTOTP(t, c, "initial-pw-1")
	if again.Secret == s.Secret {
		t.Fatal("a second setup returned the same secret")
	}
}

func TestTheAuthenticatorCountsOnlyAfterTheFirstCodeIsRight(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	at := frozenMidStep(env)
	c := plainUserClient(t, env, "tina")
	s := startTOTP(t, c, "initial-pw-1")

	c.Post(totpConfirmPath, map[string]string{"code": ""}).ExpectError(http.StatusUnprocessableEntity, "code", "required")
	wrong := totpAt(t, s.Secret, at.Add(5*totpStep)) // far outside the window
	c.Post(totpConfirmPath, map[string]string{"code": wrong}).ExpectError(http.StatusUnprocessableEntity, "code", "totpInvalid")
	if secondFactorOf(t, c).Enabled {
		t.Fatal("a wrong code enabled the second factor")
	}
	// A login still needs nothing more than the password.
	env.Client().LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK)

	c.Post(totpConfirmPath, map[string]string{"code": totpAt(t, s.Secret, at)}).ExpectStatus(http.StatusOK)
	st := secondFactorOf(t, c)
	if !st.Enabled || len(st.Authenticators) != 1 || st.Authenticators[0].Type != "totp" || st.Authenticators[0].EnabledAt == "" {
		t.Fatalf("status = %+v, want one enabled totp authenticator", st)
	}
	// Once enabled the setup cannot be started again over it.
	c.Post(totpSetupPath, map[string]string{"password": "initial-pw-1"}).ExpectError(http.StatusUnprocessableEntity, "", "totpAlreadyEnabled")
}

func TestConfirmingWithoutStartingTheSetupFindsNothing(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	at := frozenMidStep(env)
	c := plainUserClient(t, env, "tina")
	c.Post(totpConfirmPath, map[string]string{"code": totpAt(t, "JBSWY3DPEHPK3PXP", at)}).ExpectError(http.StatusNotFound, "", "notFound")
}

func TestEnablingTheSecondFactorIsRecordedInTheAuditLog(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tina := plainUserClient(t, env, "tina")
	admin := env.AdminClient()

	// A setup that was not confirmed leaves no trace.
	startTOTP(t, tina, "initial-pw-1")
	if got := auditOf(t, admin, "secondFactorEnabled"); len(got) != 0 {
		t.Fatalf("audit entries before the confirmation = %+v", got)
	}
	enableTOTP(t, tina, "initial-pw-1")

	got := auditOf(t, admin, "secondFactorEnabled")
	if len(got) != 1 {
		t.Fatalf("secondFactorEnabled entries = %+v, want exactly one", got)
	}
	e := got[0]
	if e.Actor.Username != "tina" || e.Target.Type != "user" || e.Target.Username != "tina" || e.Params["type"] != "totp" {
		t.Errorf("entry = %+v, want tina enabling her own totp", e)
	}
	if strings.Contains(string(mustJSON(t, e)), "secret") {
		t.Errorf("the audit entry mentions a secret: %+v", e)
	}
}

func TestAnLDAPOnlyUserProvesThePasswordToTheDirectory(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	addEmma(env)
	c := env.Client()
	loginLDAP(c, "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)

	c.Post(totpSetupPath, map[string]string{"password": "wrong"}).ExpectError(http.StatusUnprocessableEntity, "password", "invalidCredentials")
	env.Directory.SetUnreachable(true)
	c.Post(totpSetupPath, map[string]string{"password": "ldap-pw"}).ExpectError(http.StatusServiceUnavailable, "", "ldapUnreachable")
	env.Directory.SetUnreachable(false)
	enableTOTP(t, c, "ldap-pw")
	if !secondFactorOf(t, c).Enabled {
		t.Fatal("not enabled")
	}
}
