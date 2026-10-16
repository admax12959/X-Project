package apitest

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP is defined over HMAC-SHA1
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/cli"
)

// Administrators are always required to have a second factor (#23), and one who has none can
// do nothing but set it up. So the test administrator is enrolled once per environment,
// through the real API, the first time a test asks for AdminClient; every AdminClient after
// that shares the same logged-in session (a second login would need a fresh code).

// TOTPCode is the six-digit code of an authenticator with this base32 secret at the time at.
func TOTPCode(secret string, at time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		panic(fmt.Sprintf("secret %q is not base32: %v", secret, err))
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(at.Unix()/30))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := (uint32(sum[off]&0x7f) << 24) | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3])
	return fmt.Sprintf("%06d", n%1_000_000)
}

// CLIResult is what one run of the container's command line printed and returned.
type CLIResult struct {
	Stdout, Stderr string
	Exit           int
}

// CLI runs a command of the container's command line (xproject reset-user ...) against this
// environment's database, in the same process.
func (e *Env) CLI(args ...string) CLIResult {
	var out, errOut bytes.Buffer
	// Through app.OpenService, the way the command in the container builds its service.
	svc, closeDB, err := app.OpenService(context.Background(), e.Config, app.Deps{Now: e.Clock.Now})
	if err != nil {
		e.t.Fatalf("app.OpenService: %v", err)
	}
	defer closeDB()
	code := cli.Run(context.Background(), svc, args, &out, &errOut)
	return CLIResult{Stdout: out.String(), Stderr: errOut.String(), Exit: code}
}

// AdminTOTPSecret is the authenticator secret of the test administrator, enrolled on the
// first AdminClient call ("" before that).
func (e *Env) AdminTOTPSecret() string {
	e.adminMu.Lock()
	defer e.adminMu.Unlock()
	return e.adminSecret
}

// enrollAdmin gives the administrator an authenticator (without audit or mail noise) and logs
// in with a code, once.
func (e *Env) enrollAdmin() []*http.Cookie {
	e.adminMu.Lock()
	defer e.adminMu.Unlock()
	if e.adminCookies != nil {
		return e.adminCookies
	}
	secret, err := e.app.Service.SeedTOTP(context.Background(), 1) // the bootstrap administrator is user 1
	if err != nil {
		e.t.Fatalf("seed the administrator's authenticator: %v", err)
	}
	c := e.Client()
	c.LoginEmail(AdminEmail, AdminPassword).ExpectStatus(http.StatusOK)
	c.Post("/api/session/second-factor", map[string]string{"code": TOTPCode(secret, e.Clock.Now())}).ExpectStatus(http.StatusOK)
	e.adminSecret = secret
	e.adminCookies = c.Cookies()
	return e.adminCookies
}
