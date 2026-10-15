package domain

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Second factor (#22, ADR-0006, docs/design/iam-scope.md "二次验证"). It belongs to the
// user, not to a login identity: once a user has an enabled authenticator, every login of
// that user (LDAP or email) needs a code after the password.
//
// The login is two steps. A passed first factor does not open a session; it hands out a
// short-lived login challenge (an opaque token kept in its own table, so it can never be
// mistaken for a session). Submitting a right code with it opens the session, with a new
// token. Wrong codes are counted per user and lock the second step for a while.

// AuthenticatorType is the kind of an authenticator. Only TOTP exists today; a passkey would
// be a second value and its own setup flow, in the same table.
type AuthenticatorType string

const AuthenticatorTOTP AuthenticatorType = "totp"

// Authenticator is a second-factor method of a user.
type Authenticator struct {
	ID         int64
	UserID     UserID
	Type       AuthenticatorType
	Enabled    bool   // false: set up, but the first code was not confirmed yet
	SecretEnc  []byte // totp: the sealed secret
	KeyVersion int    // version of the master key that sealed SecretEnc
	// LastUsedStep is the time step of the last accepted TOTP code (0 = none): a code of that
	// step or an earlier one is a replay.
	LastUsedStep int64
	CreatedAt    time.Time
	EnabledAt    time.Time // zero until enabled
}

// SecondFactorLockout is the failed-attempt state of a user's second step.
type SecondFactorLockout struct {
	Failures    int
	LockedUntil time.Time // zero = not locked
}

// LoginChallenge is the stored form of the short-lived token of a login whose first factor
// passed.
type LoginChallenge struct {
	TokenHash          string
	UserID             UserID
	MustChangePassword bool
	CreatedAt          time.Time
	ExpiresAt          time.Time
}

// SecondFactorRepository is the part of the persistence port the second factor needs.
type SecondFactorRepository interface {
	// ListAuthenticators returns the user's authenticators, pending ones included.
	ListAuthenticators(ctx context.Context, userID UserID) ([]Authenticator, error)
	// ReplacePendingAuthenticator drops the user's pending authenticators of a.Type and
	// stores a as the new pending one. Call inside InTx.
	ReplacePendingAuthenticator(ctx context.Context, a Authenticator) error
	// EnableAuthenticator turns a pending authenticator on and records the time step of the
	// code that confirmed it. false: it was not pending.
	EnableAuthenticator(ctx context.Context, id int64, step int64, at time.Time) (bool, error)
	// ClaimAuthenticatorStep records step as used, only if it is after the last used one
	// (atomically). false: the step was used already, the code is a replay.
	ClaimAuthenticatorStep(ctx context.Context, id int64, step int64) (bool, error)
	// GetSecondFactorLockout reads the user's lockout state, locking the row inside InTx.
	GetSecondFactorLockout(ctx context.Context, userID UserID) (SecondFactorLockout, error)
	PutSecondFactorLockout(ctx context.Context, userID UserID, l SecondFactorLockout) error

	CreateLoginChallenge(ctx context.Context, c LoginChallenge) error
	FindLoginChallenge(ctx context.Context, tokenHash string) (LoginChallenge, bool, error)
	DeleteLoginChallenge(ctx context.Context, tokenHash string) error
	// DeleteExpiredLoginChallenges removes the challenges that expired before at.
	DeleteExpiredLoginChallenges(ctx context.Context, at time.Time) error

	// ReplaceRecoveryCodes drops all of the user's recovery codes, used ones included, and
	// stores hashes as the new set. Call inside InTx.
	ReplaceRecoveryCodes(ctx context.Context, userID UserID, hashes []string, at time.Time) error
	// ListUnusedRecoveryCodes returns the user's recovery codes that are not spent yet.
	ListUnusedRecoveryCodes(ctx context.Context, userID UserID) ([]RecoveryCode, error)
	// ClaimRecoveryCode spends a code, atomically. false: it was spent already.
	ClaimRecoveryCode(ctx context.Context, id int64, at time.Time) (bool, error)
	// DeleteAuthenticatorsAndRecoveryCodes removes the user's second factor altogether
	// (pending authenticators included). Call inside InTx.
	DeleteAuthenticatorsAndRecoveryCodes(ctx context.Context, userID UserID) error
}

const (
	// A user's second step locks after this many wrong codes, for this long. The count is
	// per user, over all logins, and starts again after a right code or a lock.
	secondFactorMaxFailures = 5
	secondFactorLockFor     = 15 * time.Minute
	// The token of a login waiting for its code is valid this long.
	loginChallengeTTL = 5 * time.Minute

	tokenSecretSection = "authenticator" // binds a sealed secret to its purpose (see sealField)
)

func totpSecretField(id UserID) string { return userTargetID(id) + "/totp" }

func totpInvalid() error        { return Validation(Invalid("code", CodeTotpInvalid, nil)) }
func totpAlreadyEnabled() error { return Validation(Invalid("", CodeTotpAlreadyEnabled, nil)) }

// ---------------------------------------------------------------- status

// SecondFactorStatus is what the security tab shows.
type SecondFactorStatus struct {
	Enabled        bool
	Authenticators []Authenticator // the enabled ones
	// RecoveryCodesRemaining counts the recovery codes that are not spent yet. The codes
	// themselves are shown once, when they are made.
	RecoveryCodesRemaining int
}

func (s *Service) SecondFactorStatus(ctx context.Context, actor Actor) (SecondFactorStatus, error) {
	all, err := s.repo.ListAuthenticators(ctx, actor.User.ID)
	if err != nil {
		return SecondFactorStatus{}, err
	}
	var st SecondFactorStatus
	for _, a := range all {
		if a.Enabled {
			st.Enabled = true
			st.Authenticators = append(st.Authenticators, a)
		}
	}
	codes, err := s.repo.ListUnusedRecoveryCodes(ctx, actor.User.ID)
	if err != nil {
		return SecondFactorStatus{}, err
	}
	st.RecoveryCodesRemaining = len(codes)
	return st, nil
}

// ---------------------------------------------------------------- enrollment

// TOTPSetup is the answer of starting the authenticator setup: the new secret in the three
// forms the user may need.
type TOTPSetup struct {
	Secret     string // base32, to type in by hand
	OtpauthURI string
	QrSVG      string
}

// verifyOwnPassword asks the user to prove who they are again. An email identity is checked
// against its password hash, an LDAP identity against the directory (unreachable is its own
// error, never "wrong password"). Either identity's password counts.
func (s *Service) verifyOwnPassword(ctx context.Context, u User, password string) error {
	if password == "" {
		return Validation(Invalid("password", CodeRequired, nil))
	}
	wrong := Validation(Invalid("password", CodeInvalidCredentials, nil))
	if ident, ok, err := s.repo.FindIdentityByUser(ctx, u.ID, IdentityEmail); err != nil {
		return err
	} else if ok && bcrypt.CompareHashAndPassword([]byte(ident.PasswordHash), []byte(password)) == nil {
		return nil
	}
	ident, ok, err := s.repo.FindIdentityByUser(ctx, u.ID, IdentityLDAP)
	if err != nil {
		return err
	}
	if !ok {
		return wrong
	}
	dir, err := s.currentDirectory(ctx)
	if err != nil {
		return err
	}
	if dir == nil {
		return ldapUnreachable()
	}
	switch dir.Authenticate(ctx, ident.LoginName, password).Outcome {
	case DirectoryOK:
		return nil
	case DirectoryUnreachable:
		return ldapUnreachable()
	default:
		return wrong
	}
}

// StartTOTPSetup creates a new secret for the user after the password was typed again. The
// secret is stored sealed and counts for nothing until ConfirmTOTP gets a right code; asking
// again replaces it.
func (s *Service) StartTOTPSetup(ctx context.Context, actor Actor, password string) (TOTPSetup, error) {
	u := actor.User
	if err := s.verifyOwnPassword(ctx, u, password); err != nil {
		return TOTPSetup{}, err
	}
	secret, err := newTOTPSecret()
	if err != nil {
		return TOTPSetup{}, err
	}
	enc, version, err := s.tokens.sealField(tokenSecretSection, totpSecretField(u.ID), secret)
	if err != nil {
		return TOTPSetup{}, err
	}
	err = s.repo.InTx(ctx, func(tx Repository) error {
		if has, err := hasEnabledTOTP(ctx, tx, u.ID); err != nil {
			return err
		} else if has {
			return totpAlreadyEnabled()
		}
		return tx.ReplacePendingAuthenticator(ctx, Authenticator{
			UserID: u.ID, Type: AuthenticatorTOTP, SecretEnc: enc, KeyVersion: version, CreatedAt: s.nowUTC(),
		})
	})
	if err != nil {
		return TOTPSetup{}, err
	}
	account := u.Email
	if account == "" {
		account = u.Username
	}
	uri := otpauthURI(secret, account)
	svg, err := qrSVG(uri)
	if err != nil {
		return TOTPSetup{}, err
	}
	return TOTPSetup{Secret: totpEncoding.EncodeToString(secret), OtpauthURI: uri, QrSVG: svg}, nil
}

func hasEnabledTOTP(ctx context.Context, tx Repository, id UserID) (bool, error) {
	all, err := tx.ListAuthenticators(ctx, id)
	if err != nil {
		return false, err
	}
	for _, a := range all {
		if a.Type == AuthenticatorTOTP && a.Enabled {
			return true, nil
		}
	}
	return false, nil
}

// openAuthenticatorSecret decrypts the TOTP secret of a. An unreadable one (the master key
// changed) is reported as such: the user cannot be verified until an administrator resets
// the second factor.
func (s *Service) openAuthenticatorSecret(a Authenticator) ([]byte, error) {
	secret, err := s.tokens.openField(tokenSecretSection, totpSecretField(a.UserID), a.SecretEnc, a.KeyVersion)
	if errors.Is(err, errSecretUnreadable) {
		slog.Error("second factor secret cannot be decrypted with the current master key", "userId", a.UserID)
		return nil, newKind(KindUnavailable, CodeSecondFactorUnreadable)
	}
	return secret, err
}

// ConfirmTOTP enables the pending authenticator if code is right for it, and makes the
// user's recovery codes, returned this once. The code's time step is used up, so the same
// code cannot log in right after.
func (s *Service) ConfirmTOTP(ctx context.Context, actor Actor, code string) ([]string, error) {
	var c Collector
	if !c.Required("code", code) {
		return nil, c.Err()
	}
	u := actor.User
	var recovery []string
	err := s.repo.InTx(ctx, func(tx Repository) error {
		all, err := tx.ListAuthenticators(ctx, u.ID)
		if err != nil {
			return err
		}
		var pending *Authenticator
		for i, a := range all {
			if a.Type == AuthenticatorTOTP && a.Enabled {
				return totpAlreadyEnabled()
			}
			if a.Type == AuthenticatorTOTP {
				pending = &all[i]
			}
		}
		if pending == nil {
			return newKind(KindNotFound, CodeNotFound)
		}
		secret, err := s.openAuthenticatorSecret(*pending)
		if err != nil {
			return err
		}
		step, ok := matchTOTP(secret, code, s.nowUTC(), 0)
		if !ok {
			return totpInvalid()
		}
		if on, err := tx.EnableAuthenticator(ctx, pending.ID, step, s.nowUTC()); err != nil {
			return err
		} else if !on {
			return newKind(KindNotFound, CodeNotFound)
		}
		if recovery, err = s.replaceRecoveryCodes(ctx, tx, u.ID); err != nil {
			return err
		}
		return s.recordAudit(ctx, tx, AuditActorOf(actor), secondFactorEnabledAudit(u.ID, AuthenticatorTOTP))
	})
	if err != nil {
		return nil, err
	}
	return recovery, nil
}

// ---------------------------------------------------------------- the login's second step

// issueLoginChallenge is what a passed first factor produces for a user who has a second
// factor: a token for SubmitSecondFactor, and no session.
func (s *Service) issueLoginChallenge(ctx context.Context, u User, mustChange bool) (LoginResult, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return LoginResult{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.nowUTC()
	ch := LoginChallenge{TokenHash: hashToken(token), UserID: u.ID, MustChangePassword: mustChange, CreatedAt: now, ExpiresAt: now.Add(loginChallengeTTL)}
	if err := s.repo.DeleteExpiredLoginChallenges(ctx, now); err != nil {
		return LoginResult{}, err
	}
	if err := s.repo.CreateLoginChallenge(ctx, ch); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{SecondFactorToken: token, SecondFactorExpiresAt: ch.ExpiresAt, User: u, MustChangePassword: mustChange}, nil
}

// SubmitSecondFactor completes a login: token is the challenge from the first step, code the
// authenticator's code. On success the challenge is spent and a session opens, with a token
// of its own.
//
// Errors: 401 unauthenticated (no, unknown or expired challenge: log in again), 422 code
// totpInvalid (wrong, or a replay), 429 totpLocked (too many wrong codes; params lockedUntil).
func (s *Service) SubmitSecondFactor(ctx context.Context, token, code string) (LoginResult, error) {
	unauth := newKind(KindUnauthenticated, CodeUnauthenticated)
	if token == "" {
		return LoginResult{}, unauth
	}
	ch, ok, err := s.repo.FindLoginChallenge(ctx, hashToken(token))
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || !s.nowUTC().Before(ch.ExpiresAt) {
		return LoginResult{}, unauth
	}
	u, ok, err := s.repo.GetUser(ctx, ch.UserID)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || u.Status != UserEnabled {
		return LoginResult{}, unauth
	}
	var c Collector
	if !c.Required("code", code) {
		return LoginResult{}, c.Err()
	}

	if lock, err := s.repo.GetSecondFactorLockout(ctx, u.ID); err != nil {
		return LoginResult{}, err
	} else if s.nowUTC().Before(lock.LockedUntil) {
		return LoginResult{}, secondFactorLocked(lock.LockedUntil)
	}

	all, err := s.repo.ListAuthenticators(ctx, u.ID)
	if err != nil {
		return LoginResult{}, err
	}
	good := false
	for _, a := range all {
		if a.Type != AuthenticatorTOTP || !a.Enabled {
			continue
		}
		secret, err := s.openAuthenticatorSecret(a)
		if err != nil {
			return LoginResult{}, err
		}
		step, matched := matchTOTP(secret, code, s.nowUTC(), a.LastUsedStep)
		if !matched {
			continue
		}
		// Claim the step; of two requests with the same code only one gets it.
		if claimed, err := s.repo.ClaimAuthenticatorStep(ctx, a.ID, step); err != nil {
			return LoginResult{}, err
		} else if claimed {
			good = true
		}
		break
	}
	if !good {
		return LoginResult{}, s.recordSecondFactorFailure(ctx, u.ID)
	}

	err = s.repo.InTx(ctx, func(tx Repository) error {
		if err := tx.PutSecondFactorLockout(ctx, u.ID, SecondFactorLockout{}); err != nil {
			return err
		}
		return tx.DeleteLoginChallenge(ctx, ch.TokenHash)
	})
	if err != nil {
		return LoginResult{}, err
	}
	return s.createSession(ctx, u, ch.MustChangePassword)
}

func secondFactorLocked(until time.Time) error {
	return &Error{Kind: KindTooManyRequests, Errors: []FieldError{{Code: CodeTotpLocked, Params: map[string]any{"lockedUntil": until.UTC().Format(time.RFC3339)}}}}
}

// recordSecondFactorFailure counts a wrong code against the user, locks the second step when
// the count reaches the threshold, and returns the error to answer with: totpLocked if that
// failure locked it, totpInvalid otherwise.
func (s *Service) recordSecondFactorFailure(ctx context.Context, id UserID) error {
	var answer error = totpInvalid()
	err := s.repo.InTx(ctx, func(tx Repository) error {
		cur, err := tx.GetSecondFactorLockout(ctx, id)
		if err != nil {
			return err
		}
		now := s.nowUTC()
		if now.Before(cur.LockedUntil) { // locked by a request that ran in parallel
			answer = secondFactorLocked(cur.LockedUntil)
			return nil
		}
		cur.Failures++
		if cur.Failures >= secondFactorMaxFailures {
			cur = SecondFactorLockout{LockedUntil: now.Add(secondFactorLockFor)}
			answer = secondFactorLocked(cur.LockedUntil)
		}
		return tx.PutSecondFactorLockout(ctx, id, cur)
	})
	if err != nil {
		return err
	}
	return answer
}
