package domain

import (
	"context"
	"slices"
	"testing"
	"time"
)

// tokenRepo records the order in which SetPasswordWithToken takes its locks. lockedToken is what
// the locking read returns, the plain read returns plainToken.
type tokenRepo struct {
	Repository
	plainToken  AuthToken
	lockedToken AuthToken
	calls       []string
}

func (r *tokenRepo) InTx(_ context.Context, fn func(Repository) error) error { return fn(r) }
func (r *tokenRepo) FindAuthToken(context.Context, string) (AuthToken, bool, error) {
	r.calls = append(r.calls, "FindAuthToken")
	return r.plainToken, true, nil
}
func (r *tokenRepo) LockAuthToken(context.Context, string) (AuthToken, bool, error) {
	r.calls = append(r.calls, "LockAuthToken")
	return r.lockedToken, true, nil
}
func (r *tokenRepo) LockEmailIdentity(_ context.Context, id UserID) (LoginIdentity, bool, error) {
	r.calls = append(r.calls, "LockEmailIdentity")
	return LoginIdentity{UserID: id, LoginName: "a@example.com"}, true, nil
}
func (r *tokenRepo) GetUser(_ context.Context, id UserID) (User, bool, error) {
	return User{ID: id, Status: UserEnabled}, true, nil
}
func (r *tokenRepo) ChangePassword(context.Context, UserID, string) error {
	r.calls = append(r.calls, "ChangePassword")
	return nil
}
func (r *tokenRepo) UseUserAuthTokens(context.Context, UserID, []TokenPurpose, time.Time) error {
	return nil
}
func (r *tokenRepo) DeleteUserLoginChallenges(context.Context, UserID) error { return nil }
func (r *tokenRepo) DeleteUserSessions(context.Context, UserID) error        { return nil }

func newTokenRepo() *tokenRepo {
	tok := AuthToken{ID: 1, UserID: 7, Purpose: TokenPasswordReset, LoginEmail: "a@example.com", ExpiresAt: time.Now().Add(time.Hour)}
	return &tokenRepo{plainToken: tok, lockedToken: tok}
}

// SendInvitation, a login email change and the reset CLI lock the user and then write the
// tokens; redeeming a token must take its locks in the same order or the two deadlock.
func TestSetPasswordWithTokenLocksTheIdentityBeforeTheToken(t *testing.T) {
	repo := newTokenRepo()
	if err := NewService(repo, Options{}).SetPasswordWithToken(context.Background(), "tok", "new-password-22"); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range repo.calls {
		if c != "FindAuthToken" {
			got = append(got, c)
		}
	}
	if want := []string{"LockEmailIdentity", "LockAuthToken", "ChangePassword"}; !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
}

// The user is taken from the plain read; once the locks are held the token is judged again.
func TestSetPasswordWithTokenJudgesTheTokenAgainUnderTheLocks(t *testing.T) {
	spent := newTokenRepo()
	spent.lockedToken.UsedAt = time.Now()
	other := newTokenRepo()
	other.lockedToken.UserID = 8
	for name, repo := range map[string]*tokenRepo{"spent": spent, "other user": other} {
		err := NewService(repo, Options{}).SetPasswordWithToken(context.Background(), "tok", "new-password-22")
		if e, ok := AsError(err); !ok || e.Kind != KindValidation {
			t.Errorf("%s: err = %v, want the token refused", name, err)
		}
		if slices.Contains(repo.calls, "ChangePassword") {
			t.Errorf("%s: the password was written", name)
		}
	}
}
