package api

import (
	"net/http"
	"regexp"
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
