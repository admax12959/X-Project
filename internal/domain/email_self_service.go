package domain

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// Self-service email identities (#30, iam-scope "登录身份"): a user without an email identity
// adds one, a user with one changes the login email. In both the user proves they own the new
// address with a six-digit code that arrives by mail, and which is a one-time token (#21) of its
// own purpose: the token row stores the SHA-256 of (purpose, user, address, code), so a code
// works only for the address it was sent to, only for the user who asked, and only once.
//
// The code is short, so guessing is limited: a user who has typed too many wrong codes is
// refused (rateLimited) for a while, and sending codes is limited per user and per client IP.

const (
	TokenBindEmail   TokenPurpose = "bindEmail"   // the code that proves a new email identity's address
	TokenChangeEmail TokenPurpose = "changeEmail" // the code that proves the new login email

	emailCodeTTL = 15 * time.Minute

	// Codes sent count towards a limit when the request was accepted (well-formed).
	emailCodeWindow       = time.Hour
	emailCodeLimitPerUser = 5
	emailCodeLimitPerIP   = 20

	// Wrong codes typed, per user.
	emailCodeFailWindow = 15 * time.Minute
	emailCodeFailLimit  = 5
)

func codeInvalid() error { return Validation(Invalid("code", CodeTokenInvalid, nil)) }

// emailCodeHash is what the token row stores for a code sent to address.
func emailCodeHash(purpose TokenPurpose, userID UserID, address, code string) string {
	return hashToken(fmt.Sprintf("%s:%d:%s:%s", purpose, userID, address, code))
}

func newEmailCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// requireMail refuses with mailNotConfigured (503) while mail does not work.
func (s *Service) requireMail(ctx context.Context) error {
	ok, err := s.MailConfigured(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return newKind(KindUnavailable, CodeMailNotConfigured)
	}
	return nil
}

// limitEmailCodes counts a code request against the user's and the IP's limits.
func (s *Service) limitEmailCodes(ctx context.Context, userID UserID, clientIP string) error {
	now := s.nowUTC()
	buckets := []struct {
		key   string
		limit int
	}{
		{fmt.Sprintf("emailcode:user:%d", userID), emailCodeLimitPerUser},
		{"emailcode:ip:" + clientIP, emailCodeLimitPerIP},
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		if err := tx.PruneRateEvents(ctx, now.Add(-emailCodeWindow)); err != nil {
			return err
		}
		for _, b := range buckets {
			n, err := tx.CountRateEvents(ctx, b.key, now.Add(-emailCodeWindow))
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
}

// RequestBindEmailCode mails a verification code to the address a user without an email
// identity wants to log in with. clientIP is the address of the TCP peer.
func (s *Service) RequestBindEmailCode(ctx context.Context, actor Actor, email, clientIP string) error {
	return s.requestEmailCode(ctx, actor, TokenBindEmail, email, clientIP)
}

func (s *Service) requestEmailCode(ctx context.Context, actor Actor, purpose TokenPurpose, email, clientIP string) error {
	if err := s.requireMail(ctx); err != nil {
		return err
	}
	email = normalizeLoginEmail(email)
	var c Collector
	checkLoginEmail(&c, "email", email)
	if err := c.Err(); err != nil {
		return err
	}
	if err := s.limitEmailCodes(ctx, actor.User.ID, clientIP); err != nil {
		return err
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, actor.User.ID)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if err := s.checkEmailTarget(ctx, tx, cur, purpose, email); err != nil {
			return err
		}
		code, err := newEmailCode()
		if err != nil {
			return err
		}
		now := s.nowUTC()
		// Asking again replaces the earlier code.
		if err := tx.UseUserAuthTokens(ctx, cur.User.ID, []TokenPurpose{purpose}, now); err != nil {
			return err
		}
		if err := tx.InsertAuthToken(ctx, AuthToken{
			TokenHash: emailCodeHash(purpose, cur.User.ID, email, code), UserID: cur.User.ID, Purpose: purpose,
			CreatedAt: now, ExpiresAt: now.Add(emailCodeTTL),
		}); err != nil {
			return err
		}
		_, err = s.enqueueMail(ctx, tx, MailRequest{
			Template: MailTemplateVerifyCode, To: email, Recipient: cur.User.ID, Data: map[string]string{"Code": code},
		})
		return err
	})
}

