package domain_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/fakes"
)

// Every call site that goes through the credential change module, each driven through its
// exported entry point against the recording repository. The whole recorded sequence is
// compared: it pins the lock order (user and identity, then the token, then the challenges, then
// the sessions) and the revoke policy (which delete runs, whether tokens are spent), and that an
// audit entry and a notice appear only where they do today.
//
// The credential_lock_test.go and token_lock_order_test.go tests that this replaces asserted:
// the identity is locked before the token and before the session is read; challenges end before
// sessions; the current password is judged against the hash as it is under the lock (see the
// stale identity in TestCredentialChangeRefusals); and a token is judged again under the locks.
func TestCredentialChangeCallSites(t *testing.T) {
	const old, next = "old-password-1", "new-password-22"
	hash, err := bcrypt.GenerateFromPassword([]byte(old), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	const recoveryCode = "ABCD-EFGH-JKLM"
	newRepo := func() *fakes.RecordingRepo {
		return &fakes.RecordingRepo{
			UserID:   7,
			Username: "alice", Email: "a@example.com",
			Identity:      domain.LoginIdentity{UserID: 7, Kind: domain.IdentityEmail, LoginName: "a@example.com", PasswordHash: string(hash), MustChangePassword: true},
			RecoveryCodes: []domain.RecoveryCode{{ID: 1, Hash: recoveryHash(7, recoveryCode)}},
			Token: domain.AuthToken{
				ID: 1, UserID: 7, Purpose: domain.TokenPasswordReset, LoginEmail: "a@example.com",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			Sessions:     map[string]bool{tokenHash("tok"): true},
			SecondFactor: true,
		}
	}
	user := domain.Actor{User: domain.User{ID: 7, Email: "a@example.com"}}
	admin := domain.Actor{User: domain.User{ID: 1, IsAdmin: true}}
	newLogin := "new@example.com"
	var totp string // the code of the authenticator that prepare enrols
	cases := []struct {
		name    string
		setup   func(*fakes.RecordingRepo)
		prepare func(*domain.Service, *fakes.RecordingRepo) // runs first; its calls are not compared
		run     func(s *domain.Service) error
		want    []string
	}{
		{
			name: "ChangePassword: others",
			run: func(s *domain.Service) error {
				return s.ChangePassword(context.Background(), domain.Actor{User: domain.User{ID: 7}}, "tok", old, next)
			},
			want: []string{
				"LockEmailIdentity", "FindSession", "ChangePassword", "ClearSessionMustChangePassword",
				"DeleteUserLoginChallenges", "DeleteOtherUserSessions",
			},
		},
		{
			name: "SetPasswordWithToken: all, and the tokens spent",
			run: func(s *domain.Service) error {
				return s.SetPasswordWithToken(context.Background(), "tok", next)
			},
			want: []string{
				"FindAuthToken", "LockEmailIdentity", "LockAuthToken", "GetUser", "ChangePassword",
				"UseUserAuthTokens", "DeleteUserLoginChallenges", "DeleteUserSessions",
			},
		},
		{
			name: "CLI password reset: all, audited, noticed",
			run: func(s *domain.Service) error {
				_, err := s.ResetUserByCLI(context.Background(), domain.CLIResetInput{User: "alice", Password: true})
				return err
			},
			want: []string{
				"FindUserByUsername", "LockUserWithIdentities", "LockEmailIdentity", "SetTemporaryPassword",
				"UseUserAuthTokens", "DeleteUserLoginChallenges", "DeleteUserSessions",
				"AppendAudit:" + string(domain.AuditPasswordReset), "GetConfigSection",
			},
		},
		{
			name: "CLI password and second factor reset: the factor goes first, none: the sessions stay until the password",
			run: func(s *domain.Service) error {
				_, err := s.ResetUserByCLI(context.Background(), domain.CLIResetInput{User: "alice", Password: true, SecondFactor: true})
				return err
			},
			want: []string{
				"FindUserByUsername", "LockUserWithIdentities", "LockEmailIdentity",
				"ListAuthenticators", "LockEmailIdentity", "DeleteAuthenticatorsAndRecoveryCodes", "PutSecondFactorLockout",
				"AppendAudit:" + string(domain.AuditSecondFactorReset), "GetConfigSection",
				"SetTemporaryPassword", "UseUserAuthTokens", "DeleteUserLoginChallenges", "DeleteUserSessions",
				"AppendAudit:" + string(domain.AuditPasswordReset), "GetConfigSection",
			},
		},
		{
			name: "CLI second factor reset alone: none",
			run: func(s *domain.Service) error {
				_, err := s.ResetUserByCLI(context.Background(), domain.CLIResetInput{User: "alice", SecondFactor: true})
				return err
			},
			want: []string{
				"FindUserByUsername", "LockUserWithIdentities",
				"ListAuthenticators", "LockEmailIdentity", "DeleteAuthenticatorsAndRecoveryCodes", "PutSecondFactorLockout",
				"AppendAudit:" + string(domain.AuditSecondFactorReset), "GetConfigSection",
			},
		},
		{
			name: "administrator second factor reset: none (the sessions stay), audited, noticed",
			run: func(s *domain.Service) error {
				_, err := s.ResetSecondFactor(context.Background(), admin, 7)
				return err
			},
			want: []string{
				"LockUserWithIdentities", "ListAuthenticators", "LockEmailIdentity",
				"DeleteAuthenticatorsAndRecoveryCodes", "PutSecondFactorLockout",
				"AppendAudit:" + string(domain.AuditSecondFactorReset), "GetConfigSection",
			},
		},
		{
			name:  "ConfirmTOTP: others, and the challenges; the failure count is read under the locks",
			setup: func(r *fakes.RecordingRepo) { r.SecondFactor = false },
			prepare: func(s *domain.Service, r *fakes.RecordingRepo) {
				setup, err := s.StartTOTPSetup(context.Background(), user, old)
				if err != nil {
					t.Fatal(err)
				}
				totp = totpNow(t, setup.Secret)
			},
			run: func(s *domain.Service) error {
				_, err := s.ConfirmTOTP(context.Background(), user, "tok", totp)
				return err
			},
			want: []string{
				"LockRateKeys", "LockEmailIdentity", "GetSecondFactorLockout", "ListAuthenticators",
				"EnableAuthenticator", "ReplaceRecoveryCodes",
				"DeleteUserLoginChallenges", "DeleteOtherUserSessions",
				"AppendAudit:" + string(domain.AuditSecondFactorEnabled), "GetConfigSection",
			},
		},
		{
			name: "DisableSecondFactor: none, audited, noticed",
			run: func(s *domain.Service) error {
				return s.DisableSecondFactor(context.Background(), user, recoveryCode)
			},
			want: []string{
				"ListAuthenticators", "GetConfigSection", // enabled? enforced?
				"ListAuthenticators", "LockRateKeys", "GetSecondFactorLockout", "ListUnusedRecoveryCodes", "ClaimRecoveryCode", // the proof
				"LockEmailIdentity", "ListAuthenticators", "DeleteAuthenticatorsAndRecoveryCodes", "PutSecondFactorLockout",
				"AppendAudit:" + string(domain.AuditSecondFactorDisabled), "GetConfigSection",
			},
		},
		{
			name: "RegenerateRecoveryCodes: none, audited, noticed",
			run: func(s *domain.Service) error {
				_, err := s.RegenerateRecoveryCodes(context.Background(), user, recoveryCode)
				return err
			},
			want: []string{
				"ListAuthenticators", "LockRateKeys", "GetSecondFactorLockout", "ListUnusedRecoveryCodes", "ClaimRecoveryCode", // the proof
				"LockEmailIdentity", "ListAuthenticators", "ReplaceRecoveryCodes",
				"AppendAudit:" + string(domain.AuditRecoveryCodesRegenerated), "GetConfigSection",
			},
		},
		{
			name:  "login email change by an administrator: none, the tokens spent, the old address and the contact email told",
			setup: func(r *fakes.RecordingRepo) { r.MailWorks = true; r.Email = "contact@example.com" },
			run: func(s *domain.Service) error {
				_, err := s.UpdateUser(context.Background(), admin, 7, domain.UpdateUserInput{LoginEmail: &newLogin})
				return err
			},
			want: []string{
				"LockUserWithIdentities", "UpdateUser",
				"LockEmailIdentity", "RenameEmailLogin", "UseUserAuthTokens",
				"AppendAudit:" + string(domain.AuditLoginEmailChanged),
				"GetConfigSection", "GetConfigSection", "GetUser", "InsertMail:" + string(domain.MailTemplateLoginEmailChanged) + ">a@example.com",
				"GetConfigSection", "GetConfigSection", "GetUser", "InsertMail:" + string(domain.MailTemplateLoginEmailChanged) + ">contact@example.com",
			},
		},
		{
			name:  "login email change by an administrator: the contact email is the old address, told once",
			setup: func(r *fakes.RecordingRepo) { r.MailWorks = true },
			run: func(s *domain.Service) error {
				_, err := s.UpdateUser(context.Background(), admin, 7, domain.UpdateUserInput{LoginEmail: &newLogin})
				return err
			},
			want: []string{
				"LockUserWithIdentities", "UpdateUser",
				"LockEmailIdentity", "RenameEmailLogin", "UseUserAuthTokens",
				"AppendAudit:" + string(domain.AuditLoginEmailChanged),
				"GetConfigSection", "GetConfigSection", "GetUser", "InsertMail:" + string(domain.MailTemplateLoginEmailChanged) + ">a@example.com",
			},
		},
		{
			name: "login email change by the user: the same module, the same steps",
			setup: func(r *fakes.RecordingRepo) {
				r.MailWorks = true
				r.Token.Purpose = domain.TokenChangeEmail // what the code the user typed is
			},
			run: func(s *domain.Service) error {
				_, err := s.ChangeOwnLoginEmail(context.Background(), user, newLogin, "123456")
				return err
			},
			want: []string{
				"GetConfigSection", // mail works?
				"LockUserWithIdentities", "FindIdentity", "CountRateEvents", "LockAuthToken", "UseUserAuthTokens",
				"UpdateUser",
				"LockEmailIdentity", "RenameEmailLogin", "UseUserAuthTokens",
				"AppendAudit:" + string(domain.AuditLoginEmailChanged),
				"GetConfigSection", "GetConfigSection", "GetUser", "InsertMail:" + string(domain.MailTemplateLoginEmailChanged) + ">a@example.com",
			},
		},
		{
			name: "unbind of the email identity: none, the password links spent, audited, noticed",
			setup: func(r *fakes.RecordingRepo) {
				r.OtherIdentities = []domain.LoginIdentity{{UserID: 7, Kind: domain.IdentityLDAP, LoginName: "alice"}}
			},
			run: func(s *domain.Service) error {
				_, err := s.UnbindOwnIdentity(context.Background(), user, "email")
				return err
			},
			want: []string{
				"LockUserWithIdentities", "LockEmailIdentity", "DeleteIdentity:email", "UseUserAuthTokens",
				"AppendAudit:" + string(domain.AuditIdentityUnbound), "GetConfigSection",
			},
		},
		{
			name: "unbind of the directory identity: no tokens spent",
			setup: func(r *fakes.RecordingRepo) {
				r.OtherIdentities = []domain.LoginIdentity{{UserID: 7, Kind: domain.IdentityLDAP, LoginName: "alice"}}
			},
			run: func(s *domain.Service) error {
				_, err := s.UnbindOwnIdentity(context.Background(), user, "ldap")
				return err
			},
			want: []string{
				"LockUserWithIdentities", "LockEmailIdentity", "DeleteIdentity:ldap",
				"AppendAudit:" + string(domain.AuditIdentityUnbound), "GetConfigSection",
			},
		},
		{
			name:  "bind of an email identity by an administrator: none, audited, noticed",
			setup: func(r *fakes.RecordingRepo) { r.NoEmailIdentity = true },
			run: func(s *domain.Service) error {
				_, err := s.BindIdentityForUser(context.Background(), admin, 7, domain.BindIdentityInput{Kind: "email", LoginName: newLogin, Password: next})
				return err
			},
			want: []string{
				"LockUserWithIdentities", "FindIdentity", "LockEmailIdentity", "CreateIdentity",
				"AppendAudit:" + string(domain.AuditIdentityBound), "GetConfigSection",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo()
			if tc.setup != nil {
				tc.setup(repo)
			}
			svc := domain.NewService(repo, domain.Options{SecretKey: "test-secret-key"})
			if tc.prepare != nil {
				tc.prepare(svc, repo)
				repo.Calls = nil
			}
			if err := tc.run(svc); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(repo.Calls, tc.want) {
				t.Fatalf("calls =\n  %v\nwant\n  %v", repo.Calls, tc.want)
			}
		})
	}
}

// The refusals of the call sites. The whole recorded sequence is compared again: a refusal
// is made with the locks held and before anything is written, so nothing after the locks may
// run, and each call site keeps its own error and its own order of checks.
func TestCredentialChangeRefusals(t *testing.T) {
	const old, next = "old-password-1", "new-password-22"
	hash, err := bcrypt.GenerateFromPassword([]byte(old), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	newRepo := func() *fakes.RecordingRepo {
		return &fakes.RecordingRepo{
			UserID:   7,
			Username: "alice", Email: "a@example.com",
			Identity: domain.LoginIdentity{UserID: 7, Kind: domain.IdentityEmail, LoginName: "a@example.com", PasswordHash: string(hash)},
			Token: domain.AuthToken{
				ID: 1, UserID: 7, Purpose: domain.TokenPasswordReset, LoginEmail: "a@example.com",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			Sessions: map[string]bool{tokenHash("tok"): true},
		}
	}
	change := func(s *domain.Service, current, session string) error {
		return s.ChangePassword(context.Background(), domain.Actor{User: domain.User{ID: 7}}, session, current, next)
	}
	setWith := func(s *domain.Service) error { return s.SetPasswordWithToken(context.Background(), "tok", next) }
	cli := func(s *domain.Service) error {
		_, err := s.ResetUserByCLI(context.Background(), domain.CLIResetInput{User: "alice", Password: true, SecondFactor: true})
		return err
	}
	kind := func(k domain.ErrorKind) func(error) bool {
		return func(err error) bool { e, ok := domain.AsError(err); return ok && e.Kind == k }
	}
	cases := []struct {
		name    string
		setup   func(*fakes.RecordingRepo)
		prepare func(*domain.Service, *fakes.RecordingRepo)
		run     func(*domain.Service) error
		wantErr func(error) bool
		want    []string
	}{
		{
			name: "ConfirmTOTP: a wrong code is counted and refused, and ends no challenge or session",
			prepare: func(s *domain.Service, r *fakes.RecordingRepo) {
				if _, err := s.StartTOTPSetup(context.Background(), domain.Actor{User: domain.User{ID: 7}}, old); err != nil {
					t.Fatal(err)
				}
			},
			run: func(s *domain.Service) error {
				_, err := s.ConfirmTOTP(context.Background(), domain.Actor{User: domain.User{ID: 7}}, "tok", "000000")
				return err
			},
			wantErr: kind(domain.KindValidation),
			want: []string{
				"LockRateKeys", "LockEmailIdentity", "GetSecondFactorLockout", "ListAuthenticators",
				"PutSecondFactorLockout",
			},
		},
		{
			name:    "ChangePassword: a user without an email identity is not found, before the session is read",
			setup:   func(r *fakes.RecordingRepo) { r.NoEmailIdentity = true },
			run:     func(s *domain.Service) error { return change(s, old, "tok") },
			wantErr: kind(domain.KindNotFound),
			want:    []string{"LockEmailIdentity"},
		},
		{
			name:    "ChangePassword: a session that is gone is unauthenticated, before the password is checked",
			run:     func(s *domain.Service) error { return change(s, "wrong-password-9", "other") },
			wantErr: kind(domain.KindUnauthenticated),
			want:    []string{"LockEmailIdentity", "FindSession"},
		},
		{
			name:    "ChangePassword: a wrong current password changes and ends nothing",
			run:     func(s *domain.Service) error { return change(s, "wrong-password-9", "tok") },
			wantErr: kind(domain.KindValidation),
			want:    []string{"LockEmailIdentity", "FindSession"},
		},
		{
			name: "ChangePassword: the same password changes and ends nothing",
			run: func(s *domain.Service) error {
				return s.ChangePassword(context.Background(), domain.Actor{User: domain.User{ID: 7}}, "tok", old, old)
			},
			wantErr: kind(domain.KindValidation),
			want:    []string{"LockEmailIdentity", "FindSession"},
		},
		{
			name: "ChangePassword: the current password is judged against the hash as the lock reads it, not a stale one",
			setup: func(r *fakes.RecordingRepo) {
				stale := r.Identity // a reset landed after this plain read
				r.StaleIdentity = &stale
				reset, _ := bcrypt.GenerateFromPassword([]byte("temporary-pass-9"), bcrypt.MinCost)
				r.Identity.PasswordHash = string(reset)
			},
			run:     func(s *domain.Service) error { return change(s, old, "tok") },
			wantErr: kind(domain.KindValidation),
			want:    []string{"LockEmailIdentity", "FindSession"},
		},
		{
			name: "SetPasswordWithToken: a token spent since the plain read is refused under the locks",
			setup: func(r *fakes.RecordingRepo) {
				spent := r.Token
				spent.UsedAt = time.Now()
				r.LockedToken = &spent
			},
			run:     setWith,
			wantErr: kind(domain.KindValidation),
			want:    []string{"FindAuthToken", "LockEmailIdentity", "LockAuthToken"},
		},
		{
			name: "SetPasswordWithToken: a token of another user under the locks is refused",
			setup: func(r *fakes.RecordingRepo) {
				other := r.Token
				other.UserID = 8
				r.LockedToken = &other
			},
			run:     setWith,
			wantErr: kind(domain.KindValidation),
			want:    []string{"FindAuthToken", "LockEmailIdentity", "LockAuthToken"},
		},
		{
			name:    "SetPasswordWithToken: no email identity, but the token is locked all the same",
			setup:   func(r *fakes.RecordingRepo) { r.NoEmailIdentity = true },
			run:     setWith,
			wantErr: kind(domain.KindValidation),
			want:    []string{"FindAuthToken", "LockEmailIdentity", "LockAuthToken", "GetUser"},
		},
		{
			name:    "SetPasswordWithToken: a token mailed to another login email is refused",
			setup:   func(r *fakes.RecordingRepo) { r.Identity.LoginName = "renamed@example.com" },
			run:     setWith,
			wantErr: kind(domain.KindValidation),
			want:    []string{"FindAuthToken", "LockEmailIdentity", "LockAuthToken", "GetUser"},
		},
		{
			name: "SetPasswordWithToken: an invitation is void once there is a password",
			setup: func(r *fakes.RecordingRepo) {
				r.Token.Purpose = domain.TokenInvite
				r.Identity.PasswordSet = true
			},
			run:     setWith,
			wantErr: kind(domain.KindValidation),
			want:    []string{"FindAuthToken", "LockEmailIdentity", "LockAuthToken", "GetUser"},
		},
		{
			name:    "CLI: no email identity is refused before any lock on it",
			setup:   func(r *fakes.RecordingRepo) { r.NoEmailIdentity = true },
			run:     cli,
			wantErr: func(err error) bool { return errors.Is(err, domain.ErrCLINoPassword) },
			want:    []string{"FindUserByUsername", "LockUserWithIdentities"},
		},
		{
			name:    "CLI: an identity unbound since it was read is refused before the second factor is touched",
			setup:   func(r *fakes.RecordingRepo) { r.IdentityGoneOnLock = true },
			run:     cli,
			wantErr: func(err error) bool { return errors.Is(err, domain.ErrCLINoPassword) },
			want:    []string{"FindUserByUsername", "LockUserWithIdentities", "LockEmailIdentity"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo()
			if tc.setup != nil {
				tc.setup(repo)
			}
			svc := domain.NewService(repo, domain.Options{SecretKey: "test-secret-key"})
			if tc.prepare != nil {
				tc.prepare(svc, repo)
				repo.Calls = nil
			}
			err := tc.run(svc)
			if err == nil || !tc.wantErr(err) {
				t.Fatalf("err = %v, not the refusal wanted", err)
			}
			if !slices.Equal(repo.Calls, tc.want) {
				t.Fatalf("calls =\n  %v\nwant\n  %v", repo.Calls, tc.want)
			}
		})
	}
}

func recoveryHash(id domain.UserID, code string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("recovery:%d:%s", id, strings.ReplaceAll(code, "-", ""))))
	return hex.EncodeToString(sum[:])
}

// totpNow is the six-digit code (RFC 6238, SHA-1, 30 s) of a base32 secret for the current step.
func totpNow(t *testing.T, secret string) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(time.Now().Unix()/30))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[off:off+4])&0x7fffffff)%1000000)
}

// tokenHash is how the service stores a session token: the hex of its SHA-256.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
