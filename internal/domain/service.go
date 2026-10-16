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
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Service is the domain service.
type Service struct {
	repo          Repository
	directories   DirectoryFactory // nil: LDAP is not available; the directory is built from the stored configuration (ldap_config.go)
	codePlatforms CodePlatformAdapter
	mailSenders   MailSenderFactory  // nil: mail cannot be sent; the sender is built from the stored mail configuration (mail_config.go)
	mailBackoff   []time.Duration    // wait after the 1st, 2nd, ... failed delivery; len+1 attempts in all (mail_queue.go)
	objectStores  ObjectStoreFactory // nil: object storage is not available; the store is built from the stored configuration (storage_config.go)
	localDir      string             // directory of the local object store, used until an S3 storage is configured
	now           func() time.Time
	sessionTTL    time.Duration
	tokens        tokenCipher // encrypts stored secrets; zero value when no SecretKey
	avatarLimits  uploadLimiter
	background    sync.WaitGroup // mail preparation after a password recovery request (auth_tokens.go)
}

// Options configures a Service.
type Options struct {
	DirectoryFactory   DirectoryFactory
	CodePlatforms      CodePlatformAdapter
	MailSenders        MailSenderFactory
	MailRetryBackoff   []time.Duration // default DefaultMailRetryBackoff
	ObjectStoreFactory ObjectStoreFactory
	LocalStorageDir    string           // directory of the local-disk object store
	Now                func() time.Time // defaults to time.Now
	SessionTTL         time.Duration
	SecretKey          string // the master key: encrypts stored secrets; the app refuses to start without it
	SecretKeyVersion   int    // version of the master key, recorded with every stored secret; default 1
}

func NewService(repo Repository, o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.SessionTTL <= 0 {
		o.SessionTTL = 12 * time.Hour
	}
	if o.MailRetryBackoff == nil {
		o.MailRetryBackoff = DefaultMailRetryBackoff
	}
	tokens, _ := newTokenCipher(o.SecretKey) // zero value on empty key; sealing then fails loudly
	if o.SecretKeyVersion > 0 {
		tokens.version = o.SecretKeyVersion
	}
	return &Service{
		repo: repo, directories: o.DirectoryFactory, codePlatforms: o.CodePlatforms, mailSenders: o.MailSenders, mailBackoff: o.MailRetryBackoff,
		objectStores: o.ObjectStoreFactory, localDir: o.LocalStorageDir, now: o.Now, sessionTTL: o.SessionTTL, tokens: tokens,
	}
}

func (s *Service) nowUTC() time.Time { return s.now().UTC().Truncate(time.Microsecond) }

// Actor is the authenticated user a request acts as.
type Actor struct {
	User User
	// MustChangePassword: the session was opened with an administrator-chosen password.
	// The HTTP layer lets such a session do nothing but change the password and log out.
	MustChangePassword bool
	// SecondFactorSetupRequired: the user is required to have a second factor and has none
	// (security_policy.go). The HTTP layer lets such a session do nothing but set one up.
	SecondFactorSetupRequired bool
}

// ---------------------------------------------------------------- bootstrap

// InitialAdmin is the configured initial administrator.
type InitialAdmin struct {
	Username    string
	Password    string
	DisplayName string
	Email       string // the login email; the administrator's contact email too
}

// ErrBootstrapConfig is returned on a first start without initial admin credentials.
var ErrBootstrapConfig = errors.New("first start: initial administrator username, email and password must be configured (admin.username/admin.email/admin.password or XP_ADMIN_USERNAME/XP_ADMIN_EMAIL/XP_ADMIN_PASSWORD)")

// Bootstrap creates the initial administrator, with an email identity whose login email
// is the configured one (taken as verified), on the very first start and does nothing on
// later starts, whatever the configuration says then.
func (s *Service) Bootstrap(ctx context.Context, admin InitialAdmin) error {
	return s.repo.InTx(ctx, func(tx Repository) error {
		done, err := tx.LockBootstrap(ctx)
		if err != nil || done {
			return err
		}
		admin.Email = normalizeLoginEmail(admin.Email)
		if strings.TrimSpace(admin.Username) == "" || admin.Email == "" || admin.Password == "" {
			return ErrBootstrapConfig
		}
		if !usernamePattern.MatchString(admin.Username) {
			return errors.New("first start: initial administrator username must match " + usernamePattern.String())
		}
		if !emailPattern.MatchString(admin.Email) || len(admin.Email) > maxEmailLength {
			return errors.New("first start: initial administrator email is not a valid email address")
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
			UserID: id, Kind: IdentityEmail, LoginName: admin.Email,
			PasswordHash: hash, PasswordSet: true, BoundAt: now, BoundVia: BoundViaBootstrap,
		}); err != nil {
			return err
		}
		return tx.MarkBootstrapped(ctx, now)
	})
}

// ---------------------------------------------------------------- sessions

// LoginInput is a login attempt. Method is "email" (Email is the login email) or "ldap"
// (Username is the directory login name).
type LoginInput struct {
	Method   string
	Username string
	Email    string
	Password string
}

