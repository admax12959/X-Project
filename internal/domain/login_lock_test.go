package domain

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// A login is not a credential change, so it does not go through changeCredential, but it
// takes the same lock on the email identity before it opens a session (openSession). The order
// of a credential change is pinned in credential_change_test.go; these two cases are about
// what the login does with the identity as the lock reads it.

// lockedLoginRepo answers what openSession touches. The identity the lock returns (locked) can
// differ from the one the login verified: that is a reset committing between the two reads.
type lockedLoginRepo struct {
	Repository
	locked     LoginIdentity
	sessions   map[string]Session
	challenges map[string]LoginChallenge
	calls      []string
}

func newLockedLoginRepo(password string) *lockedLoginRepo {
	h, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	id := LoginIdentity{UserID: 7, PasswordHash: string(h), MustChangePassword: true}
	return &lockedLoginRepo{locked: id, sessions: map[string]Session{}, challenges: map[string]LoginChallenge{}}
}

func (r *lockedLoginRepo) InTx(_ context.Context, fn func(Repository) error) error { return fn(r) }
func (r *lockedLoginRepo) LockEmailIdentity(context.Context, UserID) (LoginIdentity, bool, error) {
	r.calls = append(r.calls, "LockEmailIdentity")
	return r.locked, true, nil
}
func (r *lockedLoginRepo) ListAuthenticators(context.Context, UserID) ([]Authenticator, error) {
	return nil, nil
}
func (r *lockedLoginRepo) CreateSession(_ context.Context, s Session) error {
	r.sessions[s.TokenHash] = s
	return nil
}
func (r *lockedLoginRepo) DeleteSession(_ context.Context, h string) error {
	delete(r.sessions, h)
	return nil
}
func (r *lockedLoginRepo) DeleteExpiredLoginChallenges(context.Context, time.Time) error { return nil }
func (r *lockedLoginRepo) CreateLoginChallenge(_ context.Context, c LoginChallenge) error {
	r.challenges[c.TokenHash] = c
	return nil
}
func (r *lockedLoginRepo) GetConfigSection(context.Context, string) ([]byte, time.Time, bool, error) {
	return nil, time.Time{}, false, nil
}

// The password was verified against a hash that a reset has since replaced.
func TestALoginWhoseVerifiedHashWasReplacedBeforeTheSessionOpensGetsNone(t *testing.T) {
	repo := newLockedLoginRepo("old-password-1")
	verified := repo.locked
	repo.locked = newLockedLoginRepo("temporary-pass-9").locked
	_, err := NewService(repo, Options{}).openSession(context.Background(), User{ID: 7}, &verified)
	if e, ok := AsError(err); !ok || e.Kind != KindUnauthenticated || len(e.Errors) == 0 || e.Errors[0].Code != CodeInvalidCredentials {
		t.Fatalf("err = %v, want invalidCredentials", err)
	}
	if len(repo.sessions) != 0 || len(repo.challenges) != 0 {
		t.Fatalf("sessions %d, challenges %d, want none", len(repo.sessions), len(repo.challenges))
	}
}

func TestALoginWithTheVerifiedHashStillCurrentOpensASessionRestrictedAsTheLockedIdentityIs(t *testing.T) {
	repo := newLockedLoginRepo("old-password-1")
	verified := repo.locked
	repo.locked.MustChangePassword = false // the flag as it is now, under the lock
	res, err := NewService(repo, Options{}).openSession(context.Background(), User{ID: 7}, &verified)
	if err != nil || res.Token == "" || res.MustChangePassword {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if len(repo.sessions) != 1 || repo.calls[0] != "LockEmailIdentity" {
		t.Fatalf("sessions %d, calls %v", len(repo.sessions), repo.calls)
	}
}
