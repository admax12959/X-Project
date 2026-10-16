package domain_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
			name: "CLI password and second factor reset: the factor is reset inside the locks",
			run: func(s *domain.Service) error {
				_, err := s.ResetUserByCLI(context.Background(), domain.CLIResetInput{User: "alice", Password: true, SecondFactor: true})
				return err
			},
			want: []string{
				"FindUserByUsername", "LockUserWithIdentities", "LockEmailIdentity",
				"ListAuthenticators", "DeleteAuthenticatorsAndRecoveryCodes", "PutSecondFactorLockout",
				"AppendAudit:" + string(domain.AuditSecondFactorReset), "GetConfigSection",
				"SetTemporaryPassword", "UseUserAuthTokens", "DeleteUserLoginChallenges", "DeleteUserSessions",
				"AppendAudit:" + string(domain.AuditPasswordReset), "GetConfigSection",
			},
		},
		{
			name: "CLI second factor reset alone changes no credential",
			run: func(s *domain.Service) error {
				_, err := s.ResetUserByCLI(context.Background(), domain.CLIResetInput{User: "alice", SecondFactor: true})
				return err
			},
			want: []string{
				"FindUserByUsername", "LockUserWithIdentities",
				"ListAuthenticators", "DeleteAuthenticatorsAndRecoveryCodes", "PutSecondFactorLockout",
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

// tokenHash is how the service stores a session token: the hex of its SHA-256.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