// LoginResult carries the raw session token (to be set as a cookie) and the user.
type LoginResult struct {
	Token              string
	ExpiresAt          time.Time
	User               User
	MustChangePassword bool
	// SecondFactorSetupRequired: see Actor.
	SecondFactorSetupRequired bool
	// SecondFactorToken is set instead of Token when the user has a second factor: the first
	// factor passed, but there is no session until SubmitSecondFactor gets a right code.
	SecondFactorToken     string
	SecondFactorExpiresAt time.Time
}

// A fixed bcrypt hash so unknown login emails cost the same as wrong passwords.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("x-project-dummy"), bcrypt.DefaultCost)

func invalidCredentials() error { return newKind(KindUnauthenticated, CodeInvalidCredentials) }

// LoginMethods lists the login methods the login page may offer: "email" always,
// "ldap" when LDAP is switched on in the system configuration.
func (s *Service) LoginMethods(ctx context.Context) ([]string, error) {
	m := []string{string(IdentityEmail)}
	on, err := s.ldapEnabled(ctx)
	if on {
		m = append(m, string(IdentityLDAP))
	}
	return m, err
}

// Login authenticates a user and opens a session. Wrong login, wrong password
// and disabled users all yield the same invalidCredentials error.
func (s *Service) Login(ctx context.Context, in LoginInput) (LoginResult, error) {
	var c Collector
	allowed, err := s.LoginMethods(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	if !slices.Contains(allowed, in.Method) {
		c.Add("method", CodeInvalid, map[string]any{"allowed": allowed})
	}
	if in.Method == string(IdentityEmail) {
		c.Required("email", in.Email)
	} else {
		c.Required("username", in.Username)
	}
	if in.Password == "" {
		c.Add("password", CodeRequired, nil)
	}
	if err := c.Err(); err != nil {
		return LoginResult{}, err
	}
	if in.Method == string(IdentityLDAP) {
		return s.loginLDAP(ctx, in)
	}

	ident, found, err := s.repo.FindIdentity(ctx, IdentityEmail, normalizeLoginEmail(in.Email))
	if err != nil {
		return LoginResult{}, err
	}
	// An unknown email and an invited user (no password yet) are compared against the dummy hash
	// so the timing matches, but the outcome is rejected whatever the comparison says: the dummy's
	// own plaintext must never authenticate.
	usable := found && ident.PasswordHash != ""
	hash := dummyHash
	if usable {
		hash = []byte(ident.PasswordHash)
	}
	matched := bcrypt.CompareHashAndPassword(hash, []byte(in.Password)) == nil
	if !usable || !matched {
		return LoginResult{}, invalidCredentials()
	}
	u, ok, err := s.repo.GetUser(ctx, ident.UserID)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || u.Status != UserEnabled {
		return LoginResult{}, invalidCredentials()
	}
	return s.openSession(ctx, u, ident.MustChangePassword)
}

// openSession is the end of a passed first factor: a session, or, for a user with a second
// factor, a login challenge to be completed with SubmitSecondFactor (secondfactor.go).
func (s *Service) openSession(ctx context.Context, u User, mustChange bool) (LoginResult, error) {
	on, err := hasEnabledTOTP(ctx, s.repo, u.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if on {
		return s.issueLoginChallenge(ctx, u, mustChange)
	}
	return s.createSession(ctx, u, mustChange, false)
}

// createSession inserts a session. A password-only session (secondFactorPassed false) is checked
// again once it exists: if the user enabled a second factor between the decision to open it and
// its insertion (ConfirmTOTP ends the other sessions, but this one was not there yet), it would
// pass for fully authenticated, so it is deleted and the login becomes a challenge instead.
func (s *Service) createSession(ctx context.Context, u User, mustChange, secondFactorPassed bool) (LoginResult, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return LoginResult{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.nowUTC()
	sess := Session{TokenHash: hashToken(token), UserID: u.ID, CreatedAt: now, ExpiresAt: now.Add(s.sessionTTL), MustChangePassword: mustChange}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return LoginResult{}, err
	}
	if !secondFactorPassed {
		on, err := hasEnabledTOTP(ctx, s.repo, u.ID)
		if err != nil {
			return LoginResult{}, err
		}
		if on {
			if err := s.repo.DeleteSession(ctx, sess.TokenHash); err != nil {
				return LoginResult{}, err
			}
			return s.issueLoginChallenge(ctx, u, mustChange)
		}
	}
	setup, err := s.secondFactorSetupRequiredFor(ctx, u)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: sess.ExpiresAt, User: u, MustChangePassword: mustChange, SecondFactorSetupRequired: setup}, nil
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
	setup, err := s.secondFactorSetupRequiredFor(ctx, u)
	if err != nil {
		return Actor{}, err
	}
	return Actor{User: u, MustChangePassword: sess.MustChangePassword, SecondFactorSetupRequired: setup}, nil
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
