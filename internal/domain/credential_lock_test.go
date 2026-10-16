package domain

import (
	"context"
	"slices"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// lockRepo plays the part of the repository that the credential-changing transactions touch,
// and records the order of the calls. The identity the plain read returns (stale) can differ
// from the one the lock returns (locked): that is a reset committing between the two reads.
type lockRepo struct {
	Repository
	locked     LoginIdentity
	stale      LoginIdentity
	sessions   map[string]Session
	challenges map[string]LoginChallenge
	calls      []string
}

func newLockRepo(password string) *lockRepo {
	h, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	id := LoginIdentity{UserID: 7, PasswordHash: string(h), MustChangePassword: true}
	return &lockRepo{locked: id, stale: id, sessions: map[string]Session{}, challenges: map[string]LoginChallenge{}}
}

func (r *lockRepo) InTx(_ context.Context, fn func(Repository) error) error { return fn(r) }
func (r *lockRepo) LockEmailIdentity(context.Context, UserID) (LoginIdentity, bool, error) {
	r.calls = append(r.calls, "LockEmailIdentity")
	return r.locked, true, nil
}
func (r *lockRepo) FindIdentityByUser(context.Context, UserID, IdentityKind) (LoginIdentity, bool, error) {
	r.calls = append(r.calls, "FindIdentityByUser")
	return r.stale, true, nil
}
func (r *lockRepo) FindSession(_ context.Context, h string) (Session, bool, error) {
	r.calls = append(r.calls, "FindSession")
	s, ok := r.sessions[h]
	return s, ok, nil
}
func (r *lockRepo) ChangePassword(context.Context, UserID, string) error {
	r.calls = append(r.calls, "ChangePassword")
	return nil
}
func (r *lockRepo) ClearSessionMustChangePassword(context.Context, string) error { return nil }
func (r *lockRepo) DeleteUserLoginChallenges(context.Context, UserID) error {
	r.calls = append(r.calls, "DeleteUserLoginChallenges")
	return nil
}
func (r *lockRepo) DeleteOtherUserSessions(context.Context, UserID, string) error {
	r.calls = append(r.calls, "DeleteOtherUserSessions")
	return nil
}
func (r *lockRepo) ListAuthenticators(context.Context, UserID) ([]Authenticator, error) {
	return nil, nil
}
func (r *lockRepo) CreateSession(_ context.Context, s Session) error {
	r.sessions[s.TokenHash] = s
	return nil
}
func (r *lockRepo) DeleteSession(_ context.Context, h string) error {
	delete(r.sessions, h)
	return nil
}
func (r *lockRepo) DeleteExpiredLoginChallenges(context.Context, time.Time) error { return nil }
func (r *lockRepo) CreateLoginChallenge(_ context.Context, c LoginChallenge) error {
	r.challenges[c.TokenHash] = c
	return nil
}
func (r *lockRepo) GetConfigSection(context.Context, string) ([]byte, time.Time, bool, error) {
	return nil, time.Time{}, false, nil
}

func changeActor() Actor { return Actor{User: User{ID: 7}} }

func (r *lockRepo) withSession(token string) {
	r.sessions[hashToken(token)] = Session{TokenHash: hashToken(token), UserID: 7}
}

func TestChangePasswordLocksTheIdentityFirstAndEndsChallengesBeforeSessions(t *testing.T) {
	repo := newLockRepo("old-password-1")
	repo.withSession("tok")
	err := NewService(repo, Options{}).ChangePassword(context.Background(), changeActor(), "tok", "old-password-1", "new-password-22")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"LockEmailIdentity", "FindSession", "ChangePassword", "DeleteUserLoginChallenges", "DeleteOtherUserSessions"}
	var got []string
	for _, c := range repo.calls {
		if slices.Contains(want, c) {
			got = append(got, c)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
}

// The password is checked against the hash as it is under the lock: a reset that committed
// after a plain read must not be overwritten by a change that knew the old password.
func TestChangePasswordChecksTheCurrentPasswordAgainstTheLockedHash(t *testing.T) {
	repo := newLockRepo("old-password-1")
	repo.withSession("tok")
	reset := newLockRepo("temporary-pass-9")
	repo.locked = reset.locked // a reset landed after the plain read
	err := NewService(repo, Options{}).ChangePassword(context.Background(), changeActor(), "tok", "old-password-1", "new-password-22")
	if e, ok := AsError(err); !ok || e.Kind != KindValidation {
		t.Fatalf("err = %v, want the current password refused", err)
	}
	if slices.Contains(repo.calls, "ChangePassword") {
		t.Fatalf("the password was written: %v", repo.calls)
	}
}

// A reset deletes the sessions: a request from a session that no longer exists changes nothing.
func TestChangePasswordNeedsItsSessionToStillExist(t *testing.T) {
	repo := newLockRepo("old-password-1")
	err := NewService(repo, Options{}).ChangePassword(context.Background(), changeActor(), "tok", "old-password-1", "new-password-22")
	if !isUnauthenticated(err) {
		t.Fatalf("err = %v, want unauthenticated", err)
	}
	if slices.Contains(repo.calls, "ChangePassword") {
		t.Fatalf("the password was written: %v", repo.calls)
	}
}

// Login: the password was verified against a hash that a reset has since replaced.
func TestALoginWhoseVerifiedHashWasReplacedBeforeTheSessionOpensGetsNone(t *testing.T) {
	repo := newLockRepo("old-password-1")
	verified := repo.locked
	reset := newLockRepo("temporary-pass-9")
	repo.locked = reset.locked
	_, err := NewService(repo, Options{}).openSession(context.Background(), User{ID: 7}, &verified)
	if e, ok := AsError(err); !ok || e.Kind != KindUnauthenticated || len(e.Errors) == 0 || e.Errors[0].Code != CodeInvalidCredentials {
		t.Fatalf("err = %v, want invalidCredentials", err)
	}
	if len(repo.sessions) != 0 || len(repo.challenges) != 0 {
		t.Fatalf("sessions %d, challenges %d, want none", len(repo.sessions), len(repo.challenges))
	}
}

func TestALoginWithTheVerifiedHashStillCurrentOpensASessionRestrictedAsTheLockedIdentityIs(t *testing.T) {
	repo := newLockRepo("old-password-1")
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
