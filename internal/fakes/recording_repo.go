package fakes

import (
	"context"
	"slices"
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
	// StaleIdentity is what the plain read FindIdentityByUser returns, when the lock has seen a
	// later one (a reset committing between the two reads). Nil: the same as Identity.
	StaleIdentity *domain.LoginIdentity
	// OtherIdentities are the user's identities besides the email one, as LockUserWithIdentities
	// lists them.
	OtherIdentities []domain.LoginIdentity
	Token           domain.AuthToken // what LockAuthToken and FindAuthToken return
	// LockedToken is what LockAuthToken returns when the plain read FindAuthToken saw another
	// state of the token. Nil: the same as Token.
	LockedToken *domain.AuthToken
	// Sessions are the token hashes FindSession finds, for the user UserID.
	Sessions map[string]bool
	// SecondFactor: the user has an enabled authenticator.
	SecondFactor bool
	// Username and Email are the user's, as the command line finds them.
	Username, Email string
	// MailWorks: the mail settings are complete, so a notice is queued (InsertMail). Otherwise
	// the notice stops after reading the settings.
	MailWorks bool
	// Authenticators are the pending ones (ReplacePendingAuthenticator stores one here); the
	// enabled one that SecondFactor stands for is added to them.
	Authenticators []domain.Authenticator
	// RecoveryCodes are the unspent ones, with the hashes the service computes.
	RecoveryCodes []domain.RecoveryCode
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
	cur.Identities = append(cur.Identities, r.OtherIdentities...)
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
	if r.LockedToken != nil {
		return *r.LockedToken, true, nil
	}
	return r.Token, true, nil
}

func (r *RecordingRepo) FindIdentityByUser(_ context.Context, _ domain.UserID, kind domain.IdentityKind) (domain.LoginIdentity, bool, error) {
	r.rec("FindIdentityByUser")
	if kind != domain.IdentityEmail || r.NoEmailIdentity {
		return domain.LoginIdentity{}, false, nil
	}
	if r.StaleIdentity != nil {
		return *r.StaleIdentity, true, nil
	}
	return r.Identity, true, nil
}

// FindIdentity finds no identity: the login name asked for is free.
func (r *RecordingRepo) FindIdentity(context.Context, domain.IdentityKind, string) (domain.LoginIdentity, bool, error) {
	r.rec("FindIdentity")
	return domain.LoginIdentity{}, false, nil
}

func (r *RecordingRepo) FindSession(_ context.Context, hash string) (domain.Session, bool, error) {
	r.rec("FindSession")
	return domain.Session{TokenHash: hash, UserID: r.UserID}, r.Sessions[hash], nil
}

func (r *RecordingRepo) ListAuthenticators(context.Context, domain.UserID) ([]domain.Authenticator, error) {
	r.rec("ListAuthenticators")
	out := slices.Clone(r.Authenticators)
	if r.SecondFactor {
		out = append(out, domain.Authenticator{Type: domain.AuthenticatorTOTP, Enabled: true})
	}
	return out, nil
}

func (r *RecordingRepo) ReplacePendingAuthenticator(_ context.Context, a domain.Authenticator) error {
	r.rec("ReplacePendingAuthenticator")
	a.ID = 1
	r.Authenticators = []domain.Authenticator{a}
	return nil
}

func (r *RecordingRepo) EnableAuthenticator(context.Context, int64, int64, time.Time) (bool, error) {
	r.rec("EnableAuthenticator")
	return true, nil
}

func (r *RecordingRepo) LockRateKeys(context.Context, ...string) error {
	r.rec("LockRateKeys")
	return nil
}

func (r *RecordingRepo) GetSecondFactorLockout(context.Context, domain.UserID) (domain.SecondFactorLockout, error) {
	r.rec("GetSecondFactorLockout")
	return domain.SecondFactorLockout{}, nil
}

func (r *RecordingRepo) ListUnusedRecoveryCodes(context.Context, domain.UserID) ([]domain.RecoveryCode, error) {
	r.rec("ListUnusedRecoveryCodes")
	return r.RecoveryCodes, nil
}

func (r *RecordingRepo) ClaimRecoveryCode(context.Context, int64, time.Time) (bool, error) {
	r.rec("ClaimRecoveryCode")
	return true, nil
}

func (r *RecordingRepo) ReplaceRecoveryCodes(context.Context, domain.UserID, []string, time.Time) error {
	r.rec("ReplaceRecoveryCodes")
	return nil
}

func (r *RecordingRepo) CountRateEvents(context.Context, string, time.Time) (int, error) {
	r.rec("CountRateEvents")
	return 0, nil
}

func (r *RecordingRepo) CreateIdentity(context.Context, domain.LoginIdentity) error {
	r.rec("CreateIdentity")
	return nil
}

func (r *RecordingRepo) DeleteIdentity(_ context.Context, _ domain.UserID, kind domain.IdentityKind) error {
	r.rec("DeleteIdentity:" + string(kind))
	return nil
}

func (r *RecordingRepo) RenameEmailLogin(context.Context, domain.UserID, string) error {
	r.rec("RenameEmailLogin")
	return nil
}

func (r *RecordingRepo) UpdateUser(context.Context, domain.User, time.Time) error {
	r.rec("UpdateUser")
	return nil
}

// GetConfigSection answers the mail settings only (the security policy and the rest are
// unset). With MailWorks they are complete, which is what lets a notice be queued.
func (r *RecordingRepo) GetConfigSection(_ context.Context, section string) ([]byte, time.Time, bool, error) {
	r.rec("GetConfigSection")
	if r.MailWorks && section == domain.SectionSMTP {
		return []byte(`{"enabled":true,"host":"smtp.example.com","from":"x@example.com"}`), time.Time{}, true, nil
	}
	return nil, time.Time{}, false, nil
}

func (r *RecordingRepo) ListConfigSecrets(context.Context, string) (map[string]domain.StoredSecret, error) {
	return nil, nil
}

// InsertMail records the notice: its template and the address it goes to.
func (r *RecordingRepo) InsertMail(_ context.Context, e domain.MailEntry) (int64, error) {
	r.rec("InsertMail:" + e.Template + ">" + e.To)
	return 1, nil
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
