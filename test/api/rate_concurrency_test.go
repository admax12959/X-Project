package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/apitest"
)

// Limits and lockouts hold under concurrent requests: a burst of parallel calls is counted
// exactly like the same calls one after the other, and a burst never ends in a server error
// (a database deadlock would show up as a 500).

// outcome is a response reduced to what the tests compare: the status and the first error code.
type outcome struct {
	status int
	code   string
}

func outcomeOf(r *apitest.Response) outcome {
	var b struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(r.Body, &b)
	o := outcome{status: r.Status}
	if len(b.Errors) > 0 {
		o.code = b.Errors[0].Code
	}
	return o
}

// burst runs n calls at the same time and returns how often each outcome happened.
func burst(t *testing.T, n int, call func(i int) *apitest.Response) map[outcome]int {
	t.Helper()
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		got = map[outcome]int{}
	)
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			o := outcomeOf(call(i))
			mu.Lock()
			got[o]++
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()
	for o := range got {
		if o.status >= 500 {
			t.Errorf("a parallel call answered %d %s; outcomes = %v", o.status, o.code, got)
		}
	}
	return got
}

func TestRecoveryLimitHoldsUnderParallelRequests(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)

	got := burst(t, 24, func(int) *apitest.Response { return requestRecovery(env.Client(), bob.Email) })

	// Three per address and hour (the per-address limit is the tighter one here).
	if n := got[outcome{http.StatusAccepted, ""}]; n != 3 {
		t.Errorf("accepted = %d, want exactly 3; outcomes = %v", n, got)
	}
	if n := got[outcome{http.StatusTooManyRequests, "rateLimited"}]; n != 21 {
		t.Errorf("rateLimited = %d, want 21; outcomes = %v", n, got)
	}
}

func TestEmailCodeSendLimitHoldsUnderParallelRequests(t *testing.T) {
	t.Parallel()
	_, _, emma := emmaWithMail(t)

	got := burst(t, 20, func(int) *apitest.Response { return requestBindCode(emma, emmaEmail) })

	if n := got[outcome{http.StatusAccepted, ""}]; n != 5 {
		t.Errorf("accepted = %d, want exactly 5 (per user and hour); outcomes = %v", n, got)
	}
	if n := got[outcome{http.StatusTooManyRequests, "rateLimited"}]; n != 15 {
		t.Errorf("rateLimited = %d, want 15; outcomes = %v", n, got)
	}
}

func TestEmailCodeGuessLimitHoldsUnderParallelRequests(t *testing.T) {
	t.Parallel()
	env, _, emma := emmaWithMail(t)
	requestBindCode(emma, emmaEmail).ExpectStatus(http.StatusAccepted)
	code := codeFromMail(t, env, emmaEmail, 1)

	// 20 different wrong codes at once: only five of them may be looked at.
	got := burst(t, 20, func(i int) *apitest.Response {
		wrong := fmt.Sprintf("%06d", 100000+i)
		if wrong == code {
			wrong = "000000"
		}
		return bindEmail(emma, emmaEmail, wrong, "emma-email-pw-1")
	})

	if n := got[outcome{http.StatusUnprocessableEntity, "tokenInvalid"}]; n != 5 {
		t.Errorf("wrong codes looked at = %d, want exactly 5; outcomes = %v", n, got)
	}
	if n := got[outcome{http.StatusTooManyRequests, "rateLimited"}]; n != 15 {
		t.Errorf("rateLimited = %d, want 15; outcomes = %v", n, got)
	}
	bindEmail(emma, emmaEmail, code, "emma-email-pw-1").ExpectError(http.StatusTooManyRequests, "", "rateLimited")
}

func TestSecondFactorLockoutHoldsUnderParallelRequests(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	at := frozenMidStep(env)
	secret := tinaWithTOTP(t, env)
	at = env.Clock.Now()

	// 20 parallel second steps, each with its own login, each with a wrong code.
	var clients []*apitest.Client
	for range 20 {
		c, _ := firstFactor(t, env)
		clients = append(clients, c)
	}
	got := burst(t, len(clients), func(i int) *apitest.Response {
		return clients[i].Post(secondFactorLoginPath, map[string]string{"code": "000000"})
	})

	// Four wrong codes are answered as wrong; the fifth locks and the rest are refused.
	if n := got[outcome{http.StatusUnprocessableEntity, "totpInvalid"}]; n != 4 {
		t.Errorf("totpInvalid = %d, want exactly 4; outcomes = %v", n, got)
	}
	if n := got[outcome{http.StatusTooManyRequests, "totpLocked"}]; n != 16 {
		t.Errorf("totpLocked = %d, want 16; outcomes = %v", n, got)
	}
	c, _ := firstFactor(t, env)
	c.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at)}).ExpectError(http.StatusTooManyRequests, "", "totpLocked")
}

// The old events are pruned while parallel requests count: that must neither deadlock nor
// change the counts.
func TestRecoveryLimitHoldsUnderParallelRequestsWhileOldEventsArePruned(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	for i := range 6 {
		requestRecovery(env.Client(), fmt.Sprintf("old%d@example.com", i)).ExpectStatus(http.StatusAccepted)
	}
	env.Clock.Advance(61 * time.Minute)

	got := burst(t, 24, func(int) *apitest.Response { return requestRecovery(env.Client(), bob.Email) })

	if n := got[outcome{http.StatusAccepted, ""}]; n != 3 {
		t.Errorf("accepted = %d, want exactly 3; outcomes = %v", n, got)
	}
	if n := got[outcome{http.StatusTooManyRequests, "rateLimited"}]; n != 21 {
		t.Errorf("rateLimited = %d, want 21; outcomes = %v", n, got)
	}
}
