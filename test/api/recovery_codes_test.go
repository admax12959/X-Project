package api

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

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