// checkEmailTarget checks that the user may be given this login email by this purpose.
func (s *Service) checkEmailTarget(ctx context.Context, tx Repository, cur UserWithIdentities, purpose TokenPurpose, email string) error {
	if purpose == TokenBindEmail && cur.hasIdentity(IdentityEmail) {
		return Validation(Invalid("kind", CodeIdentityKindBound, nil))
	}
	if _, held, err := tx.FindIdentity(ctx, IdentityEmail, email); err != nil {
		return err
	} else if held {
		return loginEmailTaken("email")
	}
	return nil
}

// verifyEmailCode spends the code within tx, or says why it does not work.
func (s *Service) verifyEmailCode(ctx context.Context, tx Repository, userID UserID, purpose TokenPurpose, email, code string) error {
	t, found, err := tx.LockAuthToken(ctx, emailCodeHash(purpose, userID, email, code))
	if err != nil {
		return err
	}
	if !found || t.UserID != userID || t.Purpose != purpose {
		return codeInvalid()
	}
	now := s.nowUTC()
	if !t.UsedAt.IsZero() {
		return Validation(Invalid("code", CodeTokenUsed, nil))
	}
	if !now.Before(t.ExpiresAt) {
		return Validation(Invalid("code", CodeTokenExpired, nil))
	}
	return tx.UseUserAuthTokens(ctx, userID, []TokenPurpose{purpose}, now)
}

func (s *Service) failBucket(userID UserID) string {
	return fmt.Sprintf("emailcode-fail:user:%d", userID)
}

// guessLimit refuses a user who typed too many wrong codes lately.
func (s *Service) guessLimit(ctx context.Context, userID UserID) error {
	n, err := s.repo.CountRateEvents(ctx, s.failBucket(userID), s.nowUTC().Add(-emailCodeFailWindow))
	if err != nil {
		return err
	}
	if n >= emailCodeFailLimit {
		return rateLimitedError()
	}
	return nil
}

// noteGuess records a wrong code (after the transaction that refused it ended).
func (s *Service) noteGuess(ctx context.Context, userID UserID, err error) {
	if e, ok := AsError(err); ok && e.Kind == KindValidation && len(e.Errors) == 1 && e.Errors[0].Field == "code" && e.Errors[0].Code == CodeTokenInvalid {
		_ = s.repo.AddRateEvent(ctx, s.failBucket(userID), s.nowUTC())
	}
}

// BindEmailInput is the confirmation of a new email identity: the address, the code mailed to
// it and the password to log in with.
type BindEmailInput struct {
	Email    string
	Code     string
	Password string
}

// BindOwnEmailIdentity gives the actor an email identity after they proved the address with
// the code. The password they choose is final: no forced change.
func (s *Service) BindOwnEmailIdentity(ctx context.Context, actor Actor, in BindEmailInput) (UserWithIdentities, error) {
	if err := s.requireMail(ctx); err != nil {
		return UserWithIdentities{}, err
	}
	email := normalizeLoginEmail(in.Email)
	var c Collector
	checkLoginEmail(&c, "email", email)
	c.Required("code", in.Code)
	checkNewPassword(&c, in.Password)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}
	if err := s.guessLimit(ctx, actor.User.ID); err != nil {
		return UserWithIdentities{}, err
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return UserWithIdentities{}, err
	}
	u, err := s.bindIdentity(ctx, actor.User.ID, bindSpec{
		kind: IdentityEmail, loginName: email, hash: hash, via: BoundViaSelf, by: AuditActorOf(actor),
		before: func(tx Repository) error {
			return s.verifyEmailCode(ctx, tx, actor.User.ID, TokenBindEmail, email, in.Code)
		},
	})
	s.noteGuess(ctx, actor.User.ID, err)
	return u, err
}
