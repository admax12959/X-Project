package domain_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/fakes"
)

// The three call sites that go through the credential change module, each driven through its
// exported entry point against the recording repository. The whole recorded sequence is
// compared: it pins the lock order (user and identity, then the token, then the challenges, then
// the sessions) and the revoke policy (which delete runs, whether tokens are spent), and that an
// audit entry and a notice appear only where they do today.
func TestCredentialChangeCallSites(t *testing.T) {
	const old, next = "old-password-1", "new-password-22"
	hash, err := bcrypt.GenerateFromPassword([]byte(old), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	newRepo := func() *fakes.RecordingRepo {
		return &fakes.RecordingRepo{
			UserID:   7,
			Username: "alice", Email: "a@example.com",
			Identity: domain.LoginIdentity{UserID: 7, Kind: domain.IdentityEmail, LoginName: "a@example.com", PasswordHash: string(hash), MustChangePassword: true},
			Token: domain.AuthToken{
				ID: 1, UserID: 7, Purpose: domain.TokenPasswordReset, LoginEmail: "a@example.com",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			Sessions:     map[string]bool{tokenHash("tok"): true},
			SecondFactor: true,
		}
	}
	cases := []struct {
		name string
		run  func(s *domain.Service) error
		want []string
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
				_, err := s.ResetSecondFactor(context.Background(), domain.Actor{User: domain.User{ID: 1, IsAdmin: true}}, 7)
				return err
			},
			want: []string{
				"LockUserWithIdentities", "ListAuthenticators", "LockEmailIdentity",
				"DeleteAuthenticatorsAndRecoveryCodes", "PutSecondFactorLockout",
				"AppendAudit:" + string(domain.AuditSecondFactorReset), "GetConfigSection",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo()
			if err := tc.run(domain.NewService(repo, domain.Options{})); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(repo.Calls, tc.want) {
				t.Fatalf("calls =\n  %v\nwant\n  %v", repo.Calls, tc.want)
			}
		})
	}
}

// The refusals of the three call sites. The whole recorded sequence is compared again: a refusal
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
		run     func(*domain.Service) error
		wantErr func(error) bool
		want    []string
	}{
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
			err := tc.run(domain.NewService(repo, domain.Options{}))
			if err == nil || !tc.wantErr(err) {
				t.Fatalf("err = %v, not the refusal wanted", err)
			}
			if !slices.Equal(repo.Calls, tc.want) {
				t.Fatalf("calls =\n  %v\nwant\n  %v", repo.Calls, tc.want)
			}
		})
	}
}

// tokenHash is how the service stores a session token: the hex of its SHA-256.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
