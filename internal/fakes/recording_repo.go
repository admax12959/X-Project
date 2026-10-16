package fakes

import (
	"context"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// RecordingRepo is the thin recording fake that ADR-0008 asks for: it answers the few reads a
// credential change makes and records every call in order, so a test can pin the lock order and
// what a change ends. It does not model rows or locks (the MySQL container tests do); a
// method it does not implement panics, so an unexpected call fails the test.
//
// Calls lists the method names. The reads and writes that stand for a step carry the name of the
// step: GetConfigSection is the notice reading the mail settings, AppendAudit is the audit entry.
type RecordingRepo struct {
	domain.Repository
	Calls []string

	UserID domain.UserID
	// Identity is what LockEmailIdentity returns; NoEmailIdentity makes it report none.
	Identity        domain.LoginIdentity
	NoEmailIdentity bool
	// IdentityGoneOnLock: the identity is there for the plain reads and gone when it is locked,
	// as when it is unbound between the two.
	IdentityGoneOnLock bool
	Token           domain.AuthToken // what LockAuthToken and FindAuthToken return
	// Sessions are the token hashes FindSession finds, for the user UserID.
	Sessions map[string]bool
	// SecondFactor: the user has an enabled authenticator.
	SecondFactor bool
	// Username and Email are the user's, as the command line finds them.
	Username, Email string
}

var _ domain.Repository = (*RecordingRepo)(nil)

func (r *RecordingRepo) rec(name string) { r.Calls = append(r.Calls, name) }

func (r *RecordingRepo) InTx(_ context.Context, fn func(domain.Repository) error) error { return fn(r) }

func (r *RecordingRepo) user() domain.User {
	return domain.User{ID: r.UserID, Username: r.Username, Email: r.Email, Status: domain.UserEnabled}
}

func (r *RecordingRepo) FindUserByUsername(_ context.Context, name string) (domain.User, bool, error) {
	r.rec("FindUserByUsername")
	return r.user(), name == r.Username, nil
}

func (r *RecordingRepo) GetUser(_ context.Context, id domain.UserID) (domain.User, bool, error) {
	r.rec("GetUser")
	return r.user(), true, nil
}

func (r *RecordingRepo) LockUserWithIdentities(context.Context, domain.UserID) (domain.UserWithIdentities, bool, error) {
	r.rec("LockUserWithIdentities")
	cur := domain.UserWithIdentities{User: r.user(), SecondFactorEnabled: r.SecondFactor}
	if !r.NoEmailIdentity {
		cur.Identities = []domain.LoginIdentity{r.Identity}
	}
	return cur, true, nil
}

func (r *RecordingRepo) LockEmailIdentity(context.Context, domain.UserID) (domain.LoginIdentity, bool, error) {
	r.rec("LockEmailIdentity")
	if r.NoEmailIdentity || r.IdentityGoneOnLock {
		return domain.LoginIdentity{}, false, nil
	}
	return r.Identity, true, nil
}

func (r *RecordingRepo) FindAuthToken(context.Context, string) (domain.AuthToken, bool, error) {
	r.rec("FindAuthToken")
	return r.Token, true, nil
}

func (r *RecordingRepo) LockAuthToken(context.Context, string) (domain.AuthToken, bool, error) {
	r.rec("LockAuthToken")
	return r.Token, true, nil
}

func (r *RecordingRepo) FindSession(_ context.Context, hash string) (domain.Session, bool, error) {
	r.rec("FindSession")
	return domain.Session{TokenHash: hash, UserID: r.UserID}, r.Sessions[hash], nil
}

func (r *RecordingRepo) ListAuthenticators(context.Context, domain.UserID) ([]domain.Authenticator, error) {
	r.rec("ListAuthenticators")
	if !r.SecondFactor {
		return nil, nil
	}
	return []domain.Authenticator{{Type: domain.AuthenticatorTOTP, Enabled: true}}, nil
}

func (r *RecordingRepo) GetConfigSection(context.Context, string) ([]byte, time.Time, bool, error) {
	r.rec("GetConfigSection")
	return nil, time.Time{}, false, nil
}

func (r *RecordingRepo) ChangePassword(context.Context, domain.UserID, string) error {
	r.rec("ChangePassword")
	return nil
}

func (r *RecordingRepo) SetTemporaryPassword(context.Context, domain.UserID, string) error {
	r.rec("SetTemporaryPassword")
	return nil
}

func (r *RecordingRepo) ClearSessionMustChangePassword(context.Context, string) error {
	r.rec("ClearSessionMustChangePassword")
	return nil
}

func (r *RecordingRepo) UseUserAuthTokens(context.Context, domain.UserID, []domain.TokenPurpose, time.Time) error {
	r.rec("UseUserAuthTokens")
	return nil
}

func (r *RecordingRepo) DeleteUserLoginChallenges(context.Context, domain.UserID) error {
	r.rec("DeleteUserLoginChallenges")
	return nil
}

func (r *RecordingRepo) DeleteUserSessions(context.Context, domain.UserID) error {
	r.rec("DeleteUserSessions")
	return nil
}

func (r *RecordingRepo) DeleteOtherUserSessions(context.Context, domain.UserID, string) error {
	r.rec("DeleteOtherUserSessions")
	return nil
}

func (r *RecordingRepo) DeleteAuthenticatorsAndRecoveryCodes(context.Context, domain.UserID) error {
	r.rec("DeleteAuthenticatorsAndRecoveryCodes")
	return nil
}

func (r *RecordingRepo) PutSecondFactorLockout(context.Context, domain.UserID, domain.SecondFactorLockout) error {
	r.rec("PutSecondFactorLockout")
	return nil
}

func (r *RecordingRepo) AppendAudit(_ context.Context, e domain.StoredAuditEntry) error {
	r.rec("AppendAudit:" + string(e.Action))
	return nil
}
