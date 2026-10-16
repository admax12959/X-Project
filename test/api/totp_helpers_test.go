package api

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/apitest"
)

// The tests compute verification codes themselves, straight from RFC 6238 / RFC 4226, so
// they agree with the server only if both follow the standard.

const totpStep = 30 * time.Second

// totpAt is the RFC 6238 code (HMAC-SHA1, 6 digits) of the secret for the time step that
// contains at.
func totpAt(t testing.TB, secret string, at time.Time) string {
	t.Helper()
	return totpForStep(t, secret, at.Unix()/int64(totpStep/time.Second))
}

func totpForStep(t testing.TB, secret string, step int64) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		t.Fatalf("secret %q is not base32: %v", secret, err)
	}
	return hotp(key, uint64(step), 6)
}

func hotp(key []byte, counter uint64, digits int) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, bin%mod)
}

// The helper itself is checked against the SHA-1 test vectors of RFC 6238 Appendix B (8 digits;
// the 6-digit code is the last six of those).
func TestTheCodeHelperMatchesTheRFC6238TestVectors(t *testing.T) {
	t.Parallel()
	key := []byte("12345678901234567890")
	for _, v := range []struct {
		unix int64
		code string
	}{{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"}, {1234567890, "89005924"}, {2000000000, "69279037"}} {
		if got := hotp(key, uint64(v.unix/30), 8); got != v.code {
			t.Errorf("T=%d: code %s, want %s", v.unix, got, v.code)
		}
		if got := hotp(key, uint64(v.unix/30), 6); got != v.code[2:] {
			t.Errorf("T=%d: 6-digit code %s, want %s", v.unix, got, v.code[2:])
		}
	}
}

// frozenMidStep stops the application's clock in the middle of a time step, so that a step
// never flips during a test, and returns that time. Steps are counted from the Unix epoch.
func frozenMidStep(env *apitest.Env) time.Time {
	now := time.Now().UTC()
	at := time.Unix((now.Unix()/30)*30+15, 0).UTC()
	env.Clock.Freeze(at)
	return at
}

// setupBody is the answer of POST /api/me/second-factor/totp.
type setupBody struct {
	Secret     string `json:"secret"`
	OtpauthURI string `json:"otpauthUri"`
	QrSvg      string `json:"qrSvg"`
}

type secondFactorStatus struct {
	Enabled        bool `json:"enabled"`
	Authenticators []struct {
		Type      string `json:"type"`
		EnabledAt string `json:"enabledAt"`
	} `json:"authenticators"`
}

const (
	totpSetupPath   = "/api/me/second-factor/totp"
	totpConfirmPath = "/api/me/second-factor/totp/confirm"
)

// startTOTP asks for a new secret, as the security tab does after the password was typed again.
func startTOTP(t testing.TB, c *apitest.Client, password string) setupBody {
	t.Helper()
	var s setupBody
	c.Post(totpSetupPath, map[string]string{"password": password}).ExpectStatus(http.StatusOK).JSON(&s)
	return s
}

// enableTOTP turns the second factor on for the user the client is logged in as, confirming with
// the code of the clock's current step, and returns the secret and the time the code was for.
func enableTOTP(t testing.TB, c *apitest.Client, password string) (secret string, usedAt time.Time) {
	t.Helper()
	s := startTOTP(t, c, password)
	usedAt = c.Env().Clock.Now()
	c.Post(totpConfirmPath, map[string]string{"code": totpAt(t, s.Secret, usedAt)}).ExpectStatus(http.StatusOK)
	return s.Secret, usedAt
}

func secondFactorOf(t testing.TB, c *apitest.Client) secondFactorStatus {
	t.Helper()
	var s secondFactorStatus
	c.Get("/api/me/second-factor").ExpectStatus(http.StatusOK).JSON(&s)
	return s
}

func queryOf(t testing.TB, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

func mustJSON(t testing.TB, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
