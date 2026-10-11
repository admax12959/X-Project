// Package domain holds every business rule. The HTTP layer only parses,
// authenticates and serializes; the store is only reached through Repository.
package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Service is the domain service.
type Service struct {
	repo          Repository
	directory     Directory // nil when LDAP is not configured
	codePlatforms CodePlatformAdapter
	now           func() time.Time
	sessionTTL    time.Duration
}

// Options configures a Service.
type Options struct {
	Directory     Directory
	CodePlatforms CodePlatformAdapter
	Now           func() time.Time // defaults to time.Now
	SessionTTL    time.Duration
}

func NewService(repo Repository, o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.SessionTTL <= 0 {
		o.SessionTTL = 12 * time.Hour
	}
	return &Service{repo: repo, directory: o.Directory, codePlatforms: o.CodePlatforms, now: o.Now, sessionTTL: o.SessionTTL}
}

func (s *Service) nowUTC() time.Time { return s.now().UTC().Truncate(time.Microsecond) }

// Actor is the authenticated user a request acts as.
type Actor struct {
	User User
}

// ---------------------------------------------------------------- bootstrap

// InitialAdmin is the configured initial administrator.
type InitialAdmin struct {
	Username    string
	Password    string
	DisplayName string
	Email       string
}

// ErrBootstrapConfig is returned on a first start without initial admin credentials.
var ErrBootstrapConfig = errors.New("first start: initial administrator username and password must be configured (admin.username/admin.password or XP_ADMIN_USERNAME/XP_ADMIN_PASSWORD)")

// Bootstrap creates the local initial administrator on the very first start and
// does nothing on later starts, whatever the configuration says then.
func (s *Service) Bootstrap(ctx context.Context, admin InitialAdmin) error {
	return s.repo.InTx(ctx, func(tx Repository) error {
		done, err := tx.LockBootstrap(ctx)
		if err != nil || done {
			return err
		}
		if strings.TrimSpace(admin.Username) == "" || admin.Password == "" {
			return ErrBootstrapConfig
		}
		if !usernamePattern.MatchString(admin.Username) {
			return errors.New("first start: initial administrator username must match " + usernamePattern.String())
		}
		hash, err := hashPassword(admin.Password)
		if err != nil {
			return err
		}
		now := s.nowUTC()
		name := admin.DisplayName
		if strings.TrimSpace(name) == "" {
			name = admin.Username
		}
		id, err := tx.CreateUser(ctx, User{
			Username: admin.Username, DisplayName: name, Email: admin.Email,
			Status: UserEnabled, IsAdmin: true, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := tx.CreateIdentity(ctx, LoginIdentity{
			UserID: id, Kind: IdentityLocal, LoginName: admin.Username,
			PasswordHash: hash, BoundAt: now, BoundVia: BoundViaBootstrap,
		}); err != nil {
			return err
		}
		return tx.MarkBootstrapped(ctx, now)
	})
}

// ---------------------------------------------------------------- sessions

// LoginInput is a login attempt. Method is "local" or "ldap".
type LoginInput struct {
	Method   string
	Username string
	Password string
}

// LoginResult carries the raw session token (to be set as a cookie) and the user.
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      User
}

// A fixed bcrypt hash so unknown usernames cost the same as wrong passwords.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("x-project-dummy"), bcrypt.DefaultCost)

func invalidCredentials() error { return newKind(KindUnauthenticated, CodeInvalidCredentials) }

// LoginMethods lists the login methods the login page may offer: "local" always,
// "ldap" when a directory is configured.
func (s *Service) LoginMethods() []string {
	m := []string{string(IdentityLocal)}
	if s.directory != nil {
		m = append(m, string(IdentityLDAP))
	}
	return m
}

// Login authenticates a user and opens a session. Wrong username, wrong password
// and disabled users all yield the same invalidCredentials error.
func (s *Service) Login(ctx context.Context, in LoginInput) (LoginResult, error) {
	var c Collector
	allowed := s.LoginMethods()
	if !slices.Contains(allowed, in.Method) {
		c.Add("method", CodeInvalid, map[string]any{"allowed": allowed})
	}
	c.Required("username", in.Username)
	if in.Password == "" {
		c.Add("password", CodeRequired, nil)
	}
	if err := c.Err(); err != nil {
		return LoginResult{}, err
	}
	if in.Method == string(IdentityLDAP) {
		return s.loginLDAP(ctx, in)
	}

	ident, found, err := s.repo.FindIdentity(ctx, IdentityLocal, strings.TrimSpace(in.Username))
	if err != nil {
		return LoginResult{}, err
	}
	hash := dummyHash
	if found {
		hash = []byte(ident.PasswordHash)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(in.Password)) != nil || !found {
		return LoginResult{}, invalidCredentials()
	}
	u, ok, err := s.repo.GetUser(ctx, ident.UserID)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || u.Status != UserEnabled {
		return LoginResult{}, invalidCredentials()
	}
	return s.openSession(ctx, u)
}

func (s *Service) openSession(ctx context.Context, u User) (LoginResult, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return LoginResult{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.nowUTC()
	sess := Session{TokenHash: hashToken(token), UserID: u.ID, CreatedAt: now, ExpiresAt: now.Add(s.sessionTTL)}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: sess.ExpiresAt, User: u}, nil
}

// Authenticate resolves a session token to an Actor, or returns an unauthenticated error.
func (s *Service) Authenticate(ctx context.Context, token string) (Actor, error) {
	unauth := newKind(KindUnauthenticated, CodeUnauthenticated)
	if token == "" {
		return Actor{}, unauth
	}
	sess, ok, err := s.repo.FindSession(ctx, hashToken(token))
	if err != nil {
		return Actor{}, err
	}
	if !ok || !s.nowUTC().Before(sess.ExpiresAt) {
		return Actor{}, unauth
	}
	u, ok, err := s.repo.GetUser(ctx, sess.UserID)
	if err != nil {
		return Actor{}, err
	}
	if !ok || u.Status != UserEnabled {
		return Actor{}, unauth
	}
	return Actor{User: u}, nil
}

// Logout ends the session identified by token. Unknown tokens are ignored.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, hashToken(token))
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// ---------------------------------------------------------------- applications

// ListApplications returns every application; any logged-in user may browse.
func (s *Service) ListApplications(ctx context.Context, _ Actor) ([]ApplicationSummary, error) {
	return s.repo.ListApplications(ctx)
}
