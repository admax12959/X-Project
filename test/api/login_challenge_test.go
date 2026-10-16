package api

import (
	"net/http"
	"sync"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/httpapi"
)

// The short token of a login's first step (login_challenges) opens at most one session, and
// does not outlive a password reset.

func TestOneChallengeOpensAtMostOneSessionWhateverHappensInParallel(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	c := plainUserClient(t, env, "tina")
	_, codes := enableWithCodes(t, c, "initial-pw-1")
	_, r := firstFactor(t, env)
	pending := cookieNamed(r.Cookies, httpapi.SecondFactorCookie)

	var wg sync.WaitGroup
	var mu sync.Mutex
	sessions := 0
	for _, code := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := env.Client().WithCookies([]*http.Cookie{pending}).Post(secondFactorLoginPath, map[string]string{"code": code})
			if res.Status == http.StatusOK && cookieNamed(res.Cookies, httpapi.SessionCookie) != nil {
				mu.Lock()
				sessions++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if sessions != 1 {
		t.Fatalf("%d sessions from one challenge, want exactly 1", sessions)
	}
}

func TestAPasswordResetByLinkSpendsThePendingLoginChallenges(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	frozenMidStep(env)
	c := plainUserClient(t, env, "tina")
	_, codes := enableWithCodes(t, c, "initial-pw-1")
	enableMail(t, admin)
	_, r := firstFactor(t, env)
	pending := cookieNamed(r.Cookies, httpapi.SecondFactorCookie)

	requestRecovery(env.Client(), apitest.Email("tina")).ExpectStatus(http.StatusAccepted)
	token := tokenFromMail(t, env, apitest.Email("tina"), 1)
	setPassword(env.Client(), token, "tinas-new-pw-123").ExpectStatus(http.StatusNoContent)

	env.Client().WithCookies([]*http.Cookie{pending}).Post(secondFactorLoginPath, map[string]string{"code": codes[0]}).
		ExpectStatus(http.StatusUnauthorized)
}

func TestAPasswordResetByTheCommandLineSpendsThePendingLoginChallenges(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	c := plainUserClient(t, env, "tina")
	_, codes := enableWithCodes(t, c, "initial-pw-1")
	_, r := firstFactor(t, env)
	pending := cookieNamed(r.Cookies, httpapi.SecondFactorCookie)

	if res := env.CLI("reset-user", "--user", "tina", "--password"); res.Exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", res.Exit, res.Stderr)
	}
	env.Client().WithCookies([]*http.Cookie{pending}).Post(secondFactorLoginPath, map[string]string{"code": codes[0]}).
		ExpectStatus(http.StatusUnauthorized)
}
