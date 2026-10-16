package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

// atomicRepo plays a database with real transactions for the end of the login's second step:
// what runs inside InTx is staged and only becomes visible if the function returns nil; a call
// on the repository itself (outside any transaction) is recorded, because a challenge spent
// there and a session inserted there are two committed operations a password reset can slip
// between.
type atomicRepo struct {
	Repository
	challenges map[string]LoginChallenge
	sessions   map[string]Session
	identity   *LoginIdentity
	failInsert bool
	outside    []string
}

type atomicTx struct {
	Repository
	parent     *atomicRepo
	challenges map[string]LoginChallenge
	sessions   map[string]Session
}

func newAtomicRepo() *atomicRepo {
	return &atomicRepo{challenges: map[string]LoginChallenge{}, sessions: map[string]Session{}}
}

func (r *atomicRepo) InTx(_ context.Context, fn func(Repository) error) error {
	tx := &atomicTx{parent: r, challenges: map[string]LoginChallenge{}, sessions: map[string]Session{}}
	for k, v := range r.challenges {
		tx.challenges[k] = v
	}
	for k, v := range r.sessions {
		tx.sessions[k] = v
	}
	if err := fn(tx); err != nil {
		return err
	}
	r.challenges, r.sessions = tx.challenges, tx.sessions
	return nil
}

func (r *atomicRepo) DeleteLoginChallenge(context.Context, string) (bool, error) {
	r.outside = append(r.outside, "DeleteLoginChallenge")
	return false, errors.New("outside a transaction")
}
func (r *atomicRepo) CreateSession(context.Context, Session) error {
	r.outside = append(r.outside, "CreateSession")
	return errors.New("outside a transaction")
}
func (r *atomicRepo) GetConfigSection(context.Context, string) ([]byte, time.Time, bool, error) {
	return nil, time.Time{}, false, nil
}

func (t *atomicTx) DeleteLoginChallenge(_ context.Context, h string) (bool, error) {
	_, ok := t.challenges[h]
	delete(t.challenges, h)
	return ok, nil
}
func (t *atomicTx) CreateSession(_ context.Context, s Session) error {
	if t.parent.failInsert {
		return errors.New("insert failed")
	}
	t.sessions[s.TokenHash] = s
	return nil
}
func (t *atomicTx) FindIdentityByUser(context.Context, UserID, IdentityKind) (LoginIdentity, bool, error) {
	if t.parent.identity == nil {
		return LoginIdentity{}, false, nil
	}
	return *t.parent.identity, true, nil
}

func pendingChallenge(r *atomicRepo, mustChange bool) LoginChallenge {
	ch := LoginChallenge{TokenHash: "ch-hash", UserID: 7, MustChangePassword: mustChange}
	r.challenges[ch.TokenHash] = ch
	return ch
}

func TestSpendingTheChallengeAndOpeningTheSessionIsOneTransaction(t *testing.T) {
	repo := newAtomicRepo()
	ch := pendingChallenge(repo, false)
	res, err := NewService(repo, Options{}).completeSecondFactorLogin(context.Background(), User{ID: 7}, ch)
	if err != nil || res.Token == "" {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if len(repo.outside) != 0 {
		t.Fatalf("operations outside the transaction: %v", repo.outside)
	}
	if len(repo.challenges) != 0 || len(repo.sessions) != 1 {
		t.Fatalf("challenges %d, sessions %d, want 0 and 1", len(repo.challenges), len(repo.sessions))
	}
}

func TestAFailedSessionInsertKeepsTheChallenge(t *testing.T) {
	repo := newAtomicRepo()
	repo.failInsert = true
	ch := pendingChallenge(repo, false)
	if _, err := NewService(repo, Options{}).completeSecondFactorLogin(context.Background(), User{ID: 7}, ch); err == nil {
		t.Fatal("want the insert error")
	}
	if len(repo.challenges) != 1 || len(repo.sessions) != 0 {
		t.Fatalf("challenges %d, sessions %d, want the challenge kept and no session", len(repo.challenges), len(repo.sessions))
	}
}

// A reset that committed after the challenge was read has deleted it: no session, however
// valid the code was.
func TestAChallengeThatAResetAlreadySpentOpensNoSession(t *testing.T) {
	repo := newAtomicRepo()
	ch := LoginChallenge{TokenHash: "gone", UserID: 7}
	_, err := NewService(repo, Options{}).completeSecondFactorLogin(context.Background(), User{ID: 7}, ch)
	if !isUnauthenticated(err) || len(repo.sessions) != 0 {
		t.Fatalf("err = %v, sessions = %d, want unauthenticated and none", err, len(repo.sessions))
	}
}

// The restriction is decided from the identity as it is when the session opens, not from what
// the challenge carried: a password changed in between does not leave the session restricted.
func TestTheSessionsRestrictionIsRecomputedFromTheCurrentIdentity(t *testing.T) {
	for _, tc := range []struct {
		name         string
		challenge    bool
		identity     *LoginIdentity
		wantRestrict bool
	}{
		{"still a temporary password", true, &LoginIdentity{MustChangePassword: true}, true},
		{"password changed meanwhile", true, &LoginIdentity{MustChangePassword: false}, false},
		{"email identity removed meanwhile", true, nil, false},
		{"a directory login of a user whose email password is temporary", false, &LoginIdentity{MustChangePassword: true}, false},
	} {
		repo := newAtomicRepo()
		repo.identity = tc.identity
		ch := pendingChallenge(repo, tc.challenge)
		res, err := NewService(repo, Options{}).completeSecondFactorLogin(context.Background(), User{ID: 7}, ch)
		if err != nil || res.MustChangePassword != tc.wantRestrict {
			t.Fatalf("%s: result = %+v, %v", tc.name, res, err)
		}
		for _, s := range repo.sessions {
			if s.MustChangePassword != tc.wantRestrict {
				t.Fatalf("%s: stored session = %+v", tc.name, s)
			}
		}
	}
}

func isUnauthenticated(err error) bool {
	e, ok := AsError(err)
	return ok && e.Kind == KindUnauthenticated
}
