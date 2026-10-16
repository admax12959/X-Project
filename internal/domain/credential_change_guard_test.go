package domain

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// A caller that forgot the revoke policy, or asks to keep a session without naming it, is
// refused before any lock is taken. The embedded Repository is nil, so a call to it panics.
func TestChangeCredentialRefusesAnIncompleteRequest(t *testing.T) {
	apply := func(lockedCredential) error { return nil }
	for name, ch := range map[string]credentialChange{
		"no policy":          {User: 7, Apply: apply},
		"others, no session": {User: 7, Apply: apply, Revoke: RevokeOthers},
		"no change":          {User: 7, Revoke: RevokeAll},
	} {
		t.Run(name, func(t *testing.T) {
			var tx struct{ Repository }
			if err := NewService(nil, Options{}).changeCredential(context.Background(), tx, ch); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

// stepRepo records the calls changeCredential makes. Mail works (noticeRepo), so a notice goes as
// far as the insert; the errors say where each step fails.
type stepRepo struct {
	noticeRepo
	calls             []string
	noIdentity        bool
	lockErr, auditErr error
	identity          LoginIdentity
	token             AuthToken
	tokenFound        bool
}

func (r *stepRepo) rec(c string) { r.calls = append(r.calls, c) }
func (r *stepRepo) LockEmailIdentity(context.Context, UserID) (LoginIdentity, bool, error) {
	r.rec("LockEmailIdentity")
	return r.identity, !r.noIdentity, r.lockErr
}
func (r *stepRepo) LockAuthToken(context.Context, string) (AuthToken, bool, error) {
	r.rec("LockAuthToken")
	return r.token, r.tokenFound, nil
}
func (r *stepRepo) UseUserAuthTokens(context.Context, UserID, []TokenPurpose, time.Time) error {
	r.rec("UseUserAuthTokens")
	return nil
}
func (r *stepRepo) DeleteUserLoginChallenges(context.Context, UserID) error {
	r.rec("DeleteUserLoginChallenges")
	return nil
}
func (r *stepRepo) DeleteUserSessions(context.Context, UserID) error {
	r.rec("DeleteUserSessions")
	return nil
}
func (r *stepRepo) DeleteOtherUserSessions(context.Context, UserID, string) error {
	r.rec("DeleteOtherUserSessions")
	return nil
}
func (r *stepRepo) AppendAudit(context.Context, StoredAuditEntry) error {
	r.rec("AppendAudit")
	return r.auditErr
}
func (r *stepRepo) InsertMail(ctx context.Context, m MailEntry) (int64, error) {
	r.rec("InsertMail")
	return r.noticeRepo.InsertMail(ctx, m)
}

func runChange(r *stepRepo, ch credentialChange) error {
	return NewService(r, Options{}).changeCredential(context.Background(), r, ch)
}

func noop(lockedCredential) error { return nil }

var errStep = errors.New("fake: step failed")

func wantCalls(t *testing.T, r *stepRepo, want ...string) {
	t.Helper()
	if !slices.Equal(r.calls, want) {
		t.Fatalf("calls =\n  %v\nwant\n  %v", r.calls, want)
	}
}

// A change that is refused ends nothing: not the tokens, not the challenges, not the sessions,
// and it leaves no audit entry or notice.
func TestChangeCredentialStopsWhereTheChangeIsRefused(t *testing.T) {
	r := &stepRepo{}
	err := runChange(r, credentialChange{
		User: 7, Revoke: RevokeAll, SpendTokens: passwordTokenPurposes,
		Apply:  func(lockedCredential) error { return errStep },
		Audit:  &credentialAudit{Event: passwordResetAudit(7)},
		Notice: &securityNotice{To: "u@example.com", Template: MailTemplatePasswordChangedByOperator},
	})
	if !errors.Is(err, errStep) {
		t.Fatalf("err = %v, want the refusal", err)
	}
	wantCalls(t, r, "LockEmailIdentity")
}

func TestChangeCredentialStopsWhenTheLockFails(t *testing.T) {
	r := &stepRepo{lockErr: errStep}
	called := false
	err := runChange(r, credentialChange{User: 7, Revoke: RevokeAll, Token: "h", Apply: func(lockedCredential) error { called = true; return nil }})
	if !errors.Is(err, errStep) || called {
		t.Fatalf("err = %v, applied = %v, want the lock error and no change", err, called)
	}
	wantCalls(t, r, "LockEmailIdentity")
}

// The caller judges what was locked, so a missing identity or token reaches Apply as it is, and
// the token is locked even when there is no identity (the order of the checks is the caller's).
func TestChangeCredentialHandsOverWhatWasLockedUnjudged(t *testing.T) {
	r := &stepRepo{noIdentity: true, token: AuthToken{ID: 3}, tokenFound: true}
	var got lockedCredential
	err := runChange(r, credentialChange{User: 7, Revoke: RevokeAll, Token: "h", Apply: func(l lockedCredential) error { got = l; return errStep }})
	if !errors.Is(err, errStep) {
		t.Fatalf("err = %v", err)
	}
	if got.HasEmail || !got.TokenFound || got.Token.ID != 3 {
		t.Fatalf("locked = %+v, want no identity and the token", got)
	}
	wantCalls(t, r, "LockEmailIdentity", "LockAuthToken")
}

// RevokeNone is the second-factor resets' policy: the tokens may still be spent, but the
// challenges and the sessions are left alone.
func TestChangeCredentialRevokeNoneLeavesChallengesAndSessions(t *testing.T) {
	r := &stepRepo{}
	if err := runChange(r, credentialChange{User: 7, Revoke: RevokeNone, SpendTokens: passwordTokenPurposes, Apply: noop}); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, r, "LockEmailIdentity", "UseUserAuthTokens")
}

// Without an audit entry or a notice the module writes neither; with both, the entry goes first.
func TestChangeCredentialAuditsBeforeItNotifies(t *testing.T) {
	plain := &stepRepo{}
	if err := runChange(plain, credentialChange{User: 7, Revoke: RevokeAll, Apply: noop}); err != nil {
		t.Fatal(err)
	}
	wantCalls(t, plain, "LockEmailIdentity", "DeleteUserLoginChallenges", "DeleteUserSessions")

	full := &stepRepo{}
	err := runChange(full, credentialChange{
		User: 7, Revoke: RevokeOthers, KeepSession: "keep", Apply: noop,
		Audit:  &credentialAudit{Event: passwordResetAudit(7)},
		Notice: &securityNotice{To: "u@example.com", Template: MailTemplatePasswordChangedByOperator},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCalls(t, full, "LockEmailIdentity", "DeleteUserLoginChallenges", "DeleteOtherUserSessions", "AppendAudit", "InsertMail")
}

func TestChangeCredentialStopsWhenTheAuditFails(t *testing.T) {
	r := &stepRepo{auditErr: errStep}
	err := runChange(r, credentialChange{
		User: 7, Revoke: RevokeAll, Apply: noop,
		Audit:  &credentialAudit{Event: passwordResetAudit(7)},
		Notice: &securityNotice{To: "u@example.com", Template: MailTemplatePasswordChangedByOperator},
	})
	if !errors.Is(err, errStep) {
		t.Fatalf("err = %v, want the audit error", err)
	}
	wantCalls(t, r, "LockEmailIdentity", "DeleteUserLoginChallenges", "DeleteUserSessions", "AppendAudit")
}

// The notice is best effort (a statement that fails is swallowed) unless the transaction is lost:
// then the change fails with it.
func TestChangeCredentialNoticeFailure(t *testing.T) {
	notice := &securityNotice{To: "u@example.com", Template: MailTemplatePasswordChangedByOperator}
	swallowed := &stepRepo{noticeRepo: noticeRepo{insertErr: errors.New("fake: field has no default value (1364)")}}
	if err := runChange(swallowed, credentialChange{User: 7, Revoke: RevokeAll, Apply: noop, Notice: notice}); err != nil {
		t.Fatalf("err = %v, want the statement failure swallowed", err)
	}
	lost := &stepRepo{noticeRepo: noticeRepo{insertErr: errFakeDeadlock}}
	if err := runChange(lost, credentialChange{User: 7, Revoke: RevokeAll, Apply: noop, Notice: notice}); !errors.Is(err, errFakeDeadlock) {
		t.Fatalf("err = %v, want the lost transaction returned", err)
	}
}
