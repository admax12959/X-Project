package domain

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net"
	"strings"
	"time"
)

// One-time tokens (#21). The link in an invitation or password recovery mail carries one. Only
// the SHA-256 of a token is stored; the token itself exists in the mail and nowhere else. A
// token says what it is for (TokenPurpose), when it expires and whether it has been used, and
// is used at most once: using it spends every other token of the same user that sets a
// password as well. Whether a token has expired is judged by the service's clock
// (Options.Now), never by the database's.

// TokenPurpose is what a one-time token may be used for. Later mail flows (email
// verification, two-factor reset) add purposes, not tables.
type TokenPurpose string

const (
	TokenInvite        TokenPurpose = "invite"        // a new user chooses their first password
	TokenPasswordReset TokenPurpose = "passwordReset" // a user who forgot their password chooses a new one
)

// passwordTokenPurposes are the purposes whose tokens set a password.
var passwordTokenPurposes = []TokenPurpose{TokenInvite, TokenPasswordReset}

const (
	inviteTokenTTL        = 7 * 24 * time.Hour
	passwordResetTokenTTL = time.Hour

	// Password recovery requests that count towards a limit are those that were accepted. The
	// limits are per normalized email address and per client IP address.
	recoveryWindow        = time.Hour
	recoveryLimitPerEmail = 3
	recoveryLimitPerIP    = 10
)

// AuthToken is a row of the one-time token table. TokenHash is the SHA-256 of the token.
type AuthToken struct {
	ID        int64
	TokenHash string
	UserID    UserID
	Purpose   TokenPurpose
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    time.Time // zero unless used
}

// AuthTokenRepository is the persistence port of one-time tokens and rate-limit events.
type AuthTokenRepository interface {
	InsertAuthToken(ctx context.Context, t AuthToken) error
	// LockAuthToken reads a token by hash with a locking read (inside InTx), so that two
	// requests cannot both use it.
	LockAuthToken(ctx context.Context, tokenHash string) (AuthToken, bool, error)
	// UseUserAuthTokens marks every unused token of the user with one of the purposes as used.
	UseUserAuthTokens(ctx context.Context, userID UserID, purposes []TokenPurpose, at time.Time) error
	// DeleteUserSessions ends every session of the user.
	DeleteUserSessions(ctx context.Context, userID UserID) error

	// CountRateEvents counts the events of a bucket that happened after since.
	CountRateEvents(ctx context.Context, bucket string, since time.Time) (int, error)
	AddRateEvent(ctx context.Context, bucket string, at time.Time) error
	PruneRateEvents(ctx context.Context, before time.Time) error
}

func tokenInvalid() error      { return Validation(Invalid("token", CodeTokenInvalid, nil)) }
func rateLimitedError() *Error { return newKind(KindTooManyRequests, CodeRateLimited) }

// newToken makes a token and the row that stores its hash.
func (s *Service) newToken(userID UserID, purpose TokenPurpose, ttl time.Duration) (string, AuthToken, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", AuthToken{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.nowUTC()
	return token, AuthToken{TokenHash: hashToken(token), UserID: userID, Purpose: purpose, CreatedAt: now, ExpiresAt: now.Add(ttl)}, nil
}

// issueToken stores a new token within tx, after spending the user's earlier unused tokens of
// the same purpose, and returns the token to put into a mail.
func (s *Service) issueToken(ctx context.Context, tx Repository, userID UserID, purpose TokenPurpose, ttl time.Duration) (string, error) {
	token, row, err := s.newToken(userID, purpose, ttl)
	if err != nil {
		return "", err
	}
	if err := tx.UseUserAuthTokens(ctx, userID, []TokenPurpose{purpose}, row.CreatedAt); err != nil {
		return "", err
	}
	return token, tx.InsertAuthToken(ctx, row)
}

// setPasswordLinkPath is the page, shared by invitations and recovery, where the token is
// exchanged for a password.
func setPasswordLinkPath(token string) string { return "/set-password/" + token }

// PasswordRecoveryAvailable reports whether "forgot password" can work: it needs mail.
func (s *Service) PasswordRecoveryAvailable(ctx context.Context) (bool, error) {
	return s.MailConfigured(ctx)
}

