package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 default; the algorithm every authenticator app supports
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	"rsc.io/qr"
)

// TOTP (RFC 6238) with the parameters every authenticator app understands: HMAC-SHA1,
// 30-second steps, 6 digits, a 160-bit secret (RFC 4226 section 4 recommends 160 bits).
const (
	totpSecretBytes = 20
	totpDigits      = 6
	totpPeriodSecs  = 30
	totpIssuer      = "X-Project"
	// totpSkewSteps: a code of one step before or after the current one is accepted, for
	// clocks that are a little off.
	totpSkewSteps = 1
)

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() ([]byte, error) {
	b := make([]byte, totpSecretBytes)
	_, err := rand.Read(b)
	return b, err
}

func totpStepAt(t time.Time) int64 { return t.Unix() / totpPeriodSecs }

// totpCode is the HOTP value (RFC 4226) of the secret for one counter, as a zero-padded string.
func totpCode(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, bin%mod)
}

// matchTOTP checks code against the steps around now and returns the step it belongs to.
// Every candidate is compared, in constant time, so the answer does not leak which step
// matched. A step that is not after lastUsed is a replay and does not match.
func matchTOTP(secret []byte, code string, now time.Time, lastUsed int64) (step int64, ok bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	cur := totpStepAt(now)
	for s := cur - totpSkewSteps; s <= cur+totpSkewSteps; s++ {
		match := subtle.ConstantTimeCompare([]byte(totpCode(secret, s)), []byte(code)) == 1
		if match && s > lastUsed && !ok {
			step, ok = s, true
		}
	}
	return step, ok
}

// otpauthURI is the key URI (the de-facto Key Uri Format) that authenticator apps read from
// the QR code.
func otpauthURI(secret []byte, account string) string {
	q := url.Values{}
	q.Set("secret", totpEncoding.EncodeToString(secret))
	q.Set("issuer", totpIssuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriodSecs))
	return "otpauth://totp/" + url.PathEscape(totpIssuer+":"+account) + "?" + q.Encode()
}

// qrSVG draws text as a QR code: one SVG with a white background, a four-module quiet zone
// and one path for all the dark modules.
func qrSVG(text string) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	const quiet = 4
	size := code.Size + 2*quiet
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, size, size)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, size, size)
	for y := 0; y < code.Size; y++ {
		for x := 0; x < code.Size; x++ {
			if code.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String(), nil
}
