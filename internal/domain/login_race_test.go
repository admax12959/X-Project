package domain

import (
	"context"
	"testing"
	"time"
)

// loginRepo is the part of the repository the end of a password login touches. The authenticators
// it lists change between calls, which is how ConfirmTOTP enabling the second factor in the
// middle of a login is played back deterministically.
type loginRepo struct {
	Repository
	authenticators [][]Authenticator // answer of the 1st, 2nd, ... ListAuthenticators call; the last repeats
	listCalls      int
	sessions       map[string]Session
	challenges     map[string]LoginChallenge
}

func newLoginRepo(answers ...[]Authenticator) *loginRepo {
	return &loginRepo{authenticators: answers, sessions: map[string]Session{}, challenges: map[string]LoginChallenge{}}
}

func (r *loginRepo) ListAuthenticators(context.Context, UserID) ([]Authenticator, error) {
	i := r.listCalls
	if i >= len(r.authenticators) {
		i = len(r.authenticators) - 1
	}
	r.listCalls++
	return r.authenticators[i], nil
}
func (r *loginRepo) CreateSession(_ context.Context, s Session) error {
	r.sessions[s.TokenHash] = s
	return nil
}
func (r *loginRepo) DeleteSession(_ context.Context, h string) error {
	delete(r.sessions, h)
	return nil
}
func (r *loginRepo) DeleteExpiredLoginChallenges(context.Context, time.Time) error { return nil }
func (r *loginRepo) CreateLoginChallenge(_ context.Context, c LoginChallenge) error {
	r.challenges[c.TokenHash] = c
	return nil
}
func (r *loginRepo) GetConfigSection(context.Context, string) ([]byte, time.Time, bool, error) {
	return nil, time.Time{}, false, nil
}

var totpOn = []Authenticator{{Type: AuthenticatorTOTP, Enabled: true}}

func TestALoginThatRacesTheEnablingOfTheSecondFactorGetsAChallengeNotASession(t *testing.T) {
	// 1st look: no authenticator (the login decides on a session); 2nd look, after the session
	// was inserted: ConfirmTOTP has enabled one in between.
	repo := newLoginRepo(nil, totpOn)
	svc := NewService(repo, Options{})
	res, err := svc.openSession(context.Background(), User{ID: 7}, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Token != "" || res.SecondFactorToken == "" {
		t.Fatalf("result = %+v, want a login challenge and no session token", res)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("sessions left behind = %v, want none: the password-only session must be deleted", repo.sessions)
	}
	if ch := repo.challenges[hashToken(res.SecondFactorToken)]; ch.UserID != 7 || !ch.MustChangePassword {
		t.Fatalf("challenge = %+v, want one for the user carrying mustChangePassword", ch)
	}
}

func TestALoginWithoutARaceKeepsItsSession(t *testing.T) {
	repo := newLoginRepo(nil)
	svc := NewService(repo, Options{})
	res, err := svc.openSession(context.Background(), User{ID: 7}, false)
	if err != nil || res.Token == "" || len(repo.sessions) != 1 || len(repo.challenges) != 0 {
		t.Fatalf("result = %+v, %v; sessions %d, challenges %d", res, err, len(repo.sessions), len(repo.challenges))
	}
}

// A session that passed the second factor is complete; an authenticator that exists does not
// make it suspect (the recheck is for password-only sessions).
func TestASessionThatPassedTheSecondFactorSkipsTheRecheck(t *testing.T) {
	repo := newLoginRepo(totpOn)
	svc := NewService(repo, Options{})
	res, err := svc.createSession(context.Background(), User{ID: 7}, false, true)
	if err != nil || res.Token == "" || len(repo.sessions) != 1 {
		t.Fatalf("result = %+v, %v; sessions %d", res, err, len(repo.sessions))
	}
}