// RequestPasswordRecovery handles the "forgot password" form. Whatever the address is, the
// answer is the same and arrives in the same time: after the input checks and the rate limits
// (which count every accepted request, whether the account exists or not) it returns, and the
// lookup, the token and the mail happen in the background. Only an enabled user with an email
// identity gets a mail; everyone else gets nothing and cannot tell the difference. clientIP is
// the address of the TCP peer: no proxy header is trusted.
func (s *Service) RequestPasswordRecovery(ctx context.Context, email, clientIP string) error {
	email = normalizeLoginEmail(email)
	var c Collector
	checkLoginEmail(&c, "email", email)
	if err := c.Err(); err != nil {
		return err
	}
	ok, err := s.MailConfigured(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return newKind(KindUnavailable, CodeMailNotConfigured)
	}

	now := s.nowUTC()
	buckets := []struct {
		key   string
		limit int
	}{
		{"recovery:ip:" + clientIP, recoveryLimitPerIP},
		{"recovery:email:" + hashToken(email), recoveryLimitPerEmail},
	}
	err = s.repo.InTx(ctx, func(tx Repository) error {
		if err := tx.PruneRateEvents(ctx, now.Add(-recoveryWindow)); err != nil {
			return err
		}
		for _, b := range buckets {
			n, err := tx.CountRateEvents(ctx, b.key, now.Add(-recoveryWindow))
			if err != nil {
				return err
			}
			if n >= b.limit {
				return rateLimitedError()
			}
		}
		for _, b := range buckets {
			if err := tx.AddRateEvent(ctx, b.key, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.background.Add(1)
	go func() {
		defer s.background.Done()
		if err := s.sendPasswordRecovery(context.WithoutCancel(ctx), email); err != nil {
			slog.Error("password recovery: preparing the mail", "err", err)
		}
	}()
	return nil
}

func (s *Service) sendPasswordRecovery(ctx context.Context, email string) error {
	return s.repo.InTx(ctx, func(tx Repository) error {
		ident, found, err := tx.FindIdentity(ctx, IdentityEmail, email)
		if err != nil || !found {
			return err
		}
		u, found, err := tx.GetUser(ctx, ident.UserID)
		if err != nil || !found || u.Status != UserEnabled {
			return err
		}
		token, err := s.issueToken(ctx, tx, u.ID, TokenPasswordReset, passwordResetTokenTTL)
		if err != nil {
			return err
		}
		_, err = s.enqueueMail(ctx, tx, MailRequest{
			Template: MailTemplatePasswordReset, To: ident.LoginName, Recipient: u.ID, LinkPath: setPasswordLinkPath(token),
		})
		return err
	})
}

// SetPasswordWithToken is the set-password page (invitation and recovery): the one-time token
// from the mail plus the new password. The token is spent, every other unused token of the
// user that sets a password is spent with it, the password is replaced, and every session of
// the user ends.
func (s *Service) SetPasswordWithToken(ctx context.Context, token, password string) error {
	var c Collector
	c.Required("token", token)
	checkNewPasswordField(&c, "password", password)
	if err := c.Err(); err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		t, found, err := tx.LockAuthToken(ctx, hashToken(token))
		if err != nil {
			return err
		}
		if !found || !isPasswordPurpose(t.Purpose) {
			return tokenInvalid()
		}
		now := s.nowUTC()
		if !t.UsedAt.IsZero() {
			return Validation(Invalid("token", CodeTokenUsed, nil))
		}
		if !now.Before(t.ExpiresAt) {
			return Validation(Invalid("token", CodeTokenExpired, nil))
		}
		u, found, err := tx.GetUser(ctx, t.UserID)
		if err != nil {
			return err
		}
		if _, hasEmail, err := tx.FindIdentityByUser(ctx, t.UserID, IdentityEmail); err != nil {
			return err
		} else if !found || u.Status != UserEnabled || !hasEmail {
			return tokenInvalid()
		}
		if err := tx.ChangePassword(ctx, t.UserID, hash); err != nil {
			return err
		}
		if err := tx.UseUserAuthTokens(ctx, t.UserID, passwordTokenPurposes, now); err != nil {
			return err
		}
		return tx.DeleteUserSessions(ctx, t.UserID)
	})
}

func isPasswordPurpose(p TokenPurpose) bool {
	for _, q := range passwordTokenPurposes {
		if p == q {
			return true
		}
	}
	return false
}

// SendInvitation (re)sends the invitation to a user who has not set a password yet: earlier
// invitation links stop working. Needs mail (mailNotConfigured otherwise).
func (s *Service) SendInvitation(ctx context.Context, actor Actor, id UserID) error {
	if err := requirePermission(actor, PermUserManage); err != nil {
		return err
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		u, found, err := tx.GetUser(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		ident, hasEmail, err := tx.FindIdentityByUser(ctx, id, IdentityEmail)
		if err != nil {
			return err
		}
		if !hasEmail || ident.PasswordSet || u.Status != UserEnabled {
			return newKind(KindValidation, CodeInviteNotPending)
		}
		return s.inviteWithin(ctx, tx, u.ID, ident.LoginName)
	})
}

// inviteWithin issues an invitation token and queues the mail, within tx.
func (s *Service) inviteWithin(ctx context.Context, tx Repository, userID UserID, loginEmail string) error {
	token, err := s.issueToken(ctx, tx, userID, TokenInvite, inviteTokenTTL)
	if err != nil {
		return err
	}
	_, err = s.enqueueMail(ctx, tx, MailRequest{
		Template: MailTemplateInvite, To: loginEmail, Recipient: userID, LinkPath: setPasswordLinkPath(token),
	})
	return err
}

// WaitBackground waits for the mail preparation started by password recovery requests; the
// application calls it when it shuts down, before closing the database.
func (s *Service) WaitBackground() { s.background.Wait() }

// ClientIP is the host part of a "host:port" peer address.
func ClientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.TrimSpace(remoteAddr)
	}
	return host
}
