package api

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// A first step passed with the old password is worthless once the password has changed: the
// challenge is deleted with the change, so even a right code opens no session.
func TestAChallengeFromBeforeAPasswordChangeCannotBeCompleted(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret := tinaWithTOTP(t, env)
	at := env.Clock.Now()

	pending, _ := firstFactor(t, env) // the old password, first step only

	// Another login of the same user completes both steps and changes the password.
	other, _ := firstFactor(t, env)
	other.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, at)}).ExpectStatus(http.StatusOK)
	changePassword(other, "initial-pw-1", "changed-pw-123").ExpectStatus(http.StatusNoContent)

	env.Clock.Advance(totpStep)
	pending.Post(secondFactorLoginPath, map[string]string{"code": totpAt(t, secret, env.Clock.Now())}).ExpectStatus(http.StatusUnauthorized)
	pending.Get("/api/session").ExpectStatus(http.StatusUnauthorized)
}

// Several changes that all know the old password: the first wins, the others find the password
// already changed (or their session gone), and nothing ends in a server error.
func TestParallelPasswordChangesWithTheSameOldPasswordLetAtMostOneThrough(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	first := plainUserClient(t, env, "tina")
	const n = 8
	clients := []*apitest.Client{first}
	for range n - 1 {
		c := env.Client()
		c.LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK)
		clients = append(clients, c)
	}
	got := burst(t, n, func(i int) *apitest.Response {
		return changePassword(clients[i], "initial-pw-1", fmt.Sprintf("changed-pw-%d-xyz", i))
	})
	if got[outcome{http.StatusNoContent, ""}] != 1 {
		t.Fatalf("outcomes = %v, want exactly one change to succeed", got)
	}
	for o := range got {
		if o.status >= 500 {
			t.Fatalf("outcomes = %v, want no server error", got)
		}
	}
}

// Logins with the old password racing the change: whatever they obtained, no session that
// started from the old password is alive afterwards (except the one that made the change).
func TestNoSessionFromTheOldPasswordSurvivesAParallelPasswordChange(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	changer := plainUserClient(t, env, "tina")
	const n = 8
	logins := make([]*apitest.Client, n)
	for i := range logins {
		logins[i] = env.Client()
	}
	var (
		wg sync.WaitGroup
		mu sync.Mutex
		ok []*apitest.Client
	)
	start := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		changePassword(changer, "initial-pw-1", "changed-pw-123").ExpectStatus(http.StatusNoContent)
	}()
	for _, c := range logins {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if r := c.LoginAs("tina", "initial-pw-1"); r.Status == http.StatusOK {
				mu.Lock()
				ok = append(ok, c)
				mu.Unlock()
			} else if r.Status >= 500 {
				t.Errorf("login answered %d", r.Status)
			}
		}()
	}
	close(start)
	wg.Wait()
	for _, c := range ok {
		c.Get("/api/session").ExpectStatus(http.StatusUnauthorized)
	}
	changer.Get("/api/session").ExpectStatus(http.StatusOK)
}
